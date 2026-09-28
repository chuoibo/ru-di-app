from __future__ import annotations

import os
import sys
import tempfile

# Before anything imports ai_infer.mem (and through it mem0): a private
# MEM0_DIR per test session, so tests can check nothing was written there.
MEM0_DIR = tempfile.mkdtemp(prefix="ai-infer-test-mem0-")
os.environ["MEM0_DIR"] = MEM0_DIR
sys.path.insert(0, os.path.dirname(__file__))

import pytest  # noqa: E402

TOKEN = "t" * 40


def pytest_collection_modifyitems(config, items):
    # The live tier: skipped without a Milvus, and a FAILURE instead of a
    # skip when AI_INFER_REQUIRE_MILVUS=1 (skip is not green).
    uri = os.environ.get("AI_INFER_TEST_MILVUS_URI")
    required = os.environ.get("AI_INFER_REQUIRE_MILVUS") == "1"
    if required and not uri:
        raise pytest.UsageError(
            "AI_INFER_REQUIRE_MILVUS=1 but AI_INFER_TEST_MILVUS_URI is not set"
        )
    for item in items:
        if "milvus" in item.keywords and not uri:
            item.add_marker(pytest.mark.skip(reason="AI_INFER_TEST_MILVUS_URI not set"))


@pytest.fixture
def settings():
    from ai_infer import config

    return config.Settings(
        token=TOKEN, sparse_mode="stub", gemini_mode="stub", milvus_uri="fake://"
    )


@pytest.fixture
def fake_store(monkeypatch):
    from ai_infer.mem import store
    from fakes import FakeMilvusClient

    FakeMilvusClient.instances.clear()
    monkeypatch.setattr(store, "client_factory", FakeMilvusClient)
    return FakeMilvusClient


def build_service(llm, embed=None):
    from ai_infer.mem import bootstrap, gemini
    from ai_infer.mem.service import MemoryService

    m = bootstrap.build(
        llm=llm,
        embed=embed or gemini.StubEmbed(),
        milvus_uri="fake://",
        milvus_token="",
        db_name="nep_memory",
        collection="memories_v1",
    )
    return MemoryService(m)


@pytest.fixture
def auth():
    return {"Authorization": f"Bearer {TOKEN}"}
