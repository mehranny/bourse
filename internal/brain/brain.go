// Package brain defines what a Bourse brain is. The default (lite) ships free
// and in-binary; heavy/private brains (e.g. the 47-layer engine) implement the
// same interface behind the remote HTTP seam. Every brain's Calls are scored by
// the ledger, so the scoreboard is the shared yardstick across brains.
package brain

import (
	"context"

	"bourse/internal/store"
)

// Call is one falsifiable, scoreable prediction.
type Call struct {
	Symbol     string   `json:"symbol"`
	Direction  string   `json:"direction"` // "up" | "down"
	Horizon    string   `json:"horizon"`   // e.g. "5d"
	Prob       float64  `json:"prob"`      // P(direction is correct), 0.01..0.99
	Confidence string   `json:"confidence"`
	Rationale  string   `json:"rationale"`
	Evidence   []string `json:"evidence"`
	RefPrice   float64  `json:"ref_price"` // price when the call was made (for resolution)
}

// ResearchBundle is a day's output from a brain.
type ResearchBundle struct {
	Summary string `json:"summary"`
	Calls   []Call `json:"calls"`
}

// Brain is the swap point: lite (default), remote (your engine), or anything.
type Brain interface {
	Research(ctx context.Context, watchlist []string, profile store.Profile) (ResearchBundle, error)
}
