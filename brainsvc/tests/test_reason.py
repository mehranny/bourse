from __future__ import annotations

from brainsvc.reason import parse_bundle, build_prompt

def test_parse_bundle_tolerates_fences():
    raw = '```json\n{"summary":"s","calls":[{"symbol":"NVDA","direction":"up","horizon":"5d","prob":0.6}]}\n```'
    b = parse_bundle(raw)
    assert b["summary"] == "s"
    assert b["calls"][0]["symbol"] == "NVDA"

def test_build_prompt_includes_signals():
    p = build_prompt("## NVDA\nprice 100", {"risk": "balanced", "depth": "standard"})
    assert "NVDA" in p and "JSON" in p
