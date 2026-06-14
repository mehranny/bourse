from __future__ import annotations

from brainsvc.calibrate import calibrate

def test_cold_start_is_identity():
    assert calibrate(0.6, params=None) == 0.6

def test_platt_applies_when_fitted():
    out = calibrate(0.5, params={"A": 1.0, "B": -1.0})
    assert 0.0 < out < 0.5
