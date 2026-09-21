#!/usr/bin/env python3
"""Oracle for the Go port of app/media/storage.py (W6, ADR-0029 section 2.4).

services/core/internal/media/storage/storage_oracle_postgres_test.go builds a
list of scenarios, runs them against the Go package in one directory and
through this driver, inside the pinned API image, against the real
PhotoStorage in another directory bind-mounted at the same absolute path, then
compares what both sides answered and left on disk:

    docker run --rm -i -v "$DIR:$DIR" -v "$PWD/scripts":/oracle:ro \\
      --entrypoint python <api image> /oracle/render_photo_storage_oracle.py < spec.json

The process runs as the image's own user with the image's own umask, which is
reported, so the modes compared are the modes production gets.

Input: {"base": dir, "scenarios": [{"name", "umask" (null: the image's own),
"setup": [op], "steps": [op]}]}. Scenario i runs in base/i; "{base}" in any
path, target, root or environment value stands for that directory.

Setup ops (not traced): mkdir path mode, file path hex mode, chmod path mode,
symlink path target. Steps: storage (root, or the default media_root when
root is null), media_root, path, write (key, hex), read, delete; storage and
media_root may carry "env" (a null value unsets the variable) and "cwd".

After every step: the result or the exception (class, errno, filename,
filename2, or the message of a ValueError or RuntimeError); the os calls made
through the os module during write, read and delete (mkdir, stat, open,
fsync, replace, unlink, in order); and the scenario's file tree (path, kind,
size of a file, permission bits, link target). Paths under the scenario
directory are written "{base}/...", the running user's home "{home}/...", and
the eight random characters of a temporary name "<random>".
"""

from __future__ import annotations

import hashlib
import json
import os
import pwd
import re
import stat
import sys

sys.path.insert(0, "/srv")

from app.media.storage import PhotoStorage, media_root, new_storage_key  # noqa: E402

_TEMPORARY = re.compile(r"(\.[0-9a-f]{32}\.)[a-z0-9_]{8}(\.tmp)(/|$)")

TRACE: list | None = None
SCENARIO_DIR = ""
try:
    HOME = pwd.getpwuid(os.getuid()).pw_dir
except KeyError:
    HOME = None


def rel(path):
    if path is None:
        return None
    if isinstance(path, int):
        return path
    text = os.fsdecode(path)
    if text == SCENARIO_DIR:
        text = "{base}"
    elif text.startswith(SCENARIO_DIR + "/"):
        text = "{base}" + text[len(SCENARIO_DIR) :]
    elif HOME and (text == HOME or text.startswith(HOME + "/")):
        text = "{home}" + text[len(HOME) :]
    return _TEMPORARY.sub(r"\1<random>\2\3", text)


def _traced(name, describe):
    real = getattr(os, name)

    def wrapper(*args, **kwargs):
        if TRACE is not None:
            TRACE.append(describe(*args, **kwargs))
        return real(*args, **kwargs)

    setattr(os, name, wrapper)


_traced("mkdir", lambda path, mode=0o777, **_: ["mkdir", rel(path), oct(mode)])
_traced("stat", lambda path, **_: ["stat", rel(path)])
_traced(
    "open", lambda path, flags, mode=0o777, **_: ["open", rel(path), flags, oct(mode)]
)
_traced("fsync", lambda fd: ["fsync", rel(os.readlink(f"/proc/self/fd/{fd}"))])
_traced("replace", lambda src, dst, **_: ["replace", rel(src), rel(dst)])
_traced("unlink", lambda path, **_: ["unlink", rel(path)])


def error(exc: BaseException) -> dict:
    out = {
        "type": type(exc).__name__,
        "errno": None,
        "filename": None,
        "filename2": None,
        "message": None,
    }
    if isinstance(exc, OSError):
        out["errno"] = exc.errno
        out["filename"] = rel(exc.filename)
        out["filename2"] = rel(exc.filename2)
    else:
        out["message"] = str(exc).replace(SCENARIO_DIR, "{base}")
    return out


