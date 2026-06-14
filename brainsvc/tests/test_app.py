from __future__ import annotations

from fastapi.testclient import TestClient
from brainsvc.app import app

client = TestClient(app)

def test_health():
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"

def test_research_stub(monkeypatch):
    monkeypatch.setattr("brainsvc.app.build_context",
                        lambda wl: ("## NVDA\nprice 100\n- stub", {"NVDA": 100.0}))
    monkeypatch.setattr("brainsvc.app.reason",
                        lambda ctx, prof: {"summary": "stub",
                            "calls": [{"symbol": "NVDA", "direction": "up",
                                       "horizon": "5d", "prob": 0.5}]})
    r = client.post("/research", json={"watchlist": ["NVDA"]})
    assert r.status_code == 200
    body = r.json()
    assert "summary" in body and "calls" in body

def test_research_pipeline(monkeypatch):
    monkeypatch.setattr("brainsvc.app.build_context",
                        lambda wl: ("## NVDA\nprice 100\n- NVDA up on AI", {"NVDA": 100.0}))
    monkeypatch.setattr("brainsvc.app.reason",
                        lambda ctx, prof: {"summary": "s",
                            "calls": [{"symbol": "NVDA", "direction": "up",
                                       "horizon": "5d", "prob": 0.6}]})
    r = client.post("/research", json={"watchlist": ["NVDA"]})
    assert r.status_code == 200
    c = r.json()["calls"][0]
    assert c["symbol"] == "NVDA"
    assert c["ref_price"] == 100.0
