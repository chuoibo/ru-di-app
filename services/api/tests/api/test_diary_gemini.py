"""The diary brain forwards only explicit sources and selected image bytes."""

import base64
import json
from types import SimpleNamespace

import pytest

from app.api.diary_gemini import compose_diary


def test_selected_images_and_bundle_reach_model_without_repository(monkeypatch):
    monkeypatch.setenv("GEMINI_API_KEY", "synthetic-test-key")
    captured = {}
    calls = []
    result = {"title": "Một chiều", "pages": [], "ai_generated": True}

    class Client:
        def __init__(self, **kwargs):
            self.models = self

        def __enter__(self):
            return self

        def __exit__(self, *args):
            return False

        def generate_content(self, **kwargs):
            calls.append(kwargs)
            if len(calls) == 2:
                return SimpleNamespace(text='{"grounded": true}')
            captured.update(kwargs)
            return SimpleNamespace(text=json.dumps(result))

    monkeypatch.setattr("app.api.diary_gemini.genai.Client", Client)
    source = {"title": "Một chiều", "excerpts": ["Chuyện được chọn"]}
    body = {
        "source": source,
        "images": [
            {
                "id": "selected",
                "mime": "image/png",
                "data": base64.b64encode(b"synthetic-image").decode(),
            }
        ],
    }
    assert compose_diary(body) == result
    parts = captured["contents"][0].parts
    assert json.loads(parts[0].text) == source
    assert parts[1].text == "Photo ID: selected"
    assert parts[2].inline_data.data == b"synthetic-image"
    assert captured["config"].response_mime_type == "application/json"
    assert calls[1]["contents"][0].parts[:-1] == parts
    assert (
        json.loads(
            calls[1]["contents"][0].parts[-1].text.removeprefix("Proposed diary: ")
        )
        == result
    )


@pytest.mark.parametrize("verdict", [{"grounded": False}, {}, {"grounded": "true"}])
def test_unsubstantiated_or_malformed_grounding_verdict_refuses_draft(
    monkeypatch, verdict
):
    monkeypatch.setenv("GEMINI_API_KEY", "synthetic-test-key")

    class Client:
        def __init__(self, **kwargs):
            self.models = self
            self.calls = 0

        def __enter__(self):
            return self

        def __exit__(self, *args):
            return False

        def generate_content(self, **kwargs):
            self.calls += 1
            return SimpleNamespace(
                text=json.dumps(
                    {"title": "Invented first day", "pages": []}
                    if self.calls % 2 == 1
                    else verdict
                )
            )

    monkeypatch.setattr("app.api.diary_gemini.genai.Client", Client)
    with pytest.raises(ValueError, match="ungrounded_diary_result"):
        compose_diary({"source": {}, "images": []})


def test_missing_provider_is_not_a_fabricated_success(monkeypatch):
    monkeypatch.delenv("GEMINI_API_KEY", raising=False)
    with pytest.raises(RuntimeError, match="diary_ai_not_configured"):
        compose_diary({"source": {}, "images": []})


@pytest.mark.parametrize(
    "images",
    [
        [{}] * 41,
        [{"id": "x", "mime": "text/html", "data": ""}],
        [{"id": "x", "mime": "image/png", "data": "not base64!"}],
    ],
)
def test_invalid_image_bundle_never_calls_provider(monkeypatch, images):
    monkeypatch.setenv("GEMINI_API_KEY", "synthetic-test-key")

    def forbidden(**kwargs):
        pytest.fail("Invalid input reached the provider")

    monkeypatch.setattr("app.api.diary_gemini.genai.Client", forbidden)
    with pytest.raises(ValueError):
        compose_diary({"source": {}, "images": images})


def test_rejected_draft_is_rewritten_once_and_checked_again(monkeypatch):
    monkeypatch.setenv("GEMINI_API_KEY", "synthetic-test-key")
    calls = []
    accepted = {"title": "Những bức vẽ", "pages": [], "ai_generated": True}
    replies = iter(
        [
            {"title": "A mountain visit", "pages": []},
            {"grounded": False, "reason": "Illustrations do not prove a visit"},
            accepted,
            {"grounded": True},
        ]
    )

    class Client:
        def __init__(self, **kwargs):
            self.models = self
            assert kwargs["http_options"].timeout == 12000
            assert kwargs["http_options"].retry_options.attempts == 1

        def __enter__(self):
            return self

        def __exit__(self, *args):
            return False

        def generate_content(self, **kwargs):
            calls.append(kwargs)
            return SimpleNamespace(text=json.dumps(next(replies)))

    monkeypatch.setattr("app.api.diary_gemini.genai.Client", Client)
    source = {"excerpts": ["We drew illustrations at home."]}
    assert compose_diary({"source": source, "images": []}) == accepted
    assert len(calls) == 4
    assert json.loads(calls[2]["contents"][0].parts[0].text) == source
    assert (
        "Illustrations do not prove a visit" in calls[2]["contents"][0].parts[-1].text
    )
    assert "Những bức vẽ" in calls[3]["contents"][0].parts[-1].text
