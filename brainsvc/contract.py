from __future__ import annotations

from pydantic import BaseModel

class Profile(BaseModel):
    risk: str = "balanced"
    depth: str = "standard"

class ResearchRequest(BaseModel):
    watchlist: list[str]
    profile: Profile = Profile()

class Call(BaseModel):
    symbol: str
    direction: str          # "up" | "down"
    horizon: str            # e.g. "5d"
    prob: float
    confidence: str = "medium"
    rationale: str = ""
    evidence: list[str] = []
    ref_price: float = 0.0

class ResearchBundle(BaseModel):
    summary: str
    calls: list[Call]
