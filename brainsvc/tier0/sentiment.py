from __future__ import annotations

from functools import lru_cache

MODEL = "ProsusAI/finbert"  # labels: positive(0), negative(1), neutral(2)

@lru_cache(maxsize=1)
def _model():
    import torch  # noqa: F401  (lazy: keep heavy import out of module load)
    from transformers import AutoTokenizer, AutoModelForSequenceClassification
    tok = AutoTokenizer.from_pretrained(MODEL)
    mdl = AutoModelForSequenceClassification.from_pretrained(MODEL).eval()
    return tok, mdl

def score(headlines: list[str]) -> list[float]:
    """Signed sentiment per headline = P(pos) - P(neg), in [-1, 1]."""
    if not headlines:
        return []
    import torch
    tok, mdl = _model()
    enc = tok(headlines, return_tensors="pt", padding=True, truncation=True, max_length=128)
    with torch.no_grad():
        probs = torch.softmax(mdl(**enc).logits, dim=-1)
    return (probs[:, 0] - probs[:, 1]).tolist()
