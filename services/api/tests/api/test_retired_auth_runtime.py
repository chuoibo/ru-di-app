"""Retired auth writers cannot be reached through the Python fallback."""

from fastapi.testclient import TestClient

from app.api.main import create_app


def test_runtime_does_not_register_retired_account_writers():
    application = create_app(auth_mode="prod")
    paths = application.openapi()["paths"]
    for path in (
        "/auth/otp/request",
        "/auth/otp/verify",
        "/auth/google",
        "/identity/person-id",
        "/friends/lookup",
    ):
        assert path not in paths
    assert "post" not in paths["/sessions"]
    with TestClient(application) as client:
        for path in ("/auth/otp/request", "/identity/person-id", "/auth/google"):
            assert client.post(path, json={}).status_code == 404
