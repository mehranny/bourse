// Package lite is the free, in-binary default brain: a transparent RAG pipeline
// over free data (Yahoo prices + Google News) reasoned by the user's own LLM.
// Its value is a calibrated, sourced daily read that keeps honest score — not
// secret alpha. Users extend it by swapping sources or editing the analysis lens.
package lite

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"bourse/internal/brain"
	"bourse/internal/llm"
	"bourse/internal/sources"
	"bourse/internal/store"
)

//go:embed analysis.md
var defaultLens string

type Brain struct {
	st     *store.Store
	prices sources.PriceSource
	news   sources.NewsSource
	lens   string
}

func New(st *store.Store, lens string) *Brain {
	if lens == "" {
		lens = defaultLens
	}
	return &Brain{st: st, prices: sources.Yahoo{}, news: sources.GoogleNews{}, lens: lens}
}

func (b *Brain) Research(ctx context.Context, watchlist []string, profile store.Profile) (brain.ResearchBundle, error) {
	// 1. GATHER — free data per ticker
	var ctxBuf strings.Builder
	refPrice := map[string]float64{}
	for _, sym := range watchlist {
		q, err := b.prices.Quote(sym)
		if err != nil {
			continue
		}
		refPrice[sym] = q.Price
		fmt.Fprintf(&ctxBuf, "\n## %s\nprice %.2f (%+.2f%% vs prev close), 5d range %.2f–%.2f\n",
			sym, q.Price, q.ChangePct, q.Low5d, q.High5d)
		hs, _ := b.news.Headlines(sym, 4)
		for _, h := range hs {
			fmt.Fprintf(&ctxBuf, "- %s\n", h.Title)
		}
	}
	if ctxBuf.Len() == 0 {
		return brain.ResearchBundle{}, fmt.Errorf("no market data fetched")
	}

	// 2. REASON — the user's LLM, disciplined by the analysis lens
	prompt := b.lens +
		"\n\nRISK PROFILE: " + profile.Risk +
		"\nDEPTH: " + profile.Depth +
		"\n\nDATA:\n" + ctxBuf.String() +
		"\n\nReturn ONLY a JSON object, no prose, of the form:\n" +
		`{"summary":"2-3 sentence read of the day","calls":[{"symbol":"NVDA","direction":"up","horizon":"5d","prob":0.58,"confidence":"medium","rationale":"one sentence","evidence":["a headline or fact you used"]}]}` +
		"\nRules: prob is P(direction correct) in 0.10..0.90; anchor near 0.50 and only deviate with a real reason; never be reflexively bullish; one call per ticker."

	raw, err := llm.Generate(ctx, b.st.State.LLM, b.st.LLMSecret(), b.st.DataDir(), prompt)
	if err != nil {
		return brain.ResearchBundle{}, err
	}

	// 3. PARSE — tolerate models that wrap JSON in prose/fences
	bundle, err := parseBundle(raw)
	if err != nil {
		return brain.ResearchBundle{}, fmt.Errorf("parse model output: %w", err)
	}
	for i := range bundle.Calls {
		c := &bundle.Calls[i]
		c.RefPrice = refPrice[strings.ToUpper(c.Symbol)]
		if c.Prob < 0.05 {
			c.Prob = 0.05
		}
		if c.Prob > 0.95 {
			c.Prob = 0.95
		}
	}
	return bundle, nil
}

var jsonRe = regexp.MustCompile(`(?s)\{.*\}`)

func parseBundle(raw string) (brain.ResearchBundle, error) {
	var b brain.ResearchBundle
	if err := json.Unmarshal([]byte(raw), &b); err == nil && len(b.Calls) > 0 {
		return b, nil
	}
	m := jsonRe.FindString(raw)
	if m == "" {
		return b, fmt.Errorf("no JSON found")
	}
	return b, json.Unmarshal([]byte(m), &b)
}
