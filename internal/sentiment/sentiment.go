package sentiment

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

const maxSeqLen = 128

// Scorer loads a FinBERT ONNX model + tokenizer.json and scores headlines.
type Scorer struct {
	tk      *tokenizer.Tokenizer
	session *ort.DynamicAdvancedSession
}

// NewScorer loads finbert.onnx + tokenizer.json from dir and prepares a session.
// The ORT shared library must be reachable; set BOURSE_ORT_LIB to override the
// default search path (/usr/local/lib/libonnxruntime.{dylib,so}).
func NewScorer(dir string) (*Scorer, error) {
	if !ort.IsInitialized() {
		ort.SetSharedLibraryPath(ortLibPath())
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("init onnxruntime: %w", err)
		}
	}

	tk, err := pretrained.FromFile(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(
		filepath.Join(dir, "finbert.onnx"),
		[]string{"input_ids", "attention_mask"},
		[]string{"logits"},
		nil, // SessionOptions — use defaults
	)
	if err != nil {
		return nil, fmt.Errorf("load onnx model: %w", err)
	}

	return &Scorer{tk: tk, session: session}, nil
}

// Close releases the underlying ONNX session.
func (s *Scorer) Close() {
	if s.session != nil {
		_ = s.session.Destroy()
	}
}

// Score returns one signed score in [-1,+1] per input headline.
// Positive values indicate bullish sentiment, negative indicate bearish.
func (s *Scorer) Score(headlines []string) ([]float32, error) {
	out := make([]float32, len(headlines))
	for i, h := range headlines {
		ids, mask := s.encode(h, maxSeqLen)

		shape := ort.NewShape(1, int64(maxSeqLen))

		idsTensor, err := ort.NewTensor[int64](shape, ids)
		if err != nil {
			return nil, fmt.Errorf("create input_ids tensor: %w", err)
		}
		defer idsTensor.Destroy() //nolint:gocritic // per-iter defer is fine here

		maskTensor, err := ort.NewTensor[int64](shape, mask)
		if err != nil {
			return nil, fmt.Errorf("create attention_mask tensor: %w", err)
		}
		defer maskTensor.Destroy() //nolint:gocritic

		// Pass nil for the output; DynamicAdvancedSession auto-allocates it.
		outputs := []ort.Value{nil}
		inputs := []ort.Value{idsTensor, maskTensor}

		if err := s.session.Run(inputs, outputs); err != nil {
			return nil, fmt.Errorf("onnx inference: %w", err)
		}

		// outputs[0] is now an auto-allocated *Tensor[float32] ([1, 3]).
		logitsTensor, ok := outputs[0].(*ort.Tensor[float32])
		if !ok {
			return nil, fmt.Errorf("unexpected output type from model")
		}
		defer logitsTensor.Destroy() //nolint:gocritic

		logits := logitsTensor.GetData() // []float32 len 3
		if len(logits) < 3 {
			return nil, fmt.Errorf("expected 3 logits, got %d", len(logits))
		}

		out[i] = signedScore(logits[:3])
	}
	return out, nil
}

// encode returns padded input_ids and attention_mask of length n (int64).
// Special tokens ([CLS], [SEP]) are added so the model sees a well-formed input.
func (s *Scorer) encode(text string, n int) ([]int64, []int64) {
	en, _ := s.tk.EncodeSingle(text, true) // true = add special tokens
	ids := make([]int64, n)
	mask := make([]int64, n)
	if en != nil {
		srcIds := en.GetIds()           // []int
		srcMask := en.GetAttentionMask() // []int
		for i := 0; i < n && i < len(srcIds); i++ {
			ids[i] = int64(srcIds[i])
			if i < len(srcMask) {
				mask[i] = int64(srcMask[i])
			} else {
				mask[i] = 1
			}
		}
	}
	return ids, mask
}

// ortLibPath returns the path to the ORT shared library.
// Override with the BOURSE_ORT_LIB environment variable.
func ortLibPath() string {
	if p := os.Getenv("BOURSE_ORT_LIB"); p != "" {
		return p
	}
	if runtime.GOOS == "darwin" {
		return "/usr/local/lib/libonnxruntime.dylib"
	}
	return "/usr/local/lib/libonnxruntime.so"
}
