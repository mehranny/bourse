// Package llm is the model-agnostic generation client. API-key providers go
// over HTTP; subscription tokens (Claude/Codex) route through their CLI, because
// the raw API throttles subscription tokens — generation must use the client.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"bourse/internal/store"
)

var httpc = &http.Client{Timeout: 90 * time.Second}

// Generate runs the prompt against whatever the user configured.
func Generate(ctx context.Context, cfg store.LLMConfig, secret, dataDir, prompt string) (string, error) {
	switch {
	case cfg.Mode == "subscription" && cfg.Provider == "anthropic":
		return claudeCLI(ctx, secret, dataDir, prompt)
	case cfg.Mode == "subscription" && cfg.Provider == "openai-codex":
		return codexCLI(ctx, secret, dataDir, prompt)
	case cfg.Provider == "anthropic":
		return anthropicAPI(ctx, secret, prompt)
	case cfg.Provider == "openai":
		return openaiAPI(ctx, secret, prompt)
	case cfg.Provider == "gemini":
		return geminiAPI(ctx, secret, prompt)
	case cfg.Provider == "ollama":
		return ollama(ctx, secret, prompt)
	default:
		return "", fmt.Errorf("unsupported provider %q/%q", cfg.Mode, cfg.Provider)
	}
}

func claudeCLI(ctx context.Context, token, dataDir, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt,
		"--model", "claude-sonnet-4-6", "--dangerously-skip-permissions")
	cmd.Env = []string{
		"CLAUDE_CODE_OAUTH_TOKEN=" + token,
		"HOME=" + dataDir,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude cli: %v", err)
	}
	return string(out), nil
}

func codexCLI(ctx context.Context, token, dataDir, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "codex", "exec", prompt)
	cmd.Env = []string{
		"CODEX_API_KEY=" + token,
		"HOME=" + dataDir,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("codex cli: %v", err)
	}
	return string(out), nil
}

func anthropicAPI(ctx context.Context, key, prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": "claude-sonnet-4-6", "max_tokens": 4096,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", key)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("anthropic %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return out.Content[0].Text, nil
}

func openaiAPI(ctx context.Context, key, prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": "gpt-4o", "messages": []map[string]string{{"role": "user", "content": prompt}},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+key)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("openai %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return out.Choices[0].Message.Content, nil
}

func geminiAPI(ctx context.Context, key, prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{{"parts": []map[string]string{{"text": prompt}}}},
	})
	u := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + key
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}

func ollama(ctx context.Context, baseURL, prompt string) (string, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	body, _ := json.Marshal(map[string]any{"model": "llama3.1", "prompt": prompt, "stream": false})
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/generate", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Response string `json:"response"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Response, nil
}
