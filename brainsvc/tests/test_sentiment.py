from __future__ import annotations

import pytest
from brainsvc.tier0.sentiment import score

@pytest.mark.model  # requires model download; run with -m model
def test_sentiment_direction():
    s = score(["Company beats earnings and raises guidance",
               "Firm slashes outlook amid plunging demand"])
    assert s[0] > 0 and s[1] < 0
