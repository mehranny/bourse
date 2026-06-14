from __future__ import annotations

import os
from fastapi import FastAPI
from brainsvc.contract import ResearchRequest, ResearchBundle, Call
from brainsvc.gather import build_context
from brainsvc.reason import reason
from brainsvc.calibrate import calibrate

app = FastAPI(title="bourse brainsvc")

LOADED: list[str] = []  # populated as Tier-0 modules load (later tasks)

@app.get("/health")
def health():
    return {"status": "ok", "models_loaded": LOADED}

@app.post("/research", response_model=ResearchBundle)
def research(req: ResearchRequest):
    context, ref = build_context(req.watchlist)
    bundle = reason(context, req.profile.model_dump())
    calls = []
    for c in bundle.get("calls", []):
        sym = c.get("symbol", "").upper()
        c["ref_price"] = ref.get(sym, 0.0)
        c["prob"] = calibrate(float(c.get("prob", 0.5)), params=None)  # cold start
        calls.append(Call(**{k: v for k, v in c.items() if k in Call.model_fields}))
    if not LOADED:
        LOADED[:] = ["finbert", "bge-small"]  # reflect tier-0 models used by gather
    return ResearchBundle(summary=bundle.get("summary", ""), calls=calls)
