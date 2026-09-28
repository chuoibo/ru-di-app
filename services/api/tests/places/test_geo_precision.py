"""`mappable_point`: the line between «has coordinates» and «is here».

Same table as the Go `repo.Place.MappablePoint` test and the phone's
`veDuocLenBanDo` tests, so the three halves cannot drift apart unnoticed.
"""

from __future__ import annotations

import pytest

from app.places.geo_precision import mappable_point

POINT = {"lat": 10.7769, "lng": 106.7009}


@pytest.mark.parametrize("precision", ["rooftop", "street", None])
def test_a_rooftop_street_or_legacy_point_is_kept(precision):
    assert mappable_point({**POINT, "geo_precision": precision}) == (10.7769, 106.7009)


@pytest.mark.parametrize(
    "precision", ["ward_centroid", "province_centroid", "suy_luan", "none"]
)
def test_a_centroid_or_a_guess_is_no_point(precision):
    assert mappable_point({**POINT, "geo_precision": precision}) is None


@pytest.mark.parametrize(
    "place",
    [
        {"lat": None, "lng": None, "geo_precision": "rooftop"},
        {"lat": 10.7769, "lng": None, "geo_precision": "rooftop"},
        {"geo_precision": "rooftop"},
    ],
)
def test_no_point_or_half_a_point_is_no_point(place):
    assert mappable_point(place) is None
