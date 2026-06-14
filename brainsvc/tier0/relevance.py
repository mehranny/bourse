from __future__ import annotations

from brainsvc.tier0.rank import _embed

_ANCHOR = "financial markets, stocks, earnings, monetary policy, company performance"


def relevance(headline: str) -> float:
    """Cosine similarity to a finance anchor — a zero-shot relevance proxy.
    NOTE: approximation pending a real ModernBERT credibility fine-tune."""
    v = _embed([_ANCHOR, headline])
    return (v[1] @ v[0]).item()
