"""The memory operations the endpoints expose, over one mem0 Memory.

Every operation is scoped to one owner id given by the caller (Go, which
decided consent). Operations by memory id check the owner first: mem0's own
get/delete take no owner at all (mem0.md §6). A delete is reported only after
a Strong-consistency count of what should be gone reads zero; mem0's own
delete_all can stop early and still say "deleted successfully" (mem0.md
§Kiểm chứng 10), so its message is never trusted.
"""

from __future__ import annotations

import threading
from collections import defaultdict
from dataclasses import dataclass

from ai_infer.mem import extract
from ai_infer.mem.store import NepMilvus, check_owner

MAX_SEARCH = 10
MAX_LIST = 1000
MAX_ROUNDS = 5


class NotFound(LookupError):
    """No memory with that id belongs to that owner (same answer either way)."""


class DeleteIncomplete(RuntimeError):
    def __init__(self, remaining: int):
        super().__init__(f"{remaining} rows remain after delete")
        self.remaining = remaining


class OwnerLeak(RuntimeError):
    """The store answered a row of another owner: fail closed."""


@dataclass
class Added:
    id: str
    text: str
    loai: str | None


class MemoryService:
    def __init__(self, memory):
        self.m = memory
        self.store: NepMilvus = memory.vector_store
        self._locks: dict[str, threading.Lock] = defaultdict(threading.Lock)
        self._locks_guard = threading.Lock()

    def _lock(self, owner: str) -> threading.Lock:
        with self._locks_guard:
            return self._locks[owner]

    def _own(self, owner: str, items: list[dict]) -> list[dict]:
        for it in items:
            if it.get("user_id") != owner:
                raise OwnerLeak("a memory of another owner was returned")
        return items

    def add(self, owner: str, texts: list[str]) -> tuple[list[Added], int]:
        """Extract memories from the person's own messages. Returns what was
        stored and how many candidates the model itself refused."""
        check_owner(owner)
        messages = [{"role": "user", "content": t} for t in texts]
        with self._lock(owner):
            extract.begin()
            res = self.m.add(messages, user_id=owner, infer=True)
            out = [
                Added(id=r["id"], text=r["memory"], loai=extract.kept_kind(r["memory"]))
                for r in res.get("results", [])
                if r.get("event") == "ADD"
            ]
            return out, extract.refused()

    def search(
        self, owner: str, query: str, top_k: int, threshold: float
    ) -> list[dict]:
        check_owner(owner)
        res = self.m.search(
            query,
            top_k=min(top_k, MAX_SEARCH),
            filters={"user_id": owner},
            threshold=threshold,
        )
        return self._own(owner, res.get("results", []))

    def list(self, owner: str, limit: int) -> list[dict]:
        check_owner(owner)
        res = self.m.get_all(filters={"user_id": owner}, top_k=min(limit, MAX_LIST))
        return self._own(owner, res.get("results", []))

    def delete(self, owner: str, memory_id: str) -> int:
        check_owner(owner)
        with self._lock(owner):
            found = self.store.get(memory_id)
            if found is None or (found.payload or {}).get("owner") != owner:
                raise NotFound(memory_id)
            self.m.delete(memory_id)
            remaining = self.store.count_ids([memory_id])
            if remaining:
                raise DeleteIncomplete(remaining)
            return 1

    def delete_all(self, owner: str) -> tuple[int, int]:
        """mem0's delete_all, each call followed by a Strong count, repeated
        until the count reads zero: mem0 deletes in batches and can stop
        early while still answering "deleted successfully". A round that
        makes no progress falls back to a delete by the owner filter. After
        MAX_ROUNDS the delete is reported incomplete, never done. Returns
        (deleted, remaining=0)."""
        check_owner(owner)
        with self._lock(owner):
            before = remaining = self.store.count_owner(owner)
            rounds = 0
            while remaining and rounds < MAX_ROUNDS:
                rounds += 1
                self.m.delete_all(user_id=owner)
                now = self.store.count_owner(owner)
                if now >= remaining:
                    self.store.delete_owner(owner)
                    now = self.store.count_owner(owner)
                remaining = now
            if remaining:
                raise DeleteIncomplete(remaining)
            return before, 0

    def purge_user(self, owner: str) -> tuple[int, int, float]:
        """Account deletion: every row of the owner deleted by the owner
        filter, flushed and compacted (L0 then mix), then counted. Returns
        (deleted, remaining=0, compaction seconds)."""
        check_owner(owner)
        with self._lock(owner):
            before = self.store.count_owner(owner)
            self.store.delete_owner(owner)
            seconds = self.store.compact()
            remaining = self.store.count_owner(owner)
            if remaining:
                raise DeleteIncomplete(remaining)
            return before, 0, seconds
