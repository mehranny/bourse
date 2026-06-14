from __future__ import annotations

import pytest
from brainsvc.tier0.rank import top_k

@pytest.mark.model
def test_rank_orders_by_relevance():
    headlines = ["NVDA unveils new AI GPU", "Local bakery wins award", "Nvidia earnings beat"]
    out = top_k("NVDA", headlines, k=2)
    assert len(out) == 2
    assert "bakery" not in " ".join(out).lower()
