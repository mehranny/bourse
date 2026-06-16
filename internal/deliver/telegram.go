// Package deliver sends briefings to the user's channel. Telegram first; the
// interface lets email/Discord drop in later.
package deliver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpc = &http.Client{Timeout: 20 * time.Second}

// tgLimit is the Telegram sendMessage hard cap. We split at tgSplit to leave
// a small headroom for any encoding overhead.
const tgLimit = 4096
const tgSplit = 4000

// chunk splits text into slices each no longer than limit characters.
// It prefers to split on newline boundaries so no line is cut mid-way;
// it hard-splits only when a single line exceeds limit.
// Joining the returned slices with "\n" reconstructs the original content
// (minus any leading/trailing blank lines introduced by the split).
func chunk(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	var cur strings.Builder

	for i, line := range lines {
		// If a single line exceeds the limit, hard-split it.
		if len(line) > limit {
			// Flush whatever is buffered first.
			if cur.Len() > 0 {
				chunks = append(chunks, cur.String())
				cur.Reset()
			}
			for len(line) > limit {
				chunks = append(chunks, line[:limit])
				line = line[limit:]
			}
			if len(line) > 0 {
				cur.WriteString(line)
			}
			continue
		}

		// Would adding this line (plus a newline separator) overflow the chunk?
		sep := 0
		if cur.Len() > 0 {
			sep = 1 // for the "\n" between lines
		}
		if cur.Len()+sep+len(line) > limit {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}

		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)

		// If this is the last line, flush.
		if i == len(lines)-1 && cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
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
	for _, part := range chunk(text, tgSplit) {
		if err := telegramSend(token, chatID, part); err != nil {
			return err
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
