"""mem0's vector store, replaced: the same method surface as
mem0.vector_stores.milvus.MilvusDB, with these differences (research
milvus.md §5, mem0.md §2.3 and §6):

- the owner is a real scalar field (partition key, INVERTED index), not a
  key inside the JSON metadata;
- every filter is the one fixed expression «owner == {owner}» with the value
  passed as a template parameter, never spliced into the string;
- a filter without exactly one owner is refused (no "*" wildcard, no other
  keys), so nothing lists, searches or deletes across people;
- the collection is Strong-consistency, HNSW/COSINE on 1536 dims, and its
  BM25 text uses the same analyzer as the places collection (standard +
  lowercase + asciifolding on NFC text);
- reset() is refused: mem0's reset drops every person's memories.

Counting, owner-wide delete, flush and compaction exist for the deletion
saga: a delete is only reported done after a Strong count reads zero.
"""

from __future__ import annotations

import time
import unicodedata
from typing import Any, Callable, Optional

from pydantic import BaseModel

from ai_infer.mem import extract

OWNER_FILTER = "owner == {owner}"
IDS_FILTER = "id in {ids}"
ANALYZER = {"tokenizer": "standard", "filter": ["lowercase", "asciifolding"]}
MAX_OWNER = 128
MAX_TEXT = 4096
MAX_LIST = 16384


class OwnerError(ValueError):
    """A filter or payload without exactly one well-formed owner."""


class OutputData(BaseModel):
    id: Optional[str]
    score: Optional[float]
    payload: Optional[dict]


def check_owner(owner: Any) -> str:
    if not isinstance(owner, str) or not (0 < len(owner) <= MAX_OWNER):
        raise OwnerError("owner id empty, too long or not a string")
    if any(c.isspace() or unicodedata.category(c).startswith("C") for c in owner):
        raise OwnerError("owner id has whitespace or control characters")
    return owner


def _default_client(uri: str, token: str, db_name: str):
    from pymilvus import MilvusClient

    return MilvusClient(uri=uri, token=token, db_name=db_name)


# Replaced by tests with an in-memory fake; production uses pymilvus.
client_factory: Callable[[str, str, str], Any] = _default_client


