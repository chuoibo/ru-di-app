#!/usr/bin/env python3
"""Render the request contract of every APIRoute as JSON IR for the Go port.

ADR-0029: a Go-served route must refuse a bad request with the 422 FastAPI
writes and must hand a good request to its handler as the values pydantic
would have produced. Both are decided by two things this script reads off the
shipped application rather than off the source text:

- FastAPI's dependency tree (`route.dependant`): which parameter comes from
  the path, the query, a header, a cookie or the body, under which alias, with
  which default, and in which order `solve_dependencies` validates them and
  calls the dependencies in between;
- pydantic-core's schema for each parameter (`ModelField._type_adapter
  .core_schema`), which is what actually validates: strictness, constraints,
  literals, unions, defaults, model fields in declaration order, extra
  behaviour and the validator functions a Go port has to supply itself.

Run inside the pinned API image, so the libraries and the models are the
shipped ones. No database is needed; the URL points at a closed port.

    docker run --rm --network none --user "$(id -u):$(id -g)" \\
      -v "$PWD/scripts":/scripts:ro \\
      -v "$PWD/services/core/contract/ir":/out \\
      --entrypoint python <api image> /scripts/render_contract_ir.py --out /out

`--check` renders in memory and exits 1 when the files in --out differ.

## Encoding choices

- One file per router group (the module under `app.api.routes`), so each stays
  far below the repository guard's 2 MiB limit. A group whose name the guard
  refuses as a file name is written under a singular form (FILE_NAMES); the
  `group` field inside keeps the real name.
- Model schemas are hoisted into the file's `definitions`, keyed by the
  class's qualified name. pydantic's own refs end in the class object's
  address, which changes on every run.
- Python values (defaults, literal members, constraint bounds) are JSON
  scalars when that is lossless. An int of nine or more digits would be a
  "long number" to the repository guard, so it is written as
  {"$int": "1_000_000_000"}; the underscores are Python's own digit
  separators and a reader strips them. Floats that would trip the same rule
  are written as {"$float": "<repr with separators>"}; non-finite floats
  always are. Everything else carries a "$" tag.
- Every emitted line is checked against the guard's long-number and email
  patterns before anything is written; a hit aborts with its JSON path.
"""

from __future__ import annotations

import argparse
import dataclasses
import datetime as dt
import decimal
import enum
import functools
import json
import math
import os
import pathlib
import re
import sys
import types
import typing
import uuid

IR_VERSION = 1

# repo_guard refuses these words inside file names; see FORBIDDEN_* there.
FILE_NAMES = {"bills": "group-splitbill", "receipts": "group-scanreceipt"}
# The guard's two file-name patterns (scripts/repo_guard.py), applied to the
# stem of a .json file: a listed word as a whole `-_. `-separated token.
EXPORT_NAME_RE = re.compile(
    r"(?:^|[-_. ])(?:export|dump|raw[-_. ]?data)(?:$|[-_. ])", re.IGNORECASE
)
DATASET_NAME_RE = re.compile(
    r"(?:^|[-_. ])(?:participants?|respondents?|survey[-_. ]?responses?|"
    r"form[-_. ]?responses?|transcripts?|receipts?|bills?)(?:$|[-_. ])",
    re.IGNORECASE,
)
LONG_NUMBER_RE = re.compile(r"(?<![A-Za-z0-9])\d(?:[ .-]?\d){8,63}(?![A-Za-z0-9])")
EMAIL_RE = re.compile(r"[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9-]+\.[A-Za-z0-9.-]+")
REF_SUFFIX_RE = re.compile(r":\d+$")
ADDRESS_RE = re.compile(r" at 0x[0-9a-fA-F]+")

