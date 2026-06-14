# Bourse — the market agent that keeps score

A self-hosted market agent that interviews you once, learns your risk style,
and sends a calibrated morning briefing to Telegram — then **logs every call it
makes before the outcome and grades it against what actually happened.** Most
market bots make confident calls and never check them. Bourse keeps receipts.

> ⚠️ **Research and education, not investment advice.** See [DISCLAIMER.md](DISCLAIMER.md).

This is the **private development repo** (`bourse-dev`). The public release is a
clean drop — secrets and test history never leave here.

## Status: v0.1 — onboarding wizard

The front door, built first and on purpose. A **static web wizard** that needs
no LLM to run (the failure that kills most self-hosted agents is requiring the
LLM in order to set up the LLM). It:

- runs the moment the container boots, on the IP you get from your host
- asks **subscription or API** and validates the credential with one real test
  call — so you learn immediately if it works, never a silent half-broken state
- collects your Telegram bot token and auto-captures your chat id (no copying ids)
- sets your risk profile, watchlist, and briefing time with buttons
- refuses to finish until LLM ✓ and Telegram ✓ are both green

Briefing generation, the scoring ledger, and the pluggable 47-layer brain land
in v0.2–v0.3 (see the proposal).

## Run it

```bash
docker compose up --build
# open http://<host>:8080  and enter the SETUP CODE printed in the logs
```

## Layout

```
cmd/bourse/         entrypoint
internal/setup/     the onboarding wizard (server + validation + embedded web UI)
```

Built with the Go static-binary pattern; web assets are embedded, so the image
is one small binary with no runtime deps.

## Optional: local sentiment model (FinBERT)

The onboarding wizard offers a **"Local sentiment model"** step. If you enable it,
Bourse downloads ~440 MB of FinBERT weights into `data/models/finbert` and runs
inference on CPU alongside the LLM read. It is entirely opt-in — skip it and
Bourse works fine using the LLM-only signal.

**Docker users** — nothing to do. The ONNX Runtime shared library is baked into
the image (`/usr/local/lib/libonnxruntime.so`, ORT 1.20.1 x64).

**Bare-metal `go build` users** — install the ORT shared library separately, then
point Bourse at it:

```bash
# example — adjust version/arch as needed
ORT_VERSION=1.20.1
curl -fsSL -o /tmp/ort.tgz \
  https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-x64-${ORT_VERSION}.tgz
tar -xzf /tmp/ort.tgz -C /tmp
sudo cp /tmp/onnxruntime-linux-x64-${ORT_VERSION}/lib/libonnxruntime.so* /usr/local/lib/
sudo ldconfig

export BOURSE_ORT_LIB=/usr/local/lib/libonnxruntime.so
```

macOS (Homebrew): `brew install onnxruntime` then set
`BOURSE_ORT_LIB=$(brew --prefix onnxruntime)/lib/libonnxruntime.dylib`.
