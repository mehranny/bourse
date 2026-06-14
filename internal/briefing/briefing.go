// Package briefing turns a brain's research + the track record into the short,
// scannable message that lands in Telegram. Briefing-not-digest: the calls and
// why they matter, with the honest score on every send.
package briefing

import (
	"fmt"
	"os"
	"strings"
	"time"

	"bourse/internal/brain"
	"bourse/internal/ledger"
)

// brainHeader labels which brain produced this briefing and, in one line, what
// makes it different — so the two daily messages (lite vs pro) are distinguishable
// and self-explanatory in the shared chat.
func brainHeader(sb *strings.Builder) {
	switch os.Getenv("BOURSE_BRAIN") {
	case "pro", "remote":
		sb.WriteString("🔵 PRO brain — Tier-0 classifiers (FinBERT + embedding rerank) feed Claude's Fermi/simulation reasoning, then Platt-calibrated.\n\n")
	default:
		sb.WriteString("🟢 LITE brain — your own LLM reasoning over free data (prices + news) with FinBERT sentiment signals.\n\n")
	}
}

func Compose(now time.Time, b brain.ResearchBundle, sc ledger.Score) string {
	var sb strings.Builder
	brainHeader(&sb)
	fmt.Fprintf(&sb, "📊 Bourse — %s\n", now.Format("Mon Jan 2"))
	if b.Summary != "" {
		fmt.Fprintf(&sb, "%s\n", b.Summary)
	}
	sb.WriteString("\n")
	for _, c := range b.Calls {
		arrow := "▲"
		if c.Direction == "down" {
			arrow = "▼"
		}
		fmt.Fprintf(&sb, "%s %s · %.0f%% %s (%s)\n", arrow, strings.ToUpper(c.Symbol),
			c.Prob*100, c.Direction, c.Horizon)
		if c.Rationale != "" {
			fmt.Fprintf(&sb, "   %s\n", c.Rationale)
		}
	}
	sb.WriteString("\n")
	if sc.Resolved > 0 {
		fmt.Fprintf(&sb, "📒 Track record: %d calls resolved · Brier %.3f · %s\n",
			sc.Resolved, sc.MeanBrier, coin(sc.SkillVsCoin))
	} else {
		sb.WriteString("📒 Track record: building — today's calls are logged and will be scored.\n")
	}
	sb.WriteString("Research, not investment advice.")
	return sb.String()
}

func coin(skill float64) string {
	if skill > 0 {
		return fmt.Sprintf("beating coin-flip by %.0f%%", skill*100)
	}
	return fmt.Sprintf("%.0f%% behind coin-flip", -skill*100)
}