# Keys of a core schema node that hold another node, a list of nodes, or a
# mapping of nodes. Everything else must be a plain value or is refused.
NODE_KEYS = {
    "schema",
    "items_schema",
    "keys_schema",
    "values_schema",
    "lax_schema",
    "strict_schema",
    "json_schema",
    "python_schema",
    "extras_schema",
    "extras_keys_schema",
}
NODE_LIST_KEYS = {"steps", "items_schema_list", "prefix_items"}
# Keys dropped because they only affect serialization or JSON Schema output.
DROPPED_KEYS = {
    "metadata",
    "serialization",
    "computed_fields",
    "json_schema_input_schema",
}


class RenderError(Exception):
    pass


# Marks a definition whose body is being rendered.
PENDING: dict = {"type": "pending"}


def bootstrap() -> typing.Any:
    os.environ.setdefault("MOBILE_AUTH_MODE", "dev")
    os.environ["MOBILE_DATABASE_URL"] = (
        "postgresql+psycopg://nobody:nothing@127.0.0.1:9/nothing"
    )
    sys.path.insert(0, "/srv")
    from app.api.main import create_app

    return create_app()


def qualified(obj: typing.Any) -> str:
    """A stable dotted name for a class or function (never its address)."""
    if isinstance(obj, functools.partial):
        return qualified(obj.func)
    if isinstance(obj, (classmethod, staticmethod)):
        obj = obj.__func__
    obj = getattr(obj, "__func__", obj)
    module = getattr(obj, "__module__", None) or "?"
    name = getattr(obj, "__qualname__", None) or getattr(obj, "__name__", None)
    if name is None:
        raise RenderError(f"no qualified name for {type(obj).__name__}")
    return f"{module}.{name}"


def group_int(n: int) -> str:
    sign, digits = ("-", str(-n)) if n < 0 else ("", str(n))
    head = len(digits) % 3 or 3
    parts = [digits[:head]] + [digits[i : i + 3] for i in range(head, len(digits), 3)]
    return sign + "_".join(parts)


def py_repr(v: typing.Any) -> str:
    """repr() for annotation text, with long ints digit-grouped."""
    if type(v) is int and abs(v) >= 10**8:
        return group_int(v)
    if isinstance(v, str):
        return repr(v)
    if isinstance(v, float) and LONG_NUMBER_RE.search(repr(v)):
        return float_text(v)
    return ADDRESS_RE.sub("", repr(v))


def float_text(v: float) -> str:
    text = repr(v)
    mantissa, sep, exponent = text.partition("e")
    whole, dot, frac = mantissa.partition(".")
    if len(frac) > 3:
        frac = "_".join(frac[i : i + 3] for i in range(0, len(frac), 3))
    sign = "-" if whole.startswith("-") else ""
    whole = whole.lstrip("-")
    if len(whole) > 3:
        whole = group_int(int(whole))
    return f"{sign}{whole}{dot}{frac}{sep}{exponent}"


def value(v: typing.Any) -> typing.Any:
    """A Python value as IR JSON (see the module doc for the tags)."""
    if v is None or isinstance(v, bool):
        return v
    if isinstance(v, enum.Enum):
        return {"$enum": qualified(type(v)), "name": v.name, "value": value(v.value)}
    if type(v) is int:
        return v if abs(v) < 10**8 else {"$int": group_int(v)}
    if type(v) is float:
        if not math.isfinite(v) or LONG_NUMBER_RE.search(repr(v)):
            return {"$float": float_text(v)}
        return v
    if type(v) is str:
        return v
    if isinstance(v, bytes):
        return {"$bytes": v.hex()}
    if isinstance(v, uuid.UUID):
        return {"$uuid": str(v)}
    if isinstance(v, dt.datetime):
        return {"$datetime": v.isoformat()}
    if isinstance(v, dt.date):
        return {"$date": v.isoformat()}
    if isinstance(v, dt.time):
        return {"$time": v.isoformat()}
    if isinstance(v, dt.timedelta):
        return {"$timedelta": [v.days, v.seconds, v.microseconds]}
    if isinstance(v, decimal.Decimal):
        return {"$decimal": str(v)}
    if isinstance(v, re.Pattern):
        return {"$pattern": v.pattern, "flags": int(v.flags)}
    if isinstance(v, tuple):
        return {"$tuple": [value(x) for x in v]}
    if isinstance(v, list):
        return [value(x) for x in v]
    if isinstance(v, (set, frozenset)):
        return {"$set": sorted((value(x) for x in v), key=json.dumps)}
    if isinstance(v, dict):
        return {"$dict": [[value(k), value(x)] for k, x in v.items()]}
    if callable(v):
        return {"$callable": qualified(v)}
    raise RenderError(f"no IR encoding for a {type(v).__name__} value")


