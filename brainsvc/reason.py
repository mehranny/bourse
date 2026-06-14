from __future__ import annotations

import json
import re
import subprocess

def build_prompt(context: str, profile: dict) -> str:
    return (
        "You are a disciplined market analyst. Given the data and Tier-0 signals, "
        "produce calibrated next-move calls.\n"
        f"RISK: {profile.get('risk','balanced')}  DEPTH: {profile.get('depth','standard')}\n\n"
        f"DATA:\n{context}\n\n"
        'Return ONLY JSON: {"summary":"2-3 sentences","calls":[{"symbol":"NVDA",'
        '"direction":"up","horizon":"5d","prob":0.58,"confidence":"medium",'
        '"rationale":"one sentence","evidence":["a fact you used"]}]}\n'
        "Rules: prob = P(direction correct) in 0.10..0.90; anchor near 0.50; "
        "one call per ticker; never reflexively bullish."
    )

def parse_bundle(raw: str) -> dict:
    m = re.search(r"\{.*\}", raw, re.DOTALL)
    if not m:
        raise ValueError("no JSON object in model output")
    return json.loads(m.group(0))

def reason(context: str, profile: dict, model: str = "claude-sonnet-4-6") -> dict:
    prompt = build_prompt(context, profile)
    out = subprocess.run(
        ["claude", "-p", prompt, "--model", model],
        capture_output=True, text=True, timeout=420,
    )
    if out.returncode != 0:
        raise RuntimeError(f"claude CLI failed: {out.stderr[:500]}")
    return parse_bundle(out.stdout)