class NepMilvus:
    def __init__(
        self,
        url: str,
        token: str,
        collection_name: str,
        embedding_model_dims: int,
        metric_type: Any,
        db_name: str,
    ) -> None:
        self.collection_name = collection_name
        self.embedding_model_dims = embedding_model_dims
        self.metric_type = "COSINE"
        self.client = client_factory(url, token, db_name)
        self._has_bm25_schema = True
        self.create_col(collection_name, embedding_model_dims)

    # --- schema ------------------------------------------------------------

    def create_col(
        self, collection_name: str, vector_size: int, metric_type: Any = None
    ) -> None:
        if self.client.has_collection(collection_name):
            return
        from pymilvus import DataType, Function, FunctionType, MilvusClient

        schema = MilvusClient.create_schema(auto_id=False, enable_dynamic_field=False)
        schema.add_field("id", DataType.VARCHAR, is_primary=True, max_length=64)
        schema.add_field(
            "owner", DataType.VARCHAR, max_length=MAX_OWNER, is_partition_key=True
        )
        schema.add_field("vectors", DataType.FLOAT_VECTOR, dim=vector_size)
        schema.add_field("metadata", DataType.JSON)
        schema.add_field(
            "text",
            DataType.VARCHAR,
            max_length=MAX_TEXT,
            enable_analyzer=True,
            analyzer_params=ANALYZER,
        )
        schema.add_field("sparse", DataType.SPARSE_FLOAT_VECTOR)
        schema.add_function(
            Function(
                name="bm25",
                input_field_names=["text"],
                output_field_names=["sparse"],
                function_type=FunctionType.BM25,
            )
        )
        idx = self.client.prepare_index_params()
        idx.add_index(
            field_name="vectors",
            index_type="HNSW",
            metric_type="COSINE",
            index_name="vectors_hnsw",
            params={"M": 16, "efConstruction": 200},
        )
        idx.add_index(
            field_name="sparse", index_type="SPARSE_INVERTED_INDEX", metric_type="BM25"
        )
        idx.add_index(field_name="owner", index_type="INVERTED")
        self.client.create_collection(
            collection_name=collection_name,
            schema=schema,
            index_params=idx,
            consistency_level="Strong",
        )

    # --- filters -----------------------------------------------------------

    @staticmethod
    def _owner_of(filters: Optional[dict]) -> str:
        if not isinstance(filters, dict) or set(filters) != {"user_id"}:
            raise OwnerError("memory filters must be exactly {user_id}")
        return check_owner(filters["user_id"])

    # --- mem0 surface ------------------------------------------------------

    def _record(self, vid: str, vector: list, payload: dict) -> dict:
        owner = check_owner((payload or {}).get("user_id"))
        payload = dict(payload)
        kind = extract.kept_kind(payload.get("data", ""))
        if kind and "loai" not in payload:
            payload["loai"] = kind
        text = unicodedata.normalize("NFC", payload.get("data", ""))[:MAX_TEXT]
        return {
            "id": vid,
            "owner": owner,
            "vectors": vector,
            "metadata": payload,
            "text": text,
        }

    def insert(self, ids, vectors, payloads, **kwargs) -> None:
        data = [self._record(i, v, p) for i, v, p in zip(ids, vectors, payloads)]
        self.client.insert(collection_name=self.collection_name, data=data)

    def _parse(self, hits: list) -> list[OutputData]:
        out = []
        for h in hits:
            ent = h.get("entity", h)
            out.append(
                OutputData(
                    id=h.get("id", ent.get("id")),
                    score=h.get("distance"),
                    payload=ent.get("metadata"),
                )
            )
        return out

    def search(
        self, query: str, vectors: list, top_k: int = 5, filters: dict = None
    ) -> list:
        owner = self._owner_of(filters)
        hits = self.client.search(
            collection_name=self.collection_name,
            data=[vectors],
            anns_field="vectors",
            limit=top_k,
            filter=OWNER_FILTER,
            filter_params={"owner": owner},
            output_fields=["metadata"],
            search_params={"metric_type": "COSINE", "params": {"ef": max(64, top_k)}},
        )
        return self._parse(hits[0])

    def keyword_search(self, query, top_k=5, filters=None):
        owner = self._owner_of(filters)
        hits = self.client.search(
            collection_name=self.collection_name,
            data=[unicodedata.normalize("NFC", query or "")],
            anns_field="sparse",
            limit=top_k,
            filter=OWNER_FILTER,
            filter_params={"owner": owner},
            output_fields=["metadata"],
        )
        return self._parse(hits[0])

    def delete(self, vector_id) -> None:
        self.client.delete(
            collection_name=self.collection_name,
            filter=IDS_FILTER,
            filter_params={"ids": [vector_id]},
        )

    def update(self, vector_id=None, vector=None, payload=None) -> None:
        if vector is None or payload is None:
            rows = self.client.query(
                collection_name=self.collection_name,
                filter=IDS_FILTER,
                filter_params={"ids": [vector_id]},
                output_fields=["vectors", "metadata"],
            )
            if not rows:
                raise ValueError("memory not found")
            vector = rows[0]["vectors"] if vector is None else vector
            payload = rows[0]["metadata"] if payload is None else payload
        self.client.upsert(
            collection_name=self.collection_name,
            data=[self._record(vector_id, vector, payload)],
        )

    def get(self, vector_id) -> Optional[OutputData]:
        rows = self.client.query(
            collection_name=self.collection_name,
            filter=IDS_FILTER,
            filter_params={"ids": [vector_id]},
            output_fields=["owner", "metadata"],
        )
        if not rows:
            return None
        payload = dict(rows[0].get("metadata") or {})
        payload["owner"] = rows[0].get("owner")
        return OutputData(id=rows[0].get("id", vector_id), score=None, payload=payload)

    def list(self, filters: dict = None, top_k: int = 100) -> list:
        owner = self._owner_of(filters)
        rows = self.client.query(
            collection_name=self.collection_name,
            filter=OWNER_FILTER,
            filter_params={"owner": owner},
            output_fields=["metadata"],
            limit=min(max(1, top_k), MAX_LIST),
        )
        return [
            [
                OutputData(id=r.get("id"), score=None, payload=r.get("metadata"))
                for r in rows
            ]
        ]

    def list_cols(self):
        return self.client.list_collections()

    def delete_col(self):
        raise PermissionError(
            "dropping the memory collection is an operator action, not a sidecar call"
        )

    def col_info(self):
        return {"collection": self.collection_name}

    def reset(self):
        raise PermissionError("reset would delete every person's memories; refused")

    # --- deletion saga -----------------------------------------------------

    def count_owner(self, owner: str) -> int:
        rows = self.client.query(
            collection_name=self.collection_name,
            filter=OWNER_FILTER,
            filter_params={"owner": check_owner(owner)},
            output_fields=["count(*)"],
            consistency_level="Strong",
        )
        return int(rows[0]["count(*)"]) if rows else 0

    def count_ids(self, ids: list[str]) -> int:
        rows = self.client.query(
            collection_name=self.collection_name,
            filter=IDS_FILTER,
            filter_params={"ids": list(ids)},
            output_fields=["count(*)"],
            consistency_level="Strong",
        )
        return int(rows[0]["count(*)"]) if rows else 0

    def delete_owner(self, owner: str) -> None:
        self.client.delete(
            collection_name=self.collection_name,
            filter=OWNER_FILTER,
            filter_params={"owner": check_owner(owner)},
        )

    def compact(self, timeout_s: float = 120.0) -> float:
        """Flush, then an L0 compaction (moves deletes into the segments),
        then a mix compaction (rewrites them without the deleted rows). A mix
        compaction alone removes nothing (bring-up finding 1). Returns the
        seconds spent. Physical file removal is Milvus GC's, later."""
        start = time.monotonic()
        self.client.flush(self.collection_name)
        for is_l0 in (True, False):
            job = self.client.compact(self.collection_name, is_l0=is_l0)
            while (state := self.client.get_compaction_state(job)) != "Completed":
                if time.monotonic() - start > timeout_s:
                    raise TimeoutError(f"compaction still {state} after {timeout_s}s")
                time.sleep(0.2)
        return time.monotonic() - start
