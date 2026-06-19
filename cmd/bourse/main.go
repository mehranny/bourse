// Command bourse runs the market agent that keeps score.
//
//	bourse          — onboarding wizard (default)
//	bourse brief    — gather → reason → score → send one briefing to Telegram
//	bourse notify   — send a message read from stdin to the configured Telegram chat
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
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
	if len(os.Args) > 1 && os.Args[1] == "watchlist" {
		if err := runWatchlist(dataDir, os.Args[2:]); err != nil {
			log.Fatalf("watchlist: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "notify" { // send stdin to the configured Telegram chat
		if err := runNotify(dataDir); err != nil {
			log.Fatalf("notify: %v", err)
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

// runWatchlist implements `bourse watchlist [set SYM1 SYM2 ...]`.
// With no args it prints the current watchlist.
// With "set SYM…" it replaces the watchlist (uppercased, de-duped, order preserved).
func runWatchlist(dataDir string, args []string) error {
	st, err := store.Load(dataDir)
	if err != nil {
		return fmt.Errorf("load config (run onboarding first): %w", err)
	}

	if len(args) == 0 {
		// Print current watchlist.
		wl := st.State.Profile.Watchlist
		if len(wl) == 0 {
			fmt.Println("watchlist: (empty)")
		} else {
			fmt.Println("watchlist:", strings.Join(wl, " "))
		}
		return nil
	}

	if args[0] != "set" {
		return fmt.Errorf("unknown watchlist subcommand %q — usage: bourse watchlist [set SYM1 SYM2 ...]", args[0])
	}

	syms := args[1:]
	if len(syms) == 0 {
		return fmt.Errorf("usage: bourse watchlist set SYM1 SYM2 ...")
	}

	// Uppercase, de-dup preserving order.
	seen := make(map[string]bool, len(syms))
	deduped := make([]string, 0, len(syms))
	for _, s := range syms {
		upper := strings.ToUpper(s)
		if !seen[upper] {
			seen[upper] = true
			deduped = append(deduped, upper)
		}
	}

	st.State.Profile.Watchlist = deduped
	if err := st.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	fmt.Println("watchlist set to:", strings.Join(deduped, " "))
	return nil
}

// runNotify sends a message read from stdin to the configured Telegram chat,
// reusing the agent's own decrypted token + deliver.Telegram (so the token never
// leaves the agent and >4096-char messages are chunked). Used by ops tooling
// (e.g. the nightly review drafter) that must post to the same chat as briefings
// without holding the secret itself.
func runNotify(dataDir string) error {
	st, err := store.Load(dataDir)
	if err != nil {
		return fmt.Errorf("load config (run onboarding first): %w", err)
	}
	if st.State.ChatID == 0 || st.TelegramToken() == "" {
		return fmt.Errorf("no telegram destination configured — finish onboarding first")
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	text := strings.TrimRight(string(raw), "\n")
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("empty message on stdin")
	}
	if err := deliver.Telegram(st.TelegramToken(), st.State.ChatID, text); err != nil {
		return err
	}
	log.Printf("notify: sent %d runes", len([]rune(text)))
	return nil
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
