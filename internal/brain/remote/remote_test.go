package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"bourse/internal/store"
)

func TestResearch(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantErr  bool
		wantCall int
	}{
		{"ok", 200, `{"summary":"s","calls":[{"symbol":"NVDA","direction":"up","horizon":"5d","prob":0.6}]}`, false, 1},
		{"down", 502, `{}`, true, 0},
		{"malformed", 200, `not json`, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			}))
			defer srv.Close()
			b := New(srv.URL)
			got, err := b.Research(context.Background(), []string{"NVDA"}, store.Profile{Risk: "balanced"})
			if c.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !c.wantErr {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				if len(got.Calls) != c.wantCall {
					t.Fatalf("want %d calls, got %d", c.wantCall, len(got.Calls))
				}
			}
		})
	}
}
