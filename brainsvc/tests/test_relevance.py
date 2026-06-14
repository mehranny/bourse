from __future__ import annotations

import pytest
from brainsvc.tier0.relevance import relevance


@pytest.mark.model
def test_relevance_filters_offtopic():
    fin = relevance("Fed signals rate cut affecting bank stocks")
    off = relevance("Recipe for chocolate chip cookies")
    assert fin > off
