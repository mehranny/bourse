// Package deliver sends briefings to the user's channel. Telegram first; the
// interface lets email/Discord drop in later.
package deliver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var httpc = &http.Client{Timeout: 20 * time.Second}

// tgSplit is the character limit we target per chunk, leaving headroom below
// Telegram's 4096-character hard cap.
const tgSplit = 4000

// chunk splits text into slices each no longer than limit *characters* (runes).
// It prefers to split on newline boundaries so no line is cut mid-way;
// it hard-splits only when a single line exceeds limit, always on rune
// boundaries so multi-byte characters (e.g. emoji) are never corrupted.
//
// Reassembly contract:
//   - Newline-split lines are joined with "\n" to recover the original.
//   - A hard-split line's pieces are joined with "" (no separator).
func chunk(text string, limit int) []string {
	if utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	var cur strings.Builder
	curRunes := 0 // rune count of cur's current content

	for i, line := range lines {
		lineRunes := utf8.RuneCountInString(line)

		// If a single line exceeds the limit, hard-split it on rune boundaries.
		if lineRunes > limit {
			// Flush whatever is buffered first.
			if cur.Len() > 0 {
				chunks = append(chunks, cur.String())
				cur.Reset()
				curRunes = 0
			}
			r := []rune(line)
			for len(r) > limit {
				chunks = append(chunks, string(r[:limit]))
				r = r[limit:]
			}
			if len(r) > 0 {
				cur.WriteString(string(r))
				curRunes = len(r)
			}
			continue
		}

		// Would adding this line (plus a newline separator) overflow the chunk?
		sep := 0
		if curRunes > 0 {
			sep = 1 // for the "\n" between lines
		}
		if curRunes+sep+lineRunes > limit {
			chunks = append(chunks, cur.String())
			cur.Reset()
			curRunes = 0
		}

		if curRunes > 0 {
			cur.WriteByte('\n')
			curRunes++
		}
		cur.WriteString(line)
		curRunes += lineRunes

		// If this is the last line, flush.
		if i == len(lines)-1 && cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
			curRunes = 0
		}
	}

	// Flush any remainder (handles cases where the loop ends without flushing).
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}

	return chunks
}

// Telegram sends text to a Telegram chat via the Bot API.
// Long messages are split into <=tgSplit-char chunks (preferring newline
// boundaries) and sent in order. On any non-200 response, the response body
// is included in the error for diagnosability.
func Telegram(token string, chatID int64, text string) error {
	parts := chunk(text, tgSplit)
	n := len(parts)
	if n > 1 {
		log.Printf("telegram: sending %d chunks", n)
	}
	for i, part := range parts {
		if err := telegramSend(token, chatID, part); err != nil {
			return fmt.Errorf("telegram send chunk %d/%d: %w", i+1, n, err)
		}
	}
	return nil
}

func telegramSend(token string, chatID int64, text string) error {
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	resp, err := httpc.Post("https://api.telegram.org/bot"+token+"/sendMessage",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("telegram send: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
