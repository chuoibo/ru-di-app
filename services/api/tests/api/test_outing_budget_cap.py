"""An outing budget is bounded like any amount the ledger takes (audit
2026-10-05, PER-FE-MONEY-01): with at most 1000 people, budget x headcount
then stays a safe integer for the app. The Go front door reads the same bound
from the contract IR; parity w7/outings holds the two answers equal."""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.api.schemas import OutingCreateRequest
from app.domain.contract import MAX_AMOUNT_VND


def _body(budget):
    return {
        "title": "Chuyến thử (dữ liệu mẫu)",
        "starts_on": "2026-03-14",
        "ends_on": "2026-03-16",
        "headcount": 1000,
        "budget_per_person_vnd": budget,
    }


def test_the_cap_itself_is_accepted_and_the_product_stays_safe():
    request = OutingCreateRequest.model_validate(_body(MAX_AMOUNT_VND))
    assert request.budget_per_person_vnd * request.headcount <= 2**53 - 1


def test_one_dong_past_the_cap_is_refused():
    with pytest.raises(ValidationError) as caught:
        OutingCreateRequest.model_validate(_body(MAX_AMOUNT_VND + 1))
    assert caught.value.errors()[0]["type"] == "less_than_equal"
