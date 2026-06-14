package sentiment

import (
	"math"
	"os"
	"testing"
)

func TestSoftmaxSignedScore(t *testing.T) {
	// logits favoring "positive" (index 0)
	got := signedScore([]float32{2.0, 0.5, 0.5})
	if got <= 0 {
		t.Fatalf("expected positive score, got %v", got)
	}
	// symmetric logits → near zero
	if s := signedScore([]float32{1.0, 1.0, 1.0}); math.Abs(float64(s)) > 1e-6 {
		t.Fatalf("expected ~0 for equal logits, got %v", s)
	}
}

func TestSignal(t *testing.T) {
	if s := Signal(0.73); s != "[sentiment +0.73]" {
		t.Fatalf("got %q", s)
	}
	if s := Signal(-0.5); s != "[sentiment -0.50]" {
		t.Fatalf("got %q", s)
	}
}

func TestScorerInference(t *testing.T) {
	dir := os.Getenv("BOURSE_FINBERT_DIR")
	if dir == "" {
		t.Skip("set BOURSE_FINBERT_DIR to the dir with finbert.onnx + tokenizer.json")
	}
	sc, err := NewScorer(dir)
	if err != nil {
		t.Fatalf("NewScorer: %v", err)
	}
	defer sc.Close()
	scores, err := sc.Score([]string{
		"Company beats earnings and raises guidance",
		"Firm slashes outlook amid plunging demand",
	})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("want 2 scores, got %d", len(scores))
	}
	if scores[0] <= 0 {
		t.Errorf("bullish headline should score > 0, got %v", scores[0])
	}
	if scores[1] >= 0 {
		t.Errorf("bearish headline should score < 0, got %v", scores[1])
	}
}
