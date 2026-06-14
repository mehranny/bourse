package setup

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

// validateLLM makes ONE real test call so the user learns immediately whether
// their credential works — never a silent half-broken state. Returns the model
// label on success.
func validateLLM(c LLMConfig) (string, error) {
	switch c.Provider {
	case "anthropic":
		return validateAnthropic(c)
	case "openai":
		return validateOpenAI(c.Secret)
	case "openai-codex":
		return validateCodexSubscription(c.Secret)
	case "gemini":
		return validateGemini(c.Secret)
	case "ollama":
		return validateOllama(c.Secret)
	default:
		return "", fmt.Errorf("unknown provider %q", c.Provider)
	}
}

func validateAnthropic(c LLMConfig) (string, error) {
	const model = "claude-sonnet-4-6"
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	})
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	switch c.Mode {
	case "subscription":
		// Claude Code OAuth token (sk-ant-oat...) is a bearer credential.
		req.Header.Set("authorization", "Bearer "+c.Secret)
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	default: // api
		req.Header.Set("x-api-key", c.Secret)
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach Anthropic: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return model, nil
	}
	// A Claude subscription (OAuth) token authenticates here but Anthropic
	// throttles raw-API use of it (briefings route through the Claude Code
	// client, not the bare API). A *bad* token returns 401; a *real* one
	// returns 200 or 429 — so 429 on the subscription path means "valid".
	if c.Mode == "subscription" && resp.StatusCode == 429 {
		return model + " (subscription)", nil
	}
	return "", anthropicError(resp)
}

func anthropicError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(b, &e)
	switch resp.StatusCode {
	case 401:
		return fmt.Errorf("that credential was rejected (401). It may be expired, revoked, or the wrong type.")
	case 403:
		return fmt.Errorf("access denied (403). The key may lack permission for this model.")
	case 429:
		return fmt.Errorf("rate limited / no quota (429). Check billing on this key.")
	}
	if e.Error.Message != "" {
		return fmt.Errorf("Anthropic rejected it: %s", e.Error.Message)
	}
	return fmt.Errorf("Anthropic returned HTTP %d", resp.StatusCode)
}

func validateOpenAI(key string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	req.Header.Set("authorization", "Bearer "+key)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach OpenAI: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "gpt (openai)", nil
	}
	if resp.StatusCode == 401 {
		return "", fmt.Errorf("OpenAI rejected that key (401).")
	}
	return "", fmt.Errorf("OpenAI returned HTTP %d", resp.StatusCode)
}

// validateCodexSubscription does a best-effort check on a ChatGPT/Codex
// subscription token. Like Claude subscription tokens, these are meant to flow
// through the Codex CLI, so raw-API validation is imperfect — a real token may
// return 200 or 429 (throttled); only a hard 401 is treated as a bad token.
// Actual briefings will route through the Codex CLI in v0.2.
func validateCodexSubscription(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	req.Header.Set("authorization", "Bearer "+token)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach OpenAI: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 429 {
		return "codex (subscription)", nil
	}
	if resp.StatusCode == 401 {
		return "", fmt.Errorf("that ChatGPT/Codex token was rejected (401). It may be expired — re-run `codex login` and paste the new token.")
	}
	return "", fmt.Errorf("OpenAI returned HTTP %d", resp.StatusCode)
}

func validateGemini(key string) (string, error) {
	req, _ := http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1beta/models?key="+key, nil)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach Gemini: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "gemini", nil
	}
	if resp.StatusCode == 400 || resp.StatusCode == 403 {
		return "", fmt.Errorf("Gemini rejected that key (%d).", resp.StatusCode)
	}
	return "", fmt.Errorf("Gemini returned HTTP %d", resp.StatusCode)
}

func validateOllama(baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	resp, err := httpc.Get(baseURL + "/api/tags")
	if err != nil {
		return "", fmt.Errorf("could not reach Ollama at %s: %v", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "ollama (local)", nil
	}
	return "", fmt.Errorf("Ollama returned HTTP %d", resp.StatusCode)
}

// ---- Telegram ----

func validateTelegram(token string) (string, error) {
	resp, err := httpc.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return "", fmt.Errorf("could not reach Telegram: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if !out.OK || out.Result.Username == "" {
		return "", fmt.Errorf("that bot token was rejected. Create one with @BotFather and paste the token it gives you.")
	}
	return out.Result.Username, nil
}

// telegramFindChat reads recent updates to capture the chat id of the user who
// messaged the bot — so they never have to copy a numeric id by hand.
func telegramFindChat(token string) (int64, string, error) {
	resp, err := httpc.Get("https://api.telegram.org/bot" + token + "/getUpdates")
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message struct {
				Chat struct {
					ID        int64  `json:"id"`
					FirstName string `json:"first_name"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	for i := len(out.Result) - 1; i >= 0; i-- {
		if id := out.Result[i].Message.Chat.ID; id != 0 {
			return id, out.Result[i].Message.Chat.FirstName, nil
		}
	}
	return 0, "", fmt.Errorf("no message yet — open Telegram, find @%s, and send it any message", "")
}

func telegramSend(token string, chatID int64, text string) error {
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	resp, err := httpc.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
