from __future__ import annotations

from functools import lru_cache

MODEL = "BAAI/bge-small-en-v1.5"

@lru_cache(maxsize=1)
def _model():
    from transformers import AutoTokenizer, AutoModel
    tok = AutoTokenizer.from_pretrained(MODEL)
    mdl = AutoModel.from_pretrained(MODEL).eval()
    return tok, mdl

def _embed(texts: list[str]):
    import torch
    tok, mdl = _model()
    enc = tok(texts, return_tensors="pt", padding=True, truncation=True, max_length=128)
    with torch.no_grad():
        out = mdl(**enc).last_hidden_state[:, 0]  # CLS
    return torch.nn.functional.normalize(out, dim=-1)

def top_k(query: str, headlines: list[str], k: int = 4) -> list[str]:
    if len(headlines) <= k:
        return headlines
    vecs = _embed([query] + headlines)
    sims = (vecs[1:] @ vecs[0]).tolist()
    ranked = [h for _, h in sorted(zip(sims, headlines), key=lambda p: p[0], reverse=True)]
    return ranked[:k]