def tree() -> list:
    rows = []
    for dirpath, dirnames, filenames in os.walk(SCENARIO_DIR):
        for name in dirnames + filenames:
            full = os.path.join(dirpath, name)
            st = os.lstat(full)
            if stat.S_ISDIR(st.st_mode):
                kind = "d"
            elif stat.S_ISLNK(st.st_mode):
                kind = "l"
            elif stat.S_ISREG(st.st_mode):
                kind = "f"
            else:
                kind = "?"
            rows.append(
                [
                    rel(full),
                    kind,
                    st.st_size if kind == "f" else None,
                    oct(stat.S_IMODE(st.st_mode)),
                    rel(os.readlink(full)) if kind == "l" else None,
                ]
            )
    rows.sort(key=lambda row: row[0])
    return rows


def sub(text):
    return text.replace("{base}", SCENARIO_DIR) if isinstance(text, str) else text


def setup(op: dict) -> None:
    path = os.path.join(SCENARIO_DIR, op["path"])
    if op["op"] == "mkdir":
        os.mkdir(path)
        os.chmod(path, op["mode"])
    elif op["op"] == "file":
        with open(path, "wb") as handle:
            handle.write(bytes.fromhex(op["hex"]))
        os.chmod(path, op["mode"])
    elif op["op"] == "chmod":
        os.chmod(path, op["mode"])
    elif op["op"] == "symlink":
        os.symlink(sub(op["target"]), path)
    else:
        raise ValueError(op["op"])


def run_step(step: dict, storage):
    global TRACE
    record = {"result": None, "error": None, "trace": [], "tree": []}
    saved_env = {name: os.environ.get(name) for name in (step.get("env") or {})}
    saved_cwd = os.getcwd()
    for name, value in (step.get("env") or {}).items():
        if value is None:
            os.environ.pop(name, None)
        else:
            os.environ[name] = sub(value)
    if step.get("cwd"):
        os.chdir(os.path.join(SCENARIO_DIR, step["cwd"]))
    trace: list = []
    op = step["op"]
    try:
        if op in ("write", "read", "delete"):
            TRACE = trace
        if op == "storage":
            root = step.get("root")
            storage = PhotoStorage() if root is None else PhotoStorage(sub(root))
            record["result"] = rel(str(storage.root))
        elif op == "media_root":
            record["result"] = rel(str(media_root()))
        elif op == "path":
            record["result"] = rel(str(storage._path_for(step["key"])))
        elif op == "write":
            storage.write(step["key"], bytes.fromhex(step["hex"]))
        elif op == "read":
            content = storage.read(step["key"])
            record["result"] = {
                "size": len(content),
                "sha256": hashlib.sha256(content).hexdigest(),
            }
        elif op == "delete":
            record["result"] = storage.delete(step["key"])
        else:
            raise SystemExit(f"unknown step {op}")
    except Exception as exc:  # every refusal is part of the answer
        record["error"] = error(exc)
    finally:
        TRACE = None
        os.chdir(saved_cwd)
        for name, value in saved_env.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value
    record["trace"] = trace
    record["tree"] = tree()
    return record, storage


def main() -> None:
    global SCENARIO_DIR
    spec = json.load(sys.stdin)
    default_umask = os.umask(0)
    os.umask(default_umask)
    out = {
        "default_umask": default_umask,
        "keys": [new_storage_key() for _ in range(256)],
        "scenarios": [],
    }
    for index, scenario in enumerate(spec["scenarios"]):
        SCENARIO_DIR = os.path.join(spec["base"], str(index))
        os.mkdir(SCENARIO_DIR)
        os.chmod(SCENARIO_DIR, 0o777)
        for op in scenario["setup"]:
            setup(op)
        umask = scenario["umask"]
        previous = os.umask(default_umask if umask is None else umask)
        storage = None
        steps = []
        try:
            for step in scenario["steps"]:
                record, storage = run_step(step, storage)
                steps.append(record)
        finally:
            os.umask(previous)
        out["scenarios"].append({"name": scenario["name"], "steps": steps})
    json.dump(out, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