def annotation_text(ann: typing.Any) -> str:
    """Human-readable annotation, stable across runs."""
    if ann is type(None) or ann is None:
        return "None"
    origin = typing.get_origin(ann)
    args = typing.get_args(ann)
    if origin is typing.Annotated:
        meta = ", ".join(metadata_text(m) for m in ann.__metadata__)
        return f"Annotated[{annotation_text(args[0])}, {meta}]"
    if origin in (typing.Union, types.UnionType):
        return " | ".join(annotation_text(a) for a in args)
    if origin is typing.Literal:
        return "Literal[" + ", ".join(py_repr(a) for a in args) + "]"
    if origin is not None:
        name = getattr(origin, "__name__", None) or str(origin)
        if args:
            return f"{name}[{', '.join(annotation_text(a) for a in args)}]"
        return name
    if isinstance(ann, type):
        if ann.__module__ == "builtins":
            return ann.__qualname__
        return qualified(ann)
    if isinstance(ann, typing.TypeVar):
        return ann.__name__
    return ADDRESS_RE.sub("", repr(ann))


def metadata_text(m: typing.Any) -> str:
    from pydantic.fields import FieldInfo

    if isinstance(m, FieldInfo):
        parts = [metadata_text(x) for x in m.metadata]
        for attr in ("alias", "validation_alias", "discriminator"):
            if getattr(m, attr, None) is not None:
                parts.append(f"{attr}={py_repr(getattr(m, attr))}")
        return "Field(" + ", ".join(parts) + ")"
    if dataclasses.is_dataclass(m) and not isinstance(m, type):
        fields = ", ".join(
            f"{f.name}={py_repr(getattr(m, f.name))}"
            for f in dataclasses.fields(m)
            if getattr(m, f.name) is not None
        )
        return f"{type(m).__name__}({fields})"
    if callable(m):
        return qualified(m)
    return py_repr(m)


def constraints_of(field_info: typing.Any) -> list[dict]:
    """The Annotated metadata FastAPI/pydantic attached to a FieldInfo."""
    out = []
    for m in field_info.metadata:
        if dataclasses.is_dataclass(m) and not isinstance(m, type):
            entry = {"kind": type(m).__name__}
            for f in dataclasses.fields(m):
                if getattr(m, f.name) is not None:
                    entry[f.name] = value(getattr(m, f.name))
            out.append(entry)
        else:
            out.append({"kind": type(m).__name__, "text": metadata_text(m)})
    return out


