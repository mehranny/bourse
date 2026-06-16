// Package remote implements the Brain interface by calling a brainsvc HTTP
// service (the pro deep brain). It is the seam: the agent is brain-agnostic;
// the heavy reasoning lives behind this HTTP boundary.
package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bourse/internal/brain"
	"bourse/internal/store"
)

type Remote struct {
	url string
	hc  *http.Client
}

func New(url string) *Remote {
	return &Remote{url: url, hc: &http.Client{Timeout: 20 * time.Minute}}
}

func (b *Remote) Research(ctx context.Context, watchlist []string, profile store.Profile) (brain.ResearchBundle, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"watchlist": watchlist,
		"profile":   map[string]string{"risk": profile.Risk, "depth": profile.Depth},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", b.url+"/research", bytes.NewReader(reqBody))
	if err != nil {
		return brain.ResearchBundle{}, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := b.hc.Do(req)
	if err != nil {
		return brain.ResearchBundle{}, fmt.Errorf("brainsvc unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return brain.ResearchBundle{}, fmt.Errorf("brainsvc status %d", resp.StatusCode)
	}
	var out brain.ResearchBundle
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return brain.ResearchBundle{}, fmt.Errorf("decode brainsvc response: %w", err)
	}
	return out, nil
}
