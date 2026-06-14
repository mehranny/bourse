from __future__ import annotations

import pytest
from brainsvc.tier0.claim import supported


@pytest.mark.model
def test_claim_consistency():
    doc = "Nvidia reported record data-center revenue and raised guidance."
    assert supported(doc, "Nvidia raised its guidance.") > 0.5
    assert supported(doc, "Nvidia filed for bankruptcy.") < 0.5
