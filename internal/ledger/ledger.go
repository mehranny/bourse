// Package ledger is the spine: it records every call BEFORE the outcome
// (append-only, hash-chained, timestamped), resolves due calls against real
// prices (external ground truth — never the model rating itself), and scores
// them with Brier vs a coin-flip baseline.
package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bourse/internal/brain"
	"bourse/internal/sources"
)

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

type Entry struct {
	ID         string  `json:"id"`
	Brain      string  `json:"brain,omitempty"` // "lite" | "pro" (empty = legacy lite)
	Symbol     string  `json:"symbol"`
	Direction  string  `json:"direction"`
	Prob       float64 `json:"prob"`
	RefPrice   float64 `json:"ref_price"`
	Horizon    string  `json:"horizon"`
	CreatedUTC string  `json:"created_utc"`
	ResolveOn  string  `json:"resolve_on"` // YYYY-MM-DD
	Resolved   bool    `json:"resolved"`
	Outcome    int     `json:"outcome"` // 1 if direction correct, else 0
	Brier      float64 `json:"brier"`
	PrevHash   string  `json:"prev_hash"`
	Hash       string  `json:"hash"`
}

type Ledger struct{ path string }

func New(dataDir string) *Ledger { return &Ledger{path: filepath.Join(dataDir, "ledger.jsonl")} }

func (l *Ledger) load() ([]Entry, error) {
	b, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var es []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(line), &e) == nil {
			es = append(es, e)
		}
	}
	return es, nil
}

func (l *Ledger) save(es []Entry) error {
	var sb strings.Builder
	for _, e := range es {
		b, _ := json.Marshal(e)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return os.WriteFile(l.path, []byte(sb.String()), 0o600)
}

// Record appends today's calls, hash-chained, with a resolution date from horizon.
func (l *Ledger) Record(now time.Time, calls []brain.Call) error {
	es, err := l.load()
	if err != nil {
		return err
	}
	prev := genesis
	if len(es) > 0 {
		prev = es[len(es)-1].Hash
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	brainTag := os.Getenv("BOURSE_BRAIN")
	if brainTag == "" {
		brainTag = "lite"
	}
	for i, c := range calls {
		if c.RefPrice == 0 {
			continue
		}
		e := Entry{
			ID:         fmt.Sprintf("%s-%s-%d", c.Symbol, now.Format("20060102"), i),
			Brain:      brainTag,
			Symbol:     c.Symbol, Direction: c.Direction, Prob: c.Prob, RefPrice: c.RefPrice,
			Horizon: c.Horizon, CreatedUTC: now.UTC().Format(time.RFC3339),
			ResolveOn: resolveDate(now, c.Horizon), PrevHash: prev,
		}
		e.Hash = chain(prev, e)
		prev = e.Hash
		b, _ := json.Marshal(e)
		f.Write(append(b, '\n'))
	}
	return nil
}

// Resolve marks due calls outcome+brier using real closes.
func (l *Ledger) Resolve(now time.Time, ps sources.PriceSource) error {
	es, err := l.load()
	if err != nil {
		return err
	}
	today := now.Format("2006-01-02")
	changed := false
	for i := range es {
		e := &es[i]
		if e.Resolved || e.ResolveOn > today {
			continue
		}
		q, err := ps.Quote(e.Symbol)
		if err != nil {
			continue
		}
		up := q.Price > e.RefPrice
		correct := (e.Direction == "up" && up) || (e.Direction == "down" && !up)
		e.Outcome = 0
		if correct {
			e.Outcome = 1
		}
		e.Brier = (e.Prob - float64(e.Outcome)) * (e.Prob - float64(e.Outcome))
		e.Resolved = true
		changed = true
	}
	if changed {
		return l.save(es)
	}
	return nil
}

type Score struct {
	Resolved   int
	MeanBrier  float64
	SkillVsCoin float64 // 1 - brier/0.25; >0 beats a coin flip
}

func (l *Ledger) Score() (Score, error) {
	es, err := l.load()
	if err != nil {
		return Score{}, err
	}
	var n int
	var sum float64
	for _, e := range es {
		if e.Resolved {
			n++
			sum += e.Brier
		}
	}
	if n == 0 {
		return Score{}, nil
	}
	mean := sum / float64(n)
	return Score{Resolved: n, MeanBrier: mean, SkillVsCoin: 1 - mean/0.25}, nil
}

func chain(prev string, e Entry) string {
	e.Hash, e.PrevHash = "", ""
	b, _ := json.Marshal(e)
	h := sha256.Sum256(append([]byte(prev), b...))
	return hex.EncodeToString(h[:])
}

func resolveDate(now time.Time, horizon string) string {
	days := 5
	fmt.Sscanf(horizon, "%dd", &days)
	d := now
	for added := 0; added < days; {
		d = d.AddDate(0, 0, 1)
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			added++
		}
	}
	return d.Format("2006-01-02")
}
