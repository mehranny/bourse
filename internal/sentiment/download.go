package sentiment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Release asset URLs (public bourse repo). tokenizer.json is small; finbert.onnx
// is ~440MB. Both land in dir.
const (
	onnxURL      = "https://github.com/mehranny/bourse/releases/download/finbert-v1/finbert.onnx"
	tokenizerURL = "https://github.com/mehranny/bourse/releases/download/finbert-v1/tokenizer.json"
	wantSHA      = "cbceff07acb603c2e0f8112623907bdfa049bb7b69c633b4882eb8e40209d96c"
)

// EnsureModel downloads the model+tokenizer into dir if missing, verifying the
// onnx checksum. Returns nil if already present and valid.
func EnsureModel(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	onnx := filepath.Join(dir, "finbert.onnx")
	if sha256File(onnx) == wantSHA {
		return nil // already good
	}
	if err := download(tokenizerURL, filepath.Join(dir, "tokenizer.json")); err != nil {
		return fmt.Errorf("tokenizer: %w", err)
	}
	if err := download(onnxURL, onnx); err != nil {
		return fmt.Errorf("model: %w", err)
	}
	if got := sha256File(onnx); got != wantSHA {
		return fmt.Errorf("checksum mismatch: got %s want %s", got, wantSHA)
	}
	return nil
}

func download(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, dst)
}

func sha256File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
