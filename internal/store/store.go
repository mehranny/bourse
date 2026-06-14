// Package store reads the configuration the onboarding wizard wrote.
//
// NOTE: the on-disk format (data/state.json + the encryption key) is OWNED by
// internal/setup. These types and the crypto here MUST stay in sync with it.
// Kept as a separate reader so the brain/briefing can load config without
// depending on the HTTP server.
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
)

type LLMConfig struct {
	Mode     string `json:"mode"`
	Provider string `json:"provider"`
	Secret   string `json:"secret"`
	Model    string `json:"model"`
}

type Profile struct {
	Risk      string   `json:"risk"`
	Watchlist []string `json:"watchlist"`
	BriefTime string   `json:"brief_time"`
	Timezone  string   `json:"timezone"`
	Depth     string   `json:"depth"`
}

type SentimentConfig struct {
	Enabled  bool   `json:"enabled"`
	ModelDir string `json:"model_dir"` // where finbert.onnx + tokenizer.json live
}

type State struct {
	Configured  bool      `json:"configured"`
	LLM         LLMConfig `json:"llm"`
	LLMValid    bool      `json:"llm_valid"`
	TelegramTok string    `json:"telegram_token"`
	BotUsername string    `json:"bot_username"`
	ChatID      int64     `json:"chat_id"`
	Profile     Profile   `json:"profile"`
	Sentiment   SentimentConfig `json:"sentiment"`
}

type Store struct {
	dir   string
	key   []byte
	State State
}

func Load(dir string) (*Store, error) {
	s := &Store{dir: dir}
	if err := s.loadKey(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.State); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) DataDir() string       { return s.dir }
func (s *Store) LLMSecret() string     { return s.dec(s.State.LLM.Secret, "llm") }
func (s *Store) TelegramToken() string { return s.dec(s.State.TelegramTok, "telegram") }

func (s *Store) loadKey() error {
	if v := os.Getenv("BOURSE_SECRET_KEY"); v != "" {
		h := sha256.Sum256([]byte(v))
		s.key = h[:]
		return nil
	}
	b, err := os.ReadFile(filepath.Join(s.dir, ".key"))
	if err != nil {
		return err
	}
	s.key = b
	return nil
}

func (s *Store) dec(b64, label string) string {
	if b64 == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return ""
	}
	pt, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(label))
	if err != nil {
		return ""
	}
	return string(pt)
}
