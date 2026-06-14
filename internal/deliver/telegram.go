// Package deliver sends briefings to the user's channel. Telegram first; the
// interface lets email/Discord drop in later.
package deliver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var httpc = &http.Client{Timeout: 20 * time.Second}

func Telegram(token string, chatID int64, text string) error {
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	resp, err := httpc.Post("https://api.telegram.org/bot"+token+"/sendMessage",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram send: HTTP %d", resp.StatusCode)
	}
	return nil
}
