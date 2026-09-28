"""Pure in-process fakes: a Milvus client that only understands the store's
fixed filter templates (so a filter built by string splicing fails loudly
here), and a scripted extraction model."""

from __future__ import annotations

import itertools
import json
import re
import unicodedata

from ai_infer.mem.store import IDS_FILTER, OWNER_FILTER

_WORD = re.compile(r"\w+", re.UNICODE)


class FakeMilvusClient:
    instances: list["FakeMilvusClient"] = []

    def __init__(self, uri: str = "", token: str = "", db_name: str = ""):
        self.uri, self.db_name = uri, db_name
        self.collections: dict[str, dict[str, dict]] = {}
        self.calls: list[tuple] = []
        self.filters_seen: list[tuple[str, dict]] = []
        self.ignore_deletes = False  # simulates a store that silently keeps rows
        self._jobs = itertools.count(1)
        FakeMilvusClient.instances.append(self)

    # --- admin ---
    def has_collection(self, name):
        return name in self.collections

    def prepare_index_params(self):
        from pymilvus import MilvusClient

        return MilvusClient.prepare_index_params()

    def create_collection(
        self,
        collection_name,
        schema=None,
        index_params=None,
        consistency_level=None,
        **kw,
    ):
        self.calls.append(("create_collection", collection_name, consistency_level))
        self.collections[collection_name] = {}

    def list_collections(self):
        return list(self.collections)

    def flush(self, name):
        self.calls.append(("flush", name))

    def compact(self, name, is_l0=False, **kw):
        self.calls.append(("compact", name, "l0" if is_l0 else "mix"))
        return next(self._jobs)

    def get_compaction_state(self, job):
        return "Completed"

    # --- filters ---
    def _match(self, flt: str, params: dict):
        self.filters_seen.append((flt, dict(params or {})))
        if flt == OWNER_FILTER:
            return lambda r: r["owner"] == params["owner"]
        if flt == IDS_FILTER:
            return lambda r: r["id"] in params["ids"]
        raise AssertionError(f"filter outside the store's templates: {flt!r}")

    # --- data ---
    def insert(self, collection_name, data, **kw):
        for r in data:
            assert set(r) == {"id", "owner", "vectors", "metadata", "text"}, set(r)
            self.collections[collection_name][r["id"]] = json.loads(json.dumps(r))

    def upsert(self, collection_name, data, **kw):
        self.insert(collection_name, data)

    def delete(self, collection_name, filter=None, filter_params=None, ids=None, **kw):
        assert ids is None, "deletes go through the filter template"
        m = self._match(filter, filter_params)
        if self.ignore_deletes:
            return {"delete_count": 0}
        rows = self.collections[collection_name]
        gone = [k for k, r in rows.items() if m(r)]
        for k in gone:
            del rows[k]
        return {"delete_count": len(gone)}

    def query(
        self,
        collection_name,
        filter="",
        filter_params=None,
        output_fields=None,
        limit=None,
        **kw,
    ):
        m = self._match(filter, filter_params)
        rows = [r for r in self.collections[collection_name].values() if m(r)]
        if output_fields == ["count(*)"]:
            return [{"count(*)": len(rows)}]
        rows = rows[: limit or len(rows)]
        return [
            {"id": r["id"], **{f: r[f] for f in (output_fields or []) if f in r}}
            for r in rows
        ]

    def search(
        self,
        collection_name,
        data,
        anns_field,
        limit,
        filter,
        filter_params,
        output_fields,
        **kw,
    ):
        m = self._match(filter, filter_params)
        rows = [r for r in self.collections[collection_name].values() if m(r)]
        q = data[0]
        if anns_field == "vectors":
            scored = [(sum(a * b for a, b in zip(q, r["vectors"])), r) for r in rows]
        else:
            qt = set(_WORD.findall(unicodedata.normalize("NFC", q).casefold()))
            scored = [
                (len(qt & set(_WORD.findall(r["text"].casefold()))), r) for r in rows
            ]
            scored = [s for s in scored if s[0] > 0]
        scored.sort(key=lambda s: -s[0])
        return [
            [
                {
                    "id": r["id"],
                    "distance": float(s),
                    "entity": {"metadata": r["metadata"]},
                }
                for s, r in scored[:limit]
            ]
        ]


class ScriptedLlm:
    """Answers each extraction call with the next scripted list of candidate
    items (the model's own classification included)."""

    def __init__(self, *answers: list[dict]):
        self.answers = list(answers)
        self.calls: list[tuple[str, str, dict]] = []

    def complete(self, system: str, user: str, schema: dict) -> str:
        self.calls.append((system, user, schema))
        items = self.answers.pop(0) if self.answers else []
        return json.dumps({"memory": items}, ensure_ascii=False)


def own(text: str, loai: str = "thich_danh_muc") -> dict:
    return {
        "text": text,
        "ve_ai": "ban_than",
        "noi_dung": "so_thich_rang_buoc_di_choi",
        "loai": loai,
    }
