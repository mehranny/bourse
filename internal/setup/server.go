package setup

import (
	"crypto/rand"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

//go:embed web/*
var webFS embed.FS

// LLMConfig captures how the user powers Bourse. Secret holds an API key, a
// Claude subscription OAuth token, or an Ollama base URL depending on Mode.
type LLMConfig struct {
	Mode     string `json:"mode"`     // subscription | api | local
	Provider string `json:"provider"` // anthropic | openai | gemini | ollama
	Secret   string `json:"secret"`
	Model    string `json:"model"`
}

type Profile struct {
	Risk      string   `json:"risk"`      // conservative | balanced | aggressive | custom
	Watchlist []string `json:"watchlist"`
	BriefTime string   `json:"brief_time"`
	Timezone  string   `json:"timezone"`
	Depth     string   `json:"depth"` // headline | standard | deep
}

type State struct {
	Configured  bool      `json:"configured"`
	LLM         LLMConfig `json:"llm"`
	LLMValid    bool      `json:"llm_valid"`
	TelegramTok string    `json:"telegram_token"`
	BotUsername string    `json:"bot_username"`
	ChatID      int64     `json:"chat_id"`
	Profile     Profile   `json:"profile"`
}

type Server struct {
	dataDir   string
	setupCode string
	key       []byte
	keyOnDisk bool
	mu        sync.Mutex
	state     State
	mux       *http.ServeMux
}

func New(dataDir string) (*Server, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	s := &Server{dataDir: dataDir}
	if err := s.loadKey(); err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	if err := s.ensureSetupCode(); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}

func (s *Server) Configured() bool { return s.state.Configured }
func (s *Server) SetupCode() string { return s.setupCode }

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

// ---- persistence ----

func (s *Server) statePath() string { return filepath.Join(s.dataDir, "state.json") }
func (s *Server) codePath() string  { return filepath.Join(s.dataDir, "setup_code") }

func (s *Server) load() error {
	b, err := os.ReadFile(s.statePath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &s.state)
}

func (s *Server) save() error {
	b, _ := json.MarshalIndent(s.state, "", "  ")
	return os.WriteFile(s.statePath(), b, 0o600)
}

func (s *Server) ensureSetupCode() error {
	if b, err := os.ReadFile(s.codePath()); err == nil && len(b) > 0 {
		s.setupCode = string(b)
		return nil
	}
	s.setupCode = randomCode()
	return os.WriteFile(s.codePath(), []byte(s.setupCode), 0o600)
}

func randomCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b[:4]) + "-" + string(b[4:])
}

// ---- routing ----

func (s *Server) routes() {
	s.mux = http.NewServeMux()

	sub, _ := fs.Sub(webFS, "web")
	fileServer := http.FileServer(http.FS(sub))

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if s.Configured() {
				http.Redirect(w, r, "/status", http.StatusFound)
				return
			}
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/api/state", s.handleState)
	s.mux.HandleFunc("/api/unlock", s.handleUnlock)
	s.mux.HandleFunc("/api/llm", s.guard(s.handleLLM))
	s.mux.HandleFunc("/api/telegram", s.guard(s.handleTelegram))
	s.mux.HandleFunc("/api/telegram/chat", s.guard(s.handleTelegramChat))
	s.mux.HandleFunc("/api/profile", s.guard(s.handleProfile))
	s.mux.HandleFunc("/api/finish", s.guard(s.handleFinish))
}

// guard requires the setup-code cookie set by /api/unlock.
func (s *Server) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("bourse_setup")
		if err != nil || c.Value != s.setupCode {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "locked"})
			return
		}
		h(w, r)
	}
}

// ---- handlers ----

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":   s.state.Configured,
		"llm_valid":    s.state.LLMValid,
		"provider":     s.state.LLM.Provider,
		"mode":         s.state.LLM.Mode,
		"model":        s.state.LLM.Model,
		"bot_username": s.state.BotUsername,
		"chat_id":      s.state.ChatID,
		"profile":      s.state.Profile,
	})
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Code != s.setupCode {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Wrong setup code. Check the deploy logs for the SETUP CODE."})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "bourse_setup", Value: s.setupCode, Path: "/", HttpOnly: true, MaxAge: 3600})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLLM(w http.ResponseWriter, r *http.Request) {
	var in LLMConfig
	if !decode(w, r, &in) {
		return
	}
	model, err := validateLLM(in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.mu.Lock()
	in.Model = model
	in.Secret = s.enc(in.Secret, "llm") // encrypt at rest — never plaintext on disk
	s.state.LLM = in
	s.state.LLMValid = true
	s.save()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "model": model})
}

func (s *Server) handleTelegram(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	username, err := validateTelegram(in.Token)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.mu.Lock()
	s.state.TelegramTok = s.enc(in.Token, "telegram") // encrypt at rest
	s.state.BotUsername = username
	s.save()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"bot_username": username})
}

func (s *Server) handleTelegramChat(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tok := s.dec(s.state.TelegramTok, "telegram")
	s.mu.Unlock()
	if tok == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "connect the bot token first"})
		return
	}
	chatID, first, err := telegramFindChat(tok)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"found": false, "hint": err.Error()})
		return
	}
	s.mu.Lock()
	s.state.ChatID = chatID
	s.save()
	s.mu.Unlock()
	_ = telegramSend(tok, chatID, "✅ Bourse is connected. Your morning briefings will arrive here.")
	writeJSON(w, http.StatusOK, map[string]any{"found": true, "chat_id": chatID, "name": first})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	var in Profile
	if !decode(w, r, &in) {
		return
	}
	s.mu.Lock()
	s.state.Profile = in
	s.save()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFinish(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.LLMValid || s.state.ChatID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "LLM and Telegram must both be connected first"})
		return
	}
	s.state.Configured = true
	s.save()
	if s.state.TelegramTok != "" && s.state.ChatID != 0 {
		_ = telegramSend(s.dec(s.state.TelegramTok, "telegram"), s.state.ChatID,
			"🎉 Setup complete. Bourse will brief you at "+s.state.Profile.BriefTime+" ("+s.state.Profile.Timezone+"). It keeps score on every call it makes.")
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	st := s.state
	s.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	green := func(ok bool) string {
		if ok {
			return "✅"
		}
		return "❌"
	}
	fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><meta name=viewport content="width=device-width,initial-scale=1">
<title>Bourse · status</title>
<style>body{font:16px/1.5 system-ui;max-width:640px;margin:40px auto;padding:0 20px;color:#111}
h1{font-size:22px}.row{padding:10px 0;border-bottom:1px solid #eee}.muted{color:#666}</style>
<h1>Bourse — status</h1>
<div class=row>%s LLM &nbsp;<span class=muted>%s / %s (%s)</span></div>
<div class=row>%s Telegram &nbsp;<span class=muted>@%s</span></div>
<div class=row>%s Watchlist &nbsp;<span class=muted>%v</span></div>
<div class=row>Next briefing &nbsp;<span class=muted>%s %s</span></div>
<p class=muted>Bourse keeps score: every call it makes is logged before the outcome and graded against what actually happened.</p>`,
		green(st.LLMValid), st.LLM.Provider, st.LLM.Mode, st.LLM.Model,
		green(st.ChatID != 0), st.BotUsername,
		green(len(st.Profile.Watchlist) > 0), st.Profile.Watchlist,
		st.Profile.BriefTime, st.Profile.Timezone)
}

// ---- helpers ----

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// KeyOnDisk reports whether the encryption key is the on-disk fallback (weaker)
// rather than supplied via BOURSE_SECRET_KEY.
func (s *Server) KeyOnDisk() bool { return s.keyOnDisk }
