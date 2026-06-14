package sentiment

import (
	"fmt"
	"math"
)

// ProsusAI/finbert label order from its config.json id2label.
const (
	idxPositive = 0
	idxNegative = 1
	idxNeutral  = 2
)

func softmax(logits []float32) []float32 {
	max := logits[0]
	for _, v := range logits {
		if v > max {
			max = v
		}
	}
	var sum float64
	out := make([]float32, len(logits))
	for i, v := range logits {
		e := math.Exp(float64(v - max))
		out[i] = float32(e)
		sum += e
	}
	for i := range out {
		out[i] = float32(float64(out[i]) / sum)
	}
	return out
}

// signedScore = P(positive) - P(negative), in [-1, +1].
func signedScore(logits []float32) float32 {
	p := softmax(logits)
	return p[idxPositive] - p[idxNegative]
}

// Signal renders a compact prompt tag, e.g. "[sentiment +0.73]".
func Signal(score float32) string {
	return fmt.Sprintf("[sentiment %+.2f]", score)
}
