// Command bourse runs the market agent that keeps score.
//
//	bourse          — onboarding wizard (default)
//	bourse brief    — gather → reason → score → send one briefing to Telegram
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"bourse/internal/brain"
	"bourse/internal/brain/lite"
	"bourse/internal/brain/remote"
	"bourse/internal/briefing"
	"bourse/internal/deliver"
	"bourse/internal/ledger"
	"bourse/internal/setup"
	"bourse/internal/sources"
	"bourse/internal/store"
)

func main() {
	dataDir := env("BOURSE_DATA_DIR", "./data")
	if len(os.Args) > 1 && os.Args[1] == "brief" {
		if err := runBrief(dataDir); err != nil {
			log.Fatalf("brief: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "probe" { // test free data sources, no config
		syms := os.Args[2:]
		if len(syms) == 0 {
			syms = []string{"NVDA", "AAPL"}
		}
		for _, s := range syms {
			q, err := (sources.Yahoo{}).Quote(s)
			fmt.Printf("%s: %.2f (%+.2f%%) err=%v\n", s, q.Price, q.ChangePct, err)
			hs, _ := (sources.GoogleNews{}).Headlines(s, 3)
			for _, h := range hs {
				fmt.Printf("   • %s\n", h.Title)
			}
		}
		return
	}
	runWizard(dataDir, env("BOURSE_PORT", "8080"))
}

func runBrief(dataDir string) error {
	dry := len(os.Args) > 2 && os.Args[2] == "--dry"
	st, err := store.Load(dataDir)
	if err != nil {
		return fmt.Errorf("load config (run onboarding first): %w", err)
	}
	if dry {
		if !st.State.LLMValid || len(st.State.Profile.Watchlist) == 0 {
			return fmt.Errorf("dry run needs a validated LLM and a watchlist")
		}
	} else if !st.State.Configured {
		return fmt.Errorf("not configured yet — finish the setup wizard")
	}
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	var b brain.Brain
	switch os.Getenv("BOURSE_BRAIN") {
	case "remote", "pro":
		url := env("BRAIN_URL", "http://brainsvc:8000")
		b = remote.New(url)
	default:
		b = lite.New(st, loadLens(dataDir))
	}
	bundle, err := b.Research(ctx, st.State.Profile.Watchlist, st.State.Profile)
	if err != nil {
		return err
	}

	lg := ledger.New(dataDir)
	if err := lg.Record(now, bundle.Calls); err != nil { // log BEFORE outcomes
		return err
	}
	_ = lg.Resolve(now, sources.Yahoo{}) // resolve anything now due
	score, _ := lg.Score()

	text := briefing.Compose(now, bundle, score)
	if dry {
		fmt.Println("\n----- BRIEFING (dry, not sent) -----")
		fmt.Println(text)
		return nil
	}
	if err := deliver.Telegram(st.TelegramToken(), st.State.ChatID, text); err != nil {
		return err
	}
	log.Printf("briefing sent: %d calls, %d resolved", len(bundle.Calls), score.Resolved)
	return nil
}

// loadLens lets users override the analysis style at data/analysis.md.
func loadLens(dataDir string) string {
	if b, err := os.ReadFile(dataDir + "/analysis.md"); err == nil {
		return string(b)
	}
	return ""
}

func runWizard(dataDir, port string) {
	srv, err := setup.New(dataDir)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	if srv.Configured() {
		fmt.Println("Bourse is configured. Status: http://<host>:" + port + "/status")
	} else {
		fmt.Println("┌────────────────────────────────────────────────────")
		fmt.Println("│  Bourse — the market agent that keeps score")
		fmt.Println("│")
		fmt.Println("│  Open the setup wizard in your browser, then enter")
		fmt.Println("│  this one-time code to claim this instance:")
		fmt.Println("│")
		fmt.Printf("│      SETUP CODE:  %s\n", srv.SetupCode())
		fmt.Println("└────────────────────────────────────────────────────")
	}
	if srv.KeyOnDisk() {
		log.Printf("note: secrets encrypted with an on-disk key (%s/.key). Set "+
			"BOURSE_SECRET_KEY to keep the key off the data volume.", dataDir)
	}
	log.Printf("listening on :%s", port)
	if err := srv.ListenAndServe(":" + port); err != nil {
		log.Fatal(err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
