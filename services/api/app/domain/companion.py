"""Pure grounding rules for AI cards.

The model is allowed to choose catalogue identifiers, but it is never trusted
to describe a place. Rebuilding every card here makes that boundary structural:
only server-owned catalogue facts can reach a client, regardless of extra keys
or persuasive prose returned by the model.
"""

from __future__ import annotations

MAX_PLACES = 5
MAX_STOPS = 6
MAX_TEXT = 600


class CompanionError(Exception):
    """A stable refusal code that exposes no conversation or model output."""

    def __init__(self, code: str) -> None:
        super().__init__(code)
        self.code = code


def _malformed() -> CompanionError:
    return CompanionError("companion_card_malformed")


def _bounded_text(payload: dict, key: str) -> str:
    value = payload.get(key)
    if not isinstance(value, str):
        raise _malformed()
    return value[:MAX_TEXT]


def _optional_text(payload: dict, key: str) -> str:
    """Keep absent presentation prose distinct from a broken field value.

    A missing heading costs only decoration, so discarding fully grounded stops
    or places for it would trade useful catalogue-backed content for silence. A
    present non-string is different: it violates the field's type contract and
    must still make the card malformed.
    """
    if key not in payload:
        return ""
    return _bounded_text(payload, key)


def _catalogue_by_id(allowed_places: list[dict]) -> dict[str, dict]:
    return {
        place["id"]: place
        for place in allowed_places
        if isinstance(place, dict) and isinstance(place.get("id"), str)
    }


def _ground_places(payload: dict, catalogue: dict[str, dict]) -> dict:
    intro = _optional_text(payload, "intro")
    place_ids = payload.get("place_ids")
    if not isinstance(place_ids, list) or not all(
        isinstance(place_id, str) for place_id in place_ids
    ):
        raise _malformed()

    # Validate the complete model response before truncating it. An invented
    # sixth identifier must sink the card, not disappear behind MAX_PLACES.
    if any(place_id not in catalogue for place_id in place_ids):
        raise CompanionError("companion_place_not_in_catalogue")

    unique_ids: list[str] = []
    seen: set[str] = set()
    for place_id in place_ids:
        if place_id not in seen:
            seen.add(place_id)
            unique_ids.append(place_id)

    selected = unique_ids[:MAX_PLACES]
    if not selected:
        raise CompanionError("companion_card_empty")
    payload = {
        "intro": intro,
        "places": [dict(catalogue[place_id]) for place_id in selected],
    }
    # Counted against the deduplicated list: a repeated id is normalisation, not
    # a place the card withheld, and a false "còn N chỗ nữa" teaches the group
    # to ignore the notice.
    omitted = len(unique_ids) - len(selected)
    if omitted:
        payload["omitted_place_count"] = omitted
    return {"kind": "places", "payload": payload}


def _ground_itinerary(payload: dict, catalogue: dict[str, dict]) -> dict:
    title = _optional_text(payload, "title")
    raw_stops = payload.get("stops")
    if not isinstance(raw_stops, list):
        raise _malformed()

    stops: list[tuple[str, str, str]] = []
    for raw_stop in raw_stops:
        if not isinstance(raw_stop, dict):
            raise _malformed()
        place_id = raw_stop.get("place_id")
        if not isinstance(place_id, str):
            raise _malformed()
        time_text = _bounded_text(raw_stop, "time_text")
        note = _bounded_text(raw_stop, "note")
        stops.append((place_id, time_text, note))

    # Check all mentioned IDs before applying the display limit for the same
    # fail-closed reason as a places card.
    if any(place_id not in catalogue for place_id, _, _ in stops):
        raise CompanionError("companion_place_not_in_catalogue")
    if not stops:
        raise CompanionError("companion_card_empty")

    shown = stops[:MAX_STOPS]
    payload = {
        "title": title,
        "stops": [
            {
                "time_text": time_text,
                "note": note,
                "place": dict(catalogue[place_id]),
            }
            for place_id, time_text, note in shown
        ],
    }
    # The model is asked for at most MAX_STOPS (see app/api/companion_gemini.py),
    # so this slice is the fallback for a model that answered past its schema --
    # not the routine path. Either way the card admits the cut: "ghi rõ từng
    # khung giờ của cả hai ngày" is an ordinary request, and a second day that
    # vanishes without a word reads as a complete plan.
    omitted = len(stops) - len(shown)
    if omitted:
        payload["omitted_stop_count"] = omitted
    return {"kind": "itinerary", "payload": payload}


def ground_card(raw: dict, allowed_places: list[dict]) -> dict:
    """Rebuild one model card from contract fields and server-owned facts.

    Unknown keys are never copied. This whitelist is the money and
    anti-fabrication boundary: a new model field cannot become a client feature
    until a human deliberately adds it here.
    """

    if not isinstance(raw, dict) or "kind" not in raw:
        raise _malformed()
    payload = raw.get("payload")
    if not isinstance(payload, dict):
        raise _malformed()

    kind = raw["kind"]
    if kind not in {"text", "places", "itinerary"}:
        raise CompanionError("companion_card_kind_unknown")

    if kind == "text":
        text = _bounded_text(payload, "text")
        if not text.strip():
            raise CompanionError("companion_card_empty")
        return {"kind": "text", "payload": {"text": text}}

    catalogue = _catalogue_by_id(allowed_places)
    if kind == "places":
        return _ground_places(payload, catalogue)
    return _ground_itinerary(payload, catalogue)
