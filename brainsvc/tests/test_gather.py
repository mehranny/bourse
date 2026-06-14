from __future__ import annotations

from brainsvc.gather import build_context

def test_build_context_shapes(monkeypatch):
    monkeypatch.setattr("brainsvc.gather.quote", lambda s: {"price": 100.0, "change_pct": 1.2})
    monkeypatch.setattr("brainsvc.gather.headlines", lambda s, n: [f"{s} up on news"])
    monkeypatch.setattr("brainsvc.gather._sentiment", lambda hs: [0.0] * len(hs))
    monkeypatch.setattr("brainsvc.gather._rank", lambda sym, hs, k: hs)
    ctx, ref = build_context(["NVDA"])
    assert "NVDA" in ctx
    assert ref["NVDA"] == 100.0

def test_enriched_context_tags_sentiment(monkeypatch):
    monkeypatch.setattr("brainsvc.gather.quote", lambda s: {"price": 100.0, "change_pct": 0.0})
    monkeypatch.setattr("brainsvc.gather.headlines", lambda s, n: ["NVDA soars on AI demand"])
    monkeypatch.setattr("brainsvc.gather._sentiment", lambda hs: [0.8])
    monkeypatch.setattr("brainsvc.gather._rank", lambda sym, hs, k: hs)
    ctx, _ = build_context(["NVDA"])
    assert "+0.8" in ctx or "0.80" in ctx