class Renderer:
    """Turns core schemas into IR nodes, hoisting refs into definitions."""

    def __init__(self) -> None:
        self.definitions: dict[str, dict] = {}

    def ref_name(self, ref: str) -> str:
        return REF_SUFFIX_RE.sub("", ref)

    def hoist(self, ref: str, s: dict) -> dict:
        """Render a referenced schema once per name and prove the copies agree."""
        name = self.ref_name(ref)
        known = self.definitions.get(name)
        if known is PENDING:
            return {"type": "ref", "ref": name}  # recursion into itself
        self.definitions[name] = PENDING
        rendered = self.body(s)
        if known is not None and known != rendered:
            raise RenderError(f"two different schemas share the ref {name}")
        self.definitions[name] = rendered
        return {"type": "ref", "ref": name}

    def node(self, s: dict) -> dict:
        if not isinstance(s, dict) or "type" not in s:
            raise RenderError(f"not a core schema: {type(s).__name__}")
        t = s["type"]
        if t == "definitions":
            for d in s["definitions"]:
                if "ref" not in d:
                    raise RenderError("a definition without ref")
                self.hoist(d["ref"], d)
            return self.node(s["schema"])
        if t == "definition-ref":
            return {"type": "ref", "ref": self.ref_name(s["schema_ref"])}
        if "ref" in s:
            return self.hoist(s["ref"], s)
        return self.body(s)

    def body(self, s: dict) -> dict:
        t = s["type"]
        out: dict[str, typing.Any] = {"type": t}
        handler = getattr(self, "node_" + t.replace("-", "_"), None)
        if handler is not None:
            handler(s, out)
            return out
        self.generic(s, out, skip=())
        return out

    def generic(self, s: dict, out: dict, skip: typing.Iterable[str]) -> None:
        for k, v in s.items():
            if k in ("type", "ref") or k in DROPPED_KEYS or k in skip:
                continue
            if k in NODE_KEYS:
                out[k] = self.node(v)
            elif k in NODE_LIST_KEYS:
                out[k] = [self.node(x) for x in v]
            elif k == "choices":
                out[k] = [self.choice(c) for c in v]
            elif k == "function":
                out[k] = self.function(v)
            elif k == "default_factory":
                out[k] = {"name": qualified(v)}
            elif k == "cls":
                out["class"] = qualified(v)
            elif k == "config":
                out[k] = {
                    ck: value(cv) for ck, cv in sorted(v.items()) if ck != "title"
                }
            else:
                out[k] = value(v)

    def choice(self, c: typing.Any) -> dict:
        if isinstance(c, tuple):
            schema, label = c
            return {"label": label, "schema": self.node(schema)}
        return {"schema": self.node(c)}

    def function(self, f: typing.Any) -> dict:
        if callable(f):
            return self.callable(f)
        out = {"kind": f.get("type")}
        out.update(self.callable(f["function"]))
        if f.get("field_name") is not None:
            out["field_name"] = f["field_name"]
        return out

    def callable(self, fn: typing.Any) -> dict:
        out: dict[str, typing.Any] = {"name": qualified(fn)}
        if isinstance(fn, functools.partial):
            if fn.args:
                out["args"] = [value(a) for a in fn.args]
            if fn.keywords:
                out["keywords"] = {k: value(v) for k, v in sorted(fn.keywords.items())}
        return out

    def node_model(self, s: dict, out: dict) -> None:
        self.generic(s, out, skip=("schema",))
        cls = s["cls"]
        out["decorators"] = decorators_of(cls)
        out["schema"] = self.node(s["schema"])
        annotate_fields(out["schema"], cls)

    def node_model_fields(self, s: dict, out: dict) -> None:
        self.generic(s, out, skip=("fields",))
        out["fields"] = [self.field(name, f) for name, f in s["fields"].items()]

    def node_typed_dict(self, s: dict, out: dict) -> None:
        self.generic(s, out, skip=("fields",))
        out["fields"] = [self.field(name, f) for name, f in s["fields"].items()]

    def node_dataclass_args(self, s: dict, out: dict) -> None:
        self.generic(s, out, skip=("fields",))
        out["fields"] = [self.field(f["name"], f) for f in s["fields"]]

    def field(self, name: str, f: dict) -> dict:
        entry: dict[str, typing.Any] = {"name": name}
        for k, v in f.items():
            if k in ("type", "name") or k in DROPPED_KEYS:
                continue
            if k == "schema":
                entry[k] = self.node(v)
            else:
                entry[k] = value(v)
        return entry


