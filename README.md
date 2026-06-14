# Bourse — the market agent that keeps score

A self-hosted market agent that interviews you once, learns your risk style, and
sends a calibrated morning briefing to Telegram — then **logs every call it makes
before the outcome and grades it against what actually happened.** Most market
bots make confident calls and never check them. Bourse keeps receipts.

> ⚠️ Research and education, **not investment advice**. See [DISCLAIMER.md](DISCLAIMER.md).

## Why Bourse is different
- **It keeps score — honestly.** Every call is timestamped and hash-chained
  *before* the outcome, then scored (Brier) against real market closes. Not the
  model rating itself — the market.
- **Calibrated, not hype.** Probabilities sit near a coin-flip unless the evidence
  warrants more; never reflexively bullish.
- **Bring your own brain.** Ships with a free, transparent default brain (free
  data + your own LLM). A pluggable seam lets you drop in a heavier engine.
- **Yours, self-hosted.** One Go binary. Secrets encrypted at rest. No telemetry.

## Quick start
```bash
docker compose up --build
# open http://<host>:8080 and enter the SETUP CODE printed in the logs
```
The setup wizard needs **no LLM to run** — it validates your credential with one
real test call, so the instance can never sit silently half-broken.

## How it works
onboarding wizard → free **lite brain** (Yahoo prices + Google News + your LLM) →
calibrated calls → Telegram briefing → the **ledger** scores it against reality.

## Powering it
A Claude or ChatGPT subscription, an API key (Anthropic / OpenAI / Gemini), or
local Ollama.

## License
MIT.
