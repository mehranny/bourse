package setup

import (
	"net/http"
	"path/filepath"

	"bourse/internal/sentiment"
)

// handleSentiment downloads FinBERT, runs a validation inference, and persists
// the enabled flag. Failure leaves sentiment disabled (never blocks finishing).
func (s *Server) handleSentiment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enable bool `json:"enable"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !in.Enable { // user declined — explicit disabled state
		s.mu.Lock()
		s.state.Sentiment = SentimentConfig{Enabled: false}
		_ = s.save()
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	dir := filepath.Join(s.dataDir, "models", "finbert")
	if err := sentiment.EnsureModel(dir); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "download failed: " + err.Error()})
		return
	}
	sc, err := sentiment.NewScorer(dir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "model load failed: " + err.Error()})
		return
	}
	defer sc.Close()
	scores, err := sc.Score([]string{"Company beats earnings and raises guidance"})
	if err != nil || len(scores) != 1 || scores[0] <= 0 {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "validation inference failed"})
		return
	}
	s.mu.Lock()
	s.state.Sentiment = SentimentConfig{Enabled: true, ModelDir: dir}
	_ = s.save()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true})
}
