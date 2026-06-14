package setup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
)

// Secrets are encrypted at rest with AES-256-GCM. The key comes from
// BOURSE_SECRET_KEY if set (so it can live outside the data volume), otherwise
// a random key is generated once and stored at data/.key (0600). This stops
// tokens from sitting in plaintext config — the historical OpenClaw failure —
// and keeps them out of logs, backups, and any accidental file disclosure.
// It does NOT defend against an attacker with root on the host (the key is
// alongside the data); that is inherent to self-hosting without a vault.
func (s *Server) loadKey() error {
	if v := os.Getenv("BOURSE_SECRET_KEY"); v != "" {
		h := sha256.Sum256([]byte(v))
		s.key = h[:]
		return nil
	}
	// Fallback: a generated key on the data volume (0600). Weaker, because the
	// key and ciphertext can end up in the same backup/snapshot — set
	// BOURSE_SECRET_KEY (off the data volume) for the stronger posture.
	s.keyOnDisk = true
	p := filepath.Join(s.dataDir, ".key")
	if b, err := os.ReadFile(p); err == nil && len(b) == 32 {
		s.key = b
		return nil
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return err
	}
	if err := os.WriteFile(p, k, 0o600); err != nil {
		return err
	}
	s.key = k
	return nil
}

// enc/dec bind each ciphertext to a label via AES-GCM additional authenticated
// data (the confused-deputy / ciphertext-swap defense IronClaw uses): a value
// sealed under "llm" cannot be moved into the "telegram" slot by anyone with
// write access to state.json.
func (s *Server) enc(plain, label string) string {
	if plain == "" {
		return ""
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	ct := gcm.Seal(nonce, nonce, []byte(plain), []byte(label))
	return base64.StdEncoding.EncodeToString(ct)
}

func (s *Server) dec(b64, label string) string {
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
	nonce := raw[:gcm.NonceSize()]
	pt, err := gcm.Open(nil, nonce, raw[gcm.NonceSize():], []byte(label))
	if err != nil {
		return ""
	}
	return string(pt)
}