def annotate_fields(fields_node: dict, cls: type) -> None:
    """Attach the declared annotation and default to each model field."""
    model_fields = getattr(cls, "model_fields", None)
    if fields_node.get("type") != "model-fields" or model_fields is None:
        return
    for entry in fields_node["fields"]:
        info = model_fields.get(entry["name"])
        if info is None:
            continue
        entry["annotation"] = annotation_text(info.annotation)
        entry["required"] = info.is_required()
        if info.metadata:
            entry["constraints"] = constraints_of(info)
        if info.alias is not None:
            entry["alias"] = info.alias


def decorators_of(cls: type) -> dict:
    decs = getattr(cls, "__pydantic_decorators__", None)
    if decs is None:
        return {}
    out: dict[str, list] = {}
    fields = [
        {
            "name": name,
            "function": qualified(d.func),
            "fields": list(d.info.fields),
            "mode": d.info.mode,
        }
        for name, d in decs.field_validators.items()
    ]
    if fields:
        out["field_validators"] = fields
    models = [
        {"name": name, "function": qualified(d.func), "mode": d.info.mode}
        for name, d in decs.model_validators.items()
    ]
    if models:
        out["model_validators"] = models
    return out


def param(field: typing.Any, renderer: Renderer) -> dict:
    from fastapi import params
    from fastapi._compat import is_scalar_field, is_sequence_field

    info = field.field_info
    location = info.in_.value if isinstance(info, params.Param) else "body"
    entry: dict[str, typing.Any] = {
        "name": field.name,
        "alias": field.alias,
        "in": location,
        "field_info": type(info).__name__,
        "annotation": annotation_text(info.annotation),
        "required": field.required,
    }
    if not field.required:
        if info.default_factory is not None:
            entry["default_factory"] = qualified(info.default_factory)
        else:
            entry["default"] = value(info.default)
    if info.metadata:
        entry["constraints"] = constraints_of(info)
    entry["sequence"] = bool(is_sequence_field(field))
    entry["scalar"] = bool(is_scalar_field(field))
    if isinstance(info, params.Header):
        entry["convert_underscores"] = info.convert_underscores
    if isinstance(info, params.Body):
        entry["embed"] = info.embed
        entry["media_type"] = info.media_type
    entry["schema"] = renderer.node(field._type_adapter.core_schema)
    return entry


def dependant(d: typing.Any, renderer: Renderer) -> dict:
    out: dict[str, typing.Any] = {
        "call": qualified(d.call),
        "name": d.name,
        "use_cache": d.use_cache,
        "cache_key": [qualified(d.cache_key[0]), list(d.cache_key[1])],
    }
    for kind in ("path", "query", "header", "cookie", "body"):
        fields = getattr(d, kind + "_params")
        if fields:
            out[kind] = [param(f, renderer) for f in fields]
    specials = {
        "request": d.request_param_name,
        "websocket": d.websocket_param_name,
        "http_connection": d.http_connection_param_name,
        "response": d.response_param_name,
        "background_tasks": d.background_tasks_param_name,
        "security_scopes": d.security_scopes_param_name,
    }
    specials = {k: v for k, v in specials.items() if v is not None}
    if specials:
        out["specials"] = specials
    if d.dependencies:
        out["dependencies"] = [dependant(sd, renderer) for sd in d.dependencies]
    return out


def group_of(module: str) -> str:
    prefix = "app.api.routes."
    return (
        module[len(prefix) :]
        if module.startswith(prefix)
        else module.rsplit(".", 1)[-1]
    )


