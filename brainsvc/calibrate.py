from __future__ import annotations

import math

def calibrate(prob: float, params: dict | None) -> float:
    """Platt scaling on the probability. params=None -> identity (cold start).
    Fitted form: sigmoid(A * logit(p) + B)."""
    if params is None:
        return prob
    p = min(max(prob, 1e-6), 1 - 1e-6)
    logit = math.log(p / (1 - p))
    z = params["A"] * logit + params["B"]
    return 1.0 / (1.0 + math.exp(-z))
