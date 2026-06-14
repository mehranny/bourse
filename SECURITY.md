# Security — how Bourse stores secrets

Your LLM key / subscription token and Telegram bot token are **encrypted at rest**
with AES-256-GCM (the same cipher OpenFang and IronClaw use), with per-secret
random nonces and AAD that binds each ciphertext to its slot (so a value sealed
as the LLM key can't be swapped into the Telegram slot). Secrets are **never**
returned by any HTTP endpoint, never logged, and never written in plaintext.

## The encryption key

- **Recommended:** set `BOURSE_SECRET_KEY` (any string; injected via your host's
  env / Docker secret). The key then lives off the data volume, so backups and
  snapshots of `./data` contain only ciphertext.
- **Fallback (zero-config):** if unset, Bourse generates a random key at
  `data/.key` (mode 0600). This still protects against logs, accidental file
  capture, and prompt-injection reads — but if a backup grabs both `.key` and
  `state.json`, the encryption is moot. Bourse logs a note when it uses this mode.

## Honest threat model

Encryption at rest defends the **offline** case — a stolen disk, leaked backup,
or copied volume. It does **not** defend against an attacker who already runs as
Bourse's user on the box, because the running process must be able to decrypt.
There, the real controls are: **don't expose the instance publicly without auth**
(use a tunnel / reverse proxy + auth), keep the OS patched, and the setup-code
gate on the wizard. We do not store raw tokens anywhere the model can read them.