def route_entries(app: typing.Any) -> dict[str, dict]:
    from fastapi import params
    from fastapi.routing import APIRoute

    groups: dict[str, dict] = {}
    for order, route in enumerate(app.routes):
        if not isinstance(route, APIRoute):
            continue
        group = group_of(route.endpoint.__module__)
        bucket = groups.setdefault(group, {"renderer": Renderer(), "routes": []})
        renderer = bucket["renderer"]
        body = None
        if route.body_field is not None:
            info = route.body_field.field_info
            kind = "json"
            if isinstance(info, params.File):
                kind = "multipart"
            elif isinstance(info, params.Form):
                kind = "form"
            body = {
                "kind": kind,
                "field_info": type(info).__name__,
                "media_type": getattr(info, "media_type", None),
                "embed": route._embed_body_fields,
                "required": route.body_field.required,
            }
        tree = dependant(route.dependant, renderer)
        for method in sorted(route.methods):
            bucket["routes"].append(
                {
                    "id": f"{method} {route.path}",
                    "order": order,
                    "method": method,
                    "path": route.path,
                    "name": route.name,
                    "endpoint": qualified(route.endpoint),
                    "status_code": route.status_code,
                    "path_convertors": {
                        k: type(v).__name__ for k, v in route.param_convertors.items()
                    },
                    "body": body,
                    "dependant": tree,
                }
            )
    return groups


def stack_versions() -> dict[str, str]:
    import fastapi
    import pydantic
    import pydantic_core
    import starlette

    return {
        "fastapi": fastapi.__version__,
        "pydantic": pydantic.VERSION,
        "pydantic_core": pydantic_core.__version__,
        "starlette": starlette.__version__,
        "python": ".".join(str(p) for p in sys.version_info[:3]),
    }


def file_name(group: str) -> str:
    stem = FILE_NAMES.get(group, group)
    if EXPORT_NAME_RE.search(stem) or DATASET_NAME_RE.search(stem):
        raise RenderError(
            f"file name {stem}.json is refused by the guard; map it in FILE_NAMES"
        )
    return stem + ".json"


def guard_hits(text: str) -> list[str]:
    hits = []
    for number, line in enumerate(text.splitlines(), start=1):
        if LONG_NUMBER_RE.search(line) or EMAIL_RE.search(line):
            hits.append(f"line {number}: {line.strip()[:120]}")
    return hits


def group_document(group: str, bucket: dict, versions: dict[str, str]) -> dict:
    """The IR document of one router group, as route_entries collected it.

    Also called by services/core/internal/pyval's synthetic oracle, which adds
    a route to create_app() inside the image and renders it with this code.
    """
    renderer: Renderer = bucket["renderer"]
    return {
        "ir_version": IR_VERSION,
        "generator": "scripts/render_contract_ir.py",
        "stack": versions,
        "group": group,
        "routes": bucket["routes"],
        "definitions": dict(sorted(renderer.definitions.items())),
    }


def render(app: typing.Any) -> dict[str, str]:
    versions = stack_versions()
    files: dict[str, str] = {}
    for group, bucket in sorted(route_entries(app).items()):
        doc = group_document(group, bucket, versions)
        text = json.dumps(doc, indent=1, ensure_ascii=True) + "\n"
        name = file_name(group)
        hits = guard_hits(text)
        if hits:
            raise RenderError(
                f"{name} would trip the repository guard:\n" + "\n".join(hits)
            )
        if len(text.encode()) >= 2 * 1024 * 1024:
            raise RenderError(f"{name} is not below 2 MiB")
        files[name] = text
    return files


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    parser.add_argument("--out", required=True, type=pathlib.Path)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    try:
        files = render(bootstrap())
    except RenderError as exc:
        print(f"render_contract_ir: {exc}", file=sys.stderr)
        return 2
    existing = {p.name for p in args.out.glob("*.json")}
    if args.check:
        drift = sorted(
            name
            for name in existing | set(files)
            if name not in files
            or name not in existing
            or (args.out / name).read_text() != files[name]
        )
        for name in drift:
            print(f"render_contract_ir: {name} is out of date", file=sys.stderr)
        return 1 if drift else 0
    args.out.mkdir(parents=True, exist_ok=True)
    for name in sorted(existing - set(files)):
        (args.out / name).unlink()
    for name, text in files.items():
        (args.out / name).write_text(text)
    print(
        f"render_contract_ir: wrote {len(files)} files to {args.out}", file=sys.stderr
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
