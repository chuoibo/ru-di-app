#!/usr/bin/env python3
"""Render the generated 422 parity corpus for one wave of routes.

ADR-0029 §2.3 asks for a 422 corpus before a route is CARDED, and §2.4 makes
the Python answer the reference for every byte. This script writes that corpus
as parity scenarios: requests only, never an expected status, header or body,
so the files stay inside ADR-0010 §6.1 and pass `parity lint`.

The cases are read off the shipped application, not off the source text:

- `create_app()` gives the APIRoute for each route of the wave, and FastAPI's
  dependency tree gives its path, query, header, cookie and body parameters;
- the body model is read twice: from pydantic-core's schema, which is what
  validates (kinds, strictness, length and number bounds, Literal members,
  nullability, defaults, `extra`), and from `model_fields` annotations. The
  render aborts when the two readings disagree, or when either holds a
  construct this script does not know how to probe, rather than emit a thin
  corpus that looks complete;
- "a wrong JSON type pydantic would refuse" is decided by FastAPI's own body
  field (`ModelField.validate`, the call `request_body_to_args` makes), so a
  probe is only emitted where the reference really refuses it.

Two things are named by hand because nothing in the IR states them. The domain
vocabularies (area ids, interest tags, budget bands) in `vocabulary()`: a valid
body built from placeholders would be refused by the handler before it did
anything worth comparing. And the meaning of the `X-Actor-ID` header, which
the dependency tree declares as `str | None` while `get_actor` parses it as a
UUID in its body.

Run inside the pinned API image; no database and no network are needed:

    docker run --rm --network none --user "$(id -u):$(id -g)" \\
      -v "$PWD":/repo --entrypoint python mobile-parity-api:7bf58e3d \\
      /repo/scripts/render_parity_422_scenarios.py \\
      --wave w1 --out /repo/parity/scenarios/generated/w1-422

`--check` renders in memory and exits 1 naming every file that differs.

A wave may defer a route the generator cannot probe yet, naming a fragment of
the refusal it raises; the render fails when a deferred route renders or is
refused for another reason, so a deferral cannot outlive its cause. A route
can also be excluded with a reason, for what generated steps must not touch.

Every rendered file is parsed back with PyYAML and compared with the steps it
was meant to hold, and scanned with `scripts/repo_guard.py`'s own content
rules, before anything is written.
"""

from __future__ import annotations

import argparse
import copy
import dataclasses
import importlib.util
import json
import os
import pathlib
import re
import sys
import types
import typing
import urllib.parse
import uuid


@dataclasses.dataclass(frozen=True)
class Wave:
    routes: tuple[tuple[str, str], ...]
    #: (method, path, fragment of the RenderError it must still raise)
    deferred: tuple[tuple[str, str, str], ...] = ()
    #: (method, path, why generated steps must not exercise it)
    excluded: tuple[tuple[str, str, str], ...] = ()


WAVES: dict[str, Wave] = {
    "w1": Wave(
        routes=(
            ("PUT", "/people/me/interests"),
            ("POST", "/reports"),
            ("POST", "/contexts/{context_id}/meet"),
            ("GET", "/contexts/{context_id}/preference-profile"),
            ("GET", "/contexts/{context_id}/map"),
            ("GET", "/contexts/{context_id}/heatmap"),
            ("GET", "/contexts/{context_id}/recap"),
            ("GET", "/interests"),
            ("GET", "/areas"),
        )
    ),
    "w2": Wave(
        routes=(
            ("POST", "/friends/requests"),
            ("POST", "/friends/requests/{request_id}/respond"),
            ("GET", "/people/{person_id}/friends"),
            ("GET", "/stories"),
            ("POST", "/stories/{story_id}/seen"),
            ("DELETE", "/stories/{story_id}"),
            ("GET", "/posts/{post_id}"),
            ("POST", "/posts/{post_id}/reactions"),
            ("DELETE", "/posts/{post_id}/comments/{comment_id}"),
            ("GET", "/contexts/{context_id}/votes"),
            ("GET", "/votes/{vote_id}"),
            ("POST", "/votes/{vote_id}/ballots"),
            ("POST", "/votes/{vote_id}/close"),
            ("GET", "/people/{person_id}/friend-requests"),
            ("GET", "/posts"),
            ("GET", "/people/{person_id}/posts"),
            ("GET", "/posts/{post_id}/comments"),
            ("POST", "/posts/{post_id}/comments"),
        ),
        deferred=(
            ("POST", "/stories", "carries ['pattern']"),
            ("POST", "/posts", "carries ['pattern']"),
            ("DELETE", "/posts/{post_id}/reactions/{kind}", "path kind is literal"),
            ("POST", "/contexts/{context_id}/votes", "'function-after' is not probed"),
        ),
        excluded=(
            (
                "POST",
                "/friends/lookup",
                "body parsed by hand and an in-memory per-IP limiter; hand scenarios only",
            ),
            (
                "POST",
                "/identity/person-id",
                "body parsed by hand and an in-memory per-IP limiter; hand scenarios only",
            ),
        ),
    ),
    "w3": Wave(
        routes=(
            ("POST", "/contexts"),
            ("POST", "/contexts/{context_id}/members"),
            ("POST", "/memberships/{membership_id}/accept"),
            ("DELETE", "/contexts/{context_id}/members/{person_id}"),
            ("GET", "/contexts/{context_id}/members"),
            ("GET", "/contexts/{context_id}/balances"),
            ("GET", "/contexts/{context_id}"),
            ("POST", "/contexts/{context_id}/checkins"),
            ("GET", "/contexts/{context_id}/widget"),
            ("POST", "/contexts/{context_id}/memories/{memory_id}/reactions"),
            ("DELETE", "/contexts/{context_id}/memories/{memory_id}/reactions"),
            ("POST", "/contexts/{context_id}/memories/{memory_id}/comments"),
            ("GET", "/contexts/{context_id}/memories/{memory_id}/comments"),
        ),
        deferred=(
            ("PATCH", "/contexts/{context_id}", "'function-after' is not probed"),
            ("POST", "/contexts/{context_id}/memories", "carries ['pattern']"),
            (
                "GET",
                "/contexts/{context_id}/memories",
                "query kind: no candidate value is accepted",
            ),
        ),
    ),
    "w4": Wave(
        routes=(
            ("GET", "/bills/{bill_id}"),
            ("PUT", "/bills/{bill_id}/assignments"),
            ("POST", "/bills/{bill_id}/my-items"),
            ("POST", "/bills/{bill_id}/split"),
            ("GET", "/batches/{batch_id}/obligations"),
            ("GET", "/contexts/{context_id}/batches"),
            ("POST", "/obligations/{obligation_id}/confirm-receipt"),
            ("GET", "/people/{person_id}/finance"),
        ),
        deferred=(
            # occurred_at carries a timezone field_validator
            ("POST", "/expenses", "'function-after' is not probed"),
            (
                "POST",
                "/expenses/{expense_id}/confirm",
                "'function-after' is not probed",
            ),
            # surcharges and discounts default through default_factory
            (
                "POST",
                "/bills",
                "carries ['default_factory', 'default_factory_takes_data']",
            ),
            # the candidate query parses digits in a BeforeValidator
            ("GET", "/contexts/{context_id}/budget", "'function-after' is not probed"),
            # due_at and guest_link_expires_at carry a timezone field_validator
            ("POST", "/batches", "'function-after' is not probed"),
            ("POST", "/batches/{batch_id}/publish", "'function-after' is not probed"),
        ),
    ),
    # The guest boundary renders nothing yet. Every route is refused at its
    # path parameter first: the token is a str with length bounds and a
    # pattern, not a UUID. Behind that, three routes read form bodies, which
    # read_route refuses as well ("form bodies are not probed"). Their 422s are
    # hand-written in parity/scenarios/w5/guests/validation-422.yaml.
    "w5": Wave(
        routes=(),
        deferred=(
            ("GET", "/g/{token}", "carries ['pattern']"),
            ("POST", "/g/{token}/da-chuyen", "carries ['pattern']"),
            ("GET", "/g/{token}/khong-phai-toi", "carries ['pattern']"),
            ("POST", "/g/{token}/khong-phai-toi", "carries ['pattern']"),
            ("GET", "/g/{token}/doi-so-tien", "carries ['pattern']"),
            ("POST", "/g/{token}/doi-so-tien", "carries ['pattern']"),
            ("POST", "/g/{token}/xin-cach-tinh", "carries ['pattern']"),
        ),
    ),
    # The two-person notebook. Seven routes are refused before their body: a path
    # parameter that is a plain str (purpose, kind) or an int (version), a body
    # holding a date, and a line with an after-validator. Their 422s are
    # hand-written in parity/scenarios/w8/pair_notebooks and pair_papers.
    "w8": Wave(
        routes=(
            ("GET", "/contexts/{context_id}/notebook"),
            ("POST", "/contexts/{context_id}/notebook/proposals"),
            ("POST", "/contexts/{context_id}/notebook/proposals/{proposal_id}/grant"),
            ("POST", "/contexts/{context_id}/notebook/close/preview"),
            ("POST", "/contexts/{context_id}/notebook/close"),
            ("GET", "/contexts/{context_id}/papers"),
            ("POST", "/contexts/{context_id}/papers/draft"),
            ("GET", "/papers/{paper_id}"),
            ("POST", "/papers/{paper_id}/send"),
            ("POST", "/papers/{paper_id}/withdraw"),
            ("POST", "/papers/{paper_id}/skip"),
            ("POST", "/papers/{paper_id}/done"),
        ),
        deferred=(
            (
                "DELETE",
                "/contexts/{context_id}/notebook/consents/{purpose}",
                "path purpose is str",
            ),
            (
                "PUT",
                "/contexts/{context_id}/notebook/constraints/{kind}",
                "path kind is str",
            ),
            (
                "DELETE",
                "/contexts/{context_id}/notebook/constraints/{kind}",
                "path kind is str",
            ),
            ("PATCH", "/papers/{paper_id}/draft", "core schema 'date' is not probed"),
            (
                "POST",
                "/papers/{paper_id}/versions/{version}/viewed",
                "path version is int",
            ),
            (
                "POST",
                "/papers/{paper_id}/versions/{version}/responses",
                "path version is int",
            ),
            (
                "POST",
                "/papers/{paper_id}/keeps",
                "core schema 'function-after' is not probed",
            ),
        ),
    ),
    # The trip: its plan, its arrivals and its invitations. Everything with a body
    # carries a model_validator(mode="after") or a field_validator, and the v2 save
    # declares an idempotency-key header, so only the four bodiless routes render here.
    # The rest of the 422s are hand-written in parity/scenarios/w7/outings.
    "w7": Wave(
        routes=(
            ("GET", "/contexts/{context_id}/outings"),
            ("POST", "/outing-stops/{stop_id}/checkins"),
            ("GET", "/outings/{outing_id}/checkins"),
            ("POST", "/outings/{outing_id}/invites/{invite_id}/revoke"),
            ("POST", "/outings/{outing_id}/invites/{invite_id}/rotate"),
        ),
        deferred=(
            # ItineraryPreviewRequest, ItineraryRequest, OutingCreateRequest and
            # OutingInviteCreateRequest each carry a model_validator(mode="after")
            (
                "POST",
                "/outings/{outing_id}/itinerary/preview",
                "'function-after' is not probed",
            ),
            (
                "POST",
                "/contexts/{context_id}/outings",
                "'function-after' is not probed",
            ),
            ("POST", "/outings/{outing_id}/invites", "'function-after' is not probed"),
            # the v2 save declares idempotency-key as a required header parameter
            ("PUT", "/outings/{outing_id}/itinerary", "headers ['idempotency-key']"),
            # OutingStopInput.at is a clock time with a regex
            ("PUT", "/outings/{outing_id}/timeline", "carries ['pattern']"),
            # a link's secret is a bearer string, not a uuid
            ("POST", "/outing-invites/{token}/accept", "path token is str"),
        ),
    ),
    "w9": Wave(
        routes=(
            ("POST", "/sessions"),
            ("GET", "/sessions"),
            ("DELETE", "/sessions/{session_id}"),
        ),
        excluded=(
            # In dev there is no bearer at all, so this route answers 401 to every
            # step; runner.PersonasRefused reads a scenario whose every persona step
            # is a 401 as a stack whose sessions were not accepted, and stops the run
            # as INFRA. The 401 is the route's honest answer, not a broken stack.
            # parity/scenarios/w9/sessions/prod-sessions.yaml covers it in prod.
            (
                "DELETE",
                "/sessions/current",
                "dev has no bearer, so every step is 401 and PersonasRefused stops the run",
            ),
            # Same reason as POST /friends/lookup and POST /identity/person-id in
            # w2, plus one more that is specific to these three: the address window
            # is spent BEFORE the body is read, so every generated step spends one
            # of ten (or thirty). A corpus of 18-56 steps would become a wall of
            # 429s -- equal on both sides, so green and meaningless -- and would
            # empty the window out from under every other scenario in the run.
            # parity/scenarios/w9/limiter/ covers them by hand instead.
            (
                "POST",
                "/auth/otp/request",
                "body parsed by hand and an in-memory per-IP limiter; hand scenarios only",
            ),
            (
                "POST",
                "/auth/otp/verify",
                "body parsed by hand and an in-memory per-IP limiter; hand scenarios only",
            ),
            (
                "POST",
                "/auth/google",
                "body parsed by hand and an in-memory per-IP limiter; hand scenarios only",
            ),
        ),
    ),
    "w10": Wave(
        routes=(
            ("GET", "/people/me/contexts"),
            ("GET", "/people/me"),
            ("GET", "/people/me/saved-places"),
            ("GET", "/people/me/blocked"),
            ("DELETE", "/people/me"),
            ("POST", "/people/{person_id}/block"),
            ("DELETE", "/people/{person_id}/block"),
            ("POST", "/people/{person_id}/dm"),
            ("GET", "/people/{person_id}"),
        ),
        excluded=(
            (
                "PUT",
                "/people/{person_id}",
                "a valid body registers the shared path placeholder as a person, which"
                " turns later unknown-id steps into known ones; hand scenarios only",
            ),
        ),
        deferred=(
            # ProfileUpdateRequest carries a model_validator(mode="after")
            ("PATCH", "/people/me", "'function-after' is not probed"),
            # place_id is a catalogue key, any string; an unknown one is 404
            ("PUT", "/people/me/saved-places/{place_id}", "path place_id is str"),
            ("DELETE", "/people/me/saved-places/{place_id}", "path place_id is str"),
        ),
    ),
}


def scenario_prefix(wave: str) -> str:
    return f"generated/{wave}-422"


def repo_dir(wave: str) -> str:
    return f"parity/scenarios/generated/{wave}-422"


GENERATOR = "scripts/render_parity_422_scenarios.py"

PERSONA = "owner"
ANONYMOUS = "anonymous"
# W3 checkins and memory comments render 72 and 76 steps; the cap bounds gate time,
# not coverage.
MAX_STEPS = 80
MAX_FILE_BYTES = 1024 * 1024

PATH_UUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
BODY_UUID = "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa"
NOT_A_UUID = "khong-phai-uuid"
SAMPLE_TEXT = "ghi chú (dữ liệu mẫu)"
EXTRA_FIELD = "unknown_field"
SENTINEL = "__render_probe_sentinel__"

# Characters are built with chr() so a formatter cannot turn them into
# invisible literals in this file.
BACKSLASH = chr(0x5C)
BOM = chr(0xFEFF)
BMP_MULTIBYTE = chr(0x1EAF)  # three UTF-8 bytes, one UTF-16 unit
ASTRAL = chr(0x1F600)  # four UTF-8 bytes, two UTF-16 units
LONE_SURROGATE_JSON = BACKSLASH + "ud800"  # JSON escape text, not a character

JSON_CONTENT_TYPE = "application/json"
CONTENT_TYPES: tuple[tuple[str, str | None], ...] = (
    ("absent", None),
    ("text_plain", "text/plain"),
    ("json_latin1", "application/json; charset=latin-1"),
    ("form_urlencoded", "application/x-www-form-urlencoded"),
    ("upper_case", "APPLICATION/JSON"),
    ("vnd_api_json", "application/vnd.api+json"),
)
# The wrong JSON types each field is tried with; kept only where refused.
PROBES: tuple[tuple[str, typing.Any], ...] = (
    ("null", None),
    ("true", True),
    ("zero", 0),
    ("float", 1.5),
    ("string", "x"),
    ("array", []),
    ("object", {}),
)
# Fine; above CPython's recursion limit; above encoding/json's depth limit.
NEST_DEPTHS = (64, 1100, 12000)

ACTOR_ID_HEADER = "x-actor-id"
ACTOR_HEADERS = frozenset(
    {"authorization", "x-actor-id", "x-actor-roles", "x-actor-contexts"}
)

# parity/internal/scenario/scenario.go
TEMPLATE_RE = re.compile(r"\{\{\s*([a-z0-9_.]+)\s*\}\}")
STEP_ID_RE = re.compile(r"^[a-z][a-z0-9_]*$")
HEX64_RE = re.compile(r"(?<![0-9A-Fa-f])[0-9A-Fa-f]{64}(?![0-9A-Fa-f])")

COMMON_KEYS = {"type", "metadata", "ref", "serialization"}


class RenderError(Exception):
    pass


# --------------------------------------------------------------------------
# Reading the application


def bootstrap(app_root: str) -> typing.Any:
    os.environ.setdefault("MOBILE_AUTH_MODE", "dev")
    os.environ["MOBILE_DATABASE_URL"] = (
        "postgresql+psycopg://nobody:nothing@127.0.0.1:9/nothing"
    )
    sys.path.insert(0, app_root)
    from app.api.main import create_app

    return create_app()


def vocabulary() -> dict[tuple[str, str], typing.Any]:
    """Valid values no model states: the domain's closed vocabularies."""
    from app.domain.interests import BUDGET_BAND_IDS, INTEREST_IDS
    from app.places.areas import AREAS
    from app.places.meeting import MIN_ORIGIN_AREAS

    return {
        ("app.api.schemas.InterestsUpdateRequest", "interests"): list(INTEREST_IDS[:2]),
        ("app.api.schemas.InterestsUpdateRequest", "budget_band"): BUDGET_BAND_IDS[0],
        ("app.api.schemas.MeetingPointRequest", "from_areas"): [
            area["id"] for area in AREAS[:MIN_ORIGIN_AREAS]
        ],
    }


def qualified(cls: type) -> str:
    return f"{cls.__module__}.{cls.__qualname__}"


@dataclasses.dataclass(frozen=True)
class Shape:
    kind: str  # str | int | float | bool | uuid | literal | list | model
    nullable: bool = False
    strict: bool = False
    min_length: int | None = None
    max_length: int | None = None
    ge: float | None = None
    gt: float | None = None
    le: float | None = None
    lt: float | None = None
    members: tuple = ()
    items: Shape | None = None
    fields: tuple[FieldIR, ...] = ()
    extra: str | None = None
    model: str | None = None


@dataclasses.dataclass(frozen=True)
class FieldIR:
    name: str
    shape: Shape
    required: bool
    default: typing.Any = None


def allow_keys(schema: dict, keys: set[str], where: str) -> None:
    unknown = set(schema) - COMMON_KEYS - keys
    if unknown:
        raise RenderError(
            f"{where}: core schema {schema['type']!r} carries {sorted(unknown)}, "
            "which this generator does not know how to probe"
        )


def read_shape(schema: dict, defs: dict[str, dict], where: str) -> Shape:
    """pydantic-core schema -> Shape, refusing anything unrecognised."""
    kind = schema["type"]
    if kind == "definitions":
        allow_keys(schema, {"schema", "definitions"}, where)
        defs = {**defs, **{d["ref"]: d for d in schema["definitions"]}}
        return read_shape(schema["schema"], defs, where)
    if kind == "definition-ref":
        allow_keys(schema, {"schema_ref"}, where)
        return read_shape(defs[schema["schema_ref"]], defs, where)
    if kind == "nullable":
        allow_keys(schema, {"schema"}, where)
        inner = read_shape(schema["schema"], defs, where)
        return dataclasses.replace(inner, nullable=True)
    if kind == "str":
        allow_keys(schema, {"strict", "min_length", "max_length"}, where)
        return Shape(
            "str",
            strict=bool(schema.get("strict")),
            min_length=schema.get("min_length"),
            max_length=schema.get("max_length"),
        )
    if kind in ("int", "float"):
        allow_keys(schema, {"strict", "ge", "gt", "le", "lt"}, where)
        return Shape(
            kind,
            strict=bool(schema.get("strict")),
            ge=schema.get("ge"),
            gt=schema.get("gt"),
            le=schema.get("le"),
            lt=schema.get("lt"),
        )
    if kind == "bool":
        allow_keys(schema, {"strict"}, where)
        return Shape("bool", strict=bool(schema.get("strict")))
    if kind == "uuid":
        allow_keys(schema, {"strict"}, where)
        return Shape("uuid", strict=bool(schema.get("strict")))
    if kind == "literal":
        allow_keys(schema, {"expected"}, where)
        members = tuple(schema["expected"])
        if not all(isinstance(m, str) for m in members):
            raise RenderError(f"{where}: non-string Literal members {members!r}")
        return Shape("literal", members=members)
    if kind == "list":
        allow_keys(schema, {"items_schema", "min_length", "max_length"}, where)
        return Shape(
            "list",
            items=read_shape(schema["items_schema"], defs, f"{where}[]"),
            min_length=schema.get("min_length"),
            max_length=schema.get("max_length"),
        )
    if kind == "model":
        allow_keys(
            schema,
            {"cls", "schema", "config", "custom_init", "root_model", "post_init"},
            where,
        )
        if schema.get("root_model") or schema.get("post_init"):
            raise RenderError(f"{where}: root models and post_init are not probed")
        config = schema.get("config") or {}
        allowed_config = {"title", "extra_fields_behavior"}
        if set(config) - allowed_config:
            raise RenderError(
                f"{where}: model config {sorted(set(config) - allowed_config)}"
            )
        inner = schema["schema"]
        if inner["type"] != "model-fields":
            raise RenderError(f"{where}: model body {inner['type']!r}")
        allow_keys(inner, {"fields", "model_name", "computed_fields"}, where)
        fields = []
        for name, field in inner["fields"].items():
            allow_keys(field, {"schema"}, f"{where}.{name}")
            if field["type"] != "model-field":
                raise RenderError(f"{where}.{name}: field kind {field['type']!r}")
            body = field["schema"]
            required = body["type"] != "default"
            default = None
            if not required:
                allow_keys(body, {"schema", "default"}, f"{where}.{name}")
                default = body["default"]
                body = body["schema"]
            fields.append(
                FieldIR(
                    name=name,
                    shape=read_shape(body, defs, f"{where}.{name}"),
                    required=required,
                    default=default,
                )
            )
        return Shape(
            "model",
            fields=tuple(fields),
            extra=config.get("extra_fields_behavior"),
            model=qualified(schema["cls"]),
        )
    raise RenderError(f"{where}: core schema {kind!r} is not probed")


def view(shape: Shape) -> tuple:
    """The facts both readings must agree on."""
    return (
        shape.kind,
        shape.nullable,
        shape.strict,
        shape.min_length,
        shape.max_length,
        shape.ge,
        shape.gt,
        shape.le,
        shape.lt,
        shape.members,
        view(shape.items) if shape.items else None,
        shape.model,
    )


def annotation_view(annotation: typing.Any, metadata: typing.Sequence = ()) -> tuple:
    """The same facts, read from a `model_fields` annotation."""
    import annotated_types
    from pydantic import BaseModel
    from pydantic.fields import FieldInfo
    from pydantic.types import Strict

    facts: dict[str, typing.Any] = {
        "nullable": False,
        "strict": False,
        "min_length": None,
        "max_length": None,
        "ge": None,
        "gt": None,
        "le": None,
        "lt": None,
    }
    bounds = (
        (annotated_types.MinLen, "min_length"),
        (annotated_types.MaxLen, "max_length"),
        (annotated_types.Ge, "ge"),
        (annotated_types.Gt, "gt"),
        (annotated_types.Le, "le"),
        (annotated_types.Lt, "lt"),
    )

    def absorb(items: typing.Sequence) -> None:
        for item in items:
            if isinstance(item, FieldInfo):
                absorb(item.metadata)
            elif isinstance(item, Strict):
                facts["strict"] = item.strict
            else:
                for cls, key in bounds:
                    if isinstance(item, cls):
                        facts[key] = getattr(item, key)
                        break
                else:
                    raise RenderError(
                        f"annotation metadata {type(item).__name__} is not probed"
                    )

    absorb(metadata)
    while True:
        origin = typing.get_origin(annotation)
        if origin is typing.Annotated:
            annotation, *extra = typing.get_args(annotation)
            absorb(extra)
            continue
        if origin in (typing.Union, types.UnionType):
            args = typing.get_args(annotation)
            rest = [arg for arg in args if arg is not type(None)]
            if len(rest) != 1:
                raise RenderError(f"union {annotation!r} is not probed")
            facts["nullable"] = len(rest) != len(args)
            annotation = rest[0]
            continue
        break
    members: tuple = ()
    items = None
    model = None
    if origin is typing.Literal:
        kind, members = "literal", typing.get_args(annotation)
    elif origin is list:
        kind = "list"
        items = annotation_view(typing.get_args(annotation)[0])
    elif annotation in (str, int, float, bool):
        kind = annotation.__name__
    elif annotation is uuid.UUID:
        kind = "uuid"
    elif isinstance(annotation, type) and issubclass(annotation, BaseModel):
        kind, model = "model", qualified(annotation)
    else:
        raise RenderError(f"annotation {annotation!r} is not probed")
    return (
        kind,
        facts["nullable"],
        facts["strict"],
        facts["min_length"],
        facts["max_length"],
        facts["ge"],
        facts["gt"],
        facts["le"],
        facts["lt"],
        members,
        items,
        model,
    )


def cross_check(model_cls: type, shape: Shape) -> None:
    where = qualified(model_cls)
    infos = model_cls.model_fields
    if [f.name for f in shape.fields] != [info.alias or n for n, info in infos.items()]:
        raise RenderError(f"{where}: model_fields and core schema name other fields")
    config_extra = model_cls.model_config.get("extra")
    if config_extra != shape.extra:
        raise RenderError(f"{where}: extra {config_extra!r} vs core {shape.extra!r}")
    for (name, info), field in zip(infos.items(), shape.fields):
        if info.is_required() != field.required:
            raise RenderError(f"{where}.{name}: required differs between readings")
        from_annotation = annotation_view(info.annotation, info.metadata)
        if from_annotation != view(field.shape):
            raise RenderError(
                f"{where}.{name}: annotation says {from_annotation!r}, "
                f"core schema says {view(field.shape)!r}"
            )


def describe(shape: Shape) -> str:
    if shape.kind == "literal":
        text = "literal " + "|".join(shape.members)
    elif shape.kind == "list":
        text = f"list[{describe(shape.items)}]"
    elif shape.kind == "model":
        text = f"model {shape.model}"
    else:
        text = shape.kind
    if shape.strict:
        text += " strict"
    for key in ("min_length", "max_length", "ge", "gt", "le", "lt"):
        if getattr(shape, key) is not None:
            text += f" {key}={getattr(shape, key)}"
    if shape.nullable:
        text += " nullable"
    return text


def walk_dependants(dependant: typing.Any) -> typing.Iterator[typing.Any]:
    yield dependant
    for sub in dependant.dependencies:
        yield from walk_dependants(sub)


@dataclasses.dataclass
class RouteIR:
    method: str
    path: str
    route: typing.Any
    path_params: tuple[tuple[str, Shape], ...]
    actor_header: bool
    body: Shape | None
    body_field: typing.Any
    #: (alias, Shape, FastAPI ModelField) for each query parameter
    query_params: tuple = ()


def query_core_schema(param: typing.Any) -> dict:
    """A query parameter's schema without the `default` wrapper.

    FastAPI applies the default before validation when the key is absent, and
    the header comment reads it from the field; what a present value must be is
    the wrapped schema.
    """
    schema = param._type_adapter.core_schema
    while schema.get("type") == "default":
        schema = schema["schema"]
    return schema


def read_route(app: typing.Any, method: str, path: str) -> RouteIR:
    from fastapi import params
    from fastapi.routing import APIRoute

    where = f"{method} {path}"
    hits = [
        r
        for r in app.routes
        if isinstance(r, APIRoute) and r.path == path and method in r.methods
    ]
    if len(hits) != 1:
        raise RenderError(f"{where}: {len(hits)} APIRoutes match")
    route = hits[0]
    dependants = list(walk_dependants(route.dependant))
    cookies = [p.alias for d in dependants for p in d.cookie_params]
    if cookies:
        raise RenderError(f"{where}: cookie {cookies} not probed")
    query_params = []
    for param in (p for d in dependants for p in d.query_params):
        shape = read_shape(query_core_schema(param), {}, f"{where} query {param.alias}")
        if shape.kind in ("list", "model"):
            raise RenderError(f"{where}: query {param.alias} is {describe(shape)}")
        query_params.append((param.alias, shape, param))
    headers = {p.alias.lower() for d in dependants for p in d.header_params}
    if headers - ACTOR_HEADERS:
        raise RenderError(f"{where}: headers {sorted(headers - ACTOR_HEADERS)}")
    path_params = []
    for param in route.dependant.path_params:
        shape = read_shape(param._type_adapter.core_schema, {}, f"{where} {param.name}")
        if shape.kind != "uuid" or shape.nullable:
            raise RenderError(f"{where}: path {param.name} is {describe(shape)}")
        path_params.append((param.alias, shape))
    body_params = [p for d in dependants for p in d.body_params]
    body = None
    if body_params:
        if len(body_params) != 1 or getattr(route, "_embed_body_fields", False):
            raise RenderError(f"{where}: only one non-embedded body model is probed")
        if isinstance(body_params[0].field_info, (params.Form, params.File)):
            raise RenderError(f"{where}: form bodies are not probed")
        body = read_shape(route.body_field._type_adapter.core_schema, {}, where)
        if body.kind != "model" or body.nullable:
            raise RenderError(f"{where}: body is {describe(body)}")
        cross_check(route.body_field.type_, body)
    return RouteIR(
        method=method,
        path=path,
        route=route,
        path_params=tuple(path_params),
        actor_header=ACTOR_ID_HEADER in headers,
        body=body,
        body_field=route.body_field,
        query_params=tuple(query_params),
    )


# --------------------------------------------------------------------------
# Values


def dumps(value: typing.Any, *, ascii_only: bool = False) -> str:
    return json.dumps(value, ensure_ascii=ascii_only, separators=(",", ":"))


def substitute(value: typing.Any, json_text: str, *, ascii_only: bool = False) -> str:
    """Dump `value`, with the one SENTINEL string replaced by raw JSON text."""
    text = dumps(value, ascii_only=ascii_only)
    marker = dumps(SENTINEL)
    if text.count(marker) != 1:
        raise RenderError("sentinel must occur exactly once")
    return text.replace(marker, json_text)


def utf16_escape(char: str) -> str:
    units = char.encode("utf-16-be")
    return "".join(
        f"{BACKSLASH}u{int.from_bytes(units[i : i + 2], 'big'):04x}"
        for i in range(0, len(units), 2)
    )


def mixed_text(length: int) -> str:
    return "".join(BMP_MULTIBYTE if i % 2 == 0 else ASTRAL for i in range(length))


def near_miss_case(member: str) -> str | None:
    changed = member[:1].upper() + member[1:]
    if changed == member:
        changed = member.lower()
    return None if changed == member else changed


def example(
    shape: Shape, owner: str | None, name: str | None, vocab: dict
) -> typing.Any:
    if (owner, name) in vocab:
        return copy.deepcopy(vocab[(owner, name)])
    if shape.kind == "literal":
        return shape.members[0]
    if shape.kind == "uuid":
        return BODY_UUID
    if shape.kind == "str":
        text = SAMPLE_TEXT
        if shape.max_length is not None:
            text = text[: shape.max_length]
        if shape.min_length is not None and len(text) < shape.min_length:
            text += "a" * (shape.min_length - len(text))
        return text
    if shape.kind in ("int", "float"):
        low = shape.ge if shape.ge is not None else None
        if low is None and shape.gt is not None:
            low = shape.gt + 1
        value = low if low is not None else 1
        return int(value) if shape.kind == "int" else float(value)
    if shape.kind == "bool":
        return True
    if shape.kind == "list":
        item = example(shape.items, None, None, vocab)
        return [item] * max(1, shape.min_length or 0)
    if shape.kind == "model":
        return {
            f.name: example(f.shape, shape.model, f.name, vocab) for f in shape.fields
        }
    raise RenderError(f"no example for {describe(shape)}")


def list_items(shape: Shape, owner: str, name: str, vocab: dict) -> list:
    base = example(shape, owner, name, vocab)
    return base if base else [example(shape.items, None, None, vocab)]


def signature(shape: Shape) -> tuple:
    return view(dataclasses.replace(shape, members=()))


# --------------------------------------------------------------------------
# Steps


@dataclasses.dataclass(frozen=True)
class Step:
    id: str
    as_: str
    method: str
    path: str
    headers: tuple[tuple[str, str], ...]
    body: str | None
    note: str | None


class Steps:
    def __init__(self, where: str) -> None:
        self.where = where
        self.items: list[Step] = []
        self.ids: set[str] = set()
        self.wire: set[tuple] = set()
        self.pending_note: str | None = None

    def note(self, text: str) -> None:
        self.pending_note = text

    def add(
        self,
        step_id: str,
        as_: str,
        method: str,
        path: str,
        headers: typing.Sequence[tuple[str, str]] = (),
        body: str | None = None,
    ) -> None:
        if not STEP_ID_RE.match(step_id) or step_id in self.ids:
            raise RenderError(f"{self.where}: step id {step_id!r}")
        for text in [path, body or "", *(v for _, v in headers)]:
            if TEMPLATE_RE.search(text):
                raise RenderError(f"{self.where}: {step_id} looks like a template")
        key = (as_, method, path, tuple(headers), body)
        if key in self.wire:
            return  # the same request is already in the file under another id
        self.wire.add(key)
        self.ids.add(step_id)
        self.items.append(
            Step(step_id, as_, method, path, tuple(headers), body, self.pending_note)
        )
        self.pending_note = None


def ident(name: str) -> str:
    text = re.sub(r"[^a-z0-9]+", "_", name.lower()).strip("_")
    return text if text[:1].isalpha() else f"f_{text}"


def field_ident(name: str) -> str:
    """A body field's step-id stem, kept clear of the envelope's own ids.

    `body_null`, `body_array` and `body_empty_object` probe the whole body, so a
    field named `body` would give its null probe the envelope's id.
    """
    key = ident(name)
    return f"field_{key}" if key == "body" or key.startswith("body_") else key


def uuid_variants(value: str) -> tuple[tuple[str, str], ...]:
    bare = value.replace("-", "")
    return (
        ("valid", value),
        ("uppercase", value.upper()),
        ("hyphenless", bare),
        ("braced", "%7B" + value + "%7D"),
        ("urn", "urn:uuid:" + value),
        ("not_uuid", NOT_A_UUID),
        ("one_short", value[:-1]),
        ("one_long", value + "a"),
        ("non_hex", "g" + value[1:]),
        ("no_version_bits", value[:14] + "c" + value[15:19] + "c" + value[20:]),
        ("escaped_hyphens", value.replace("-", "%2D")),
        ("non_ascii_tail", value[:-1] + "%C3%A9"),
        ("trailing_space", value + "%20"),
    )


def fill_path(ir: RouteIR, overrides: dict[str, str] | None = None) -> str:
    path = ir.path
    for name, _shape in ir.path_params:
        value = (overrides or {}).get(name, PATH_UUID)
        path = path.replace("{" + name + "}", value)
    return path


class BodyContext:
    def __init__(self, ir: RouteIR, vocab: dict) -> None:
        self.ir = ir
        self.model = ir.body
        self.vocab = vocab
        self.valid = example(ir.body, None, None, vocab)
        if self.errors(self.valid):
            raise RenderError(f"{ir.method} {ir.path}: the example body is refused")

    def errors(self, body: typing.Any) -> list[dict]:
        _value, errors = self.ir.body_field.validate(body, {}, loc=("body",))
        return list(errors or [])

    def refuses(self, body: typing.Any, loc: tuple) -> bool:
        return any(tuple(e["loc"])[1 : 1 + len(loc)] == loc for e in self.errors(body))

    def with_value(self, name: str, value: typing.Any) -> dict:
        body = copy.deepcopy(self.valid)
        body[name] = copy.deepcopy(value)
        return body

    def without(self, name: str) -> dict:
        body = copy.deepcopy(self.valid)
        del body[name]
        return body

    def refused_probes(self, name: str) -> list[tuple[str, typing.Any]]:
        return [
            (label, value)
            for label, value in PROBES
            if self.refuses(self.with_value(name, value), (name,))
        ]

    def refused_item_probes(self, name: str, index: int, items: list) -> list:
        found = []
        for label, value in PROBES:
            body = self.with_value(name, items[:index] + [value])
            if self.refuses(body, (name, index)):
                found.append((label, value))
        return found

    def all_wrong(self) -> dict:
        body = {}
        for field in self.model.fields:
            probes = self.refused_probes(field.name)
            if probes:
                body[field.name] = probes[0][1]
        return body

    def invalid_text(self) -> str:
        if any(field.required for field in self.model.fields):
            return "{}"
        return dumps(self.all_wrong())


def body_steps(ir: RouteIR, steps: Steps, vocab: dict) -> None:
    ctx = BodyContext(ir, vocab)
    model = ir.body
    method, path = ir.method, fill_path(ir)
    ct = (("content-type", JSON_CONTENT_TYPE),)
    valid_text = dumps(ctx.valid)
    first = model.fields[0]

    steps.note("a body every reading accepts; the handler decides the rest")
    steps.add("valid_body", PERSONA, method, path, ct, valid_text)

    steps.note(
        "envelope; an empty body without content-type is the same bytes as no_body"
    )
    steps.add("no_body", PERSONA, method, path)
    steps.add("empty_body_json_content_type", PERSONA, method, path, ct, "")
    steps.add("whitespace_body", PERSONA, method, path, ct, " \t\n ")
    steps.add("body_null", PERSONA, method, path, ct, "null")
    steps.add("body_array", PERSONA, method, path, ct, "[]")
    steps.add("body_empty_object", PERSONA, method, path, ct, "{}")

    for field in model.fields:
        if field.required:
            steps.note(f"required field {field.name} missing")
            steps.add(
                f"missing_{field_ident(field.name)}",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.without(field.name)),
            )

    seen: set[tuple] = set()
    for field in model.fields:
        probes = ctx.refused_probes(field.name)
        repeated = signature(field.shape) in seen
        seen.add(signature(field.shape))
        if repeated:
            probes = [p for p in probes if p[0] == "null"] or probes[:1]
        if not probes:
            continue
        steps.note(
            f"{field.name}: {describe(field.shape)}; wrong JSON types the body "
            "field refuses" + (" (same shape as an earlier field)" if repeated else "")
        )
        for label, value in probes:
            steps.add(
                f"{field_ident(field.name)}_{label}",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.with_value(field.name, value)),
            )

    seen = set()
    for field in model.fields:
        if field.shape.kind != "literal":
            continue
        repeated = signature(field.shape) in seen
        seen.add(signature(field.shape))
        member = field.shape.members[0]
        misses = []
        case = near_miss_case(member)
        if case is not None:
            misses.append(("case_changed", case))
        if not repeated:
            misses.append(("trailing_space", member + " "))
        steps.note(f"{field.name}: near misses of the member {member!r}")
        for label, value in misses:
            steps.add(
                f"{field_ident(field.name)}_{label}",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.with_value(field.name, value)),
            )

    for field in model.fields:
        shape = field.shape
        if shape.kind != "str" or (
            shape.min_length is None and shape.max_length is None
        ):
            continue
        lengths: list[int] = []
        if shape.min_length is not None:
            lengths += [n for n in (shape.min_length - 1, shape.min_length) if n >= 0]
        else:
            lengths.append(0)
        if shape.max_length is not None:
            lengths += [shape.max_length, shape.max_length + 1]
        steps.note(
            f"{field.name}: {describe(shape)}; lengths in code points, alternating "
            "a 3-byte BMP letter and a 4-byte astral emoji"
        )
        for n in lengths:
            text = mixed_text(n)
            steps.add(
                f"{field_ident(field.name)}_len_{n}_mixed",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.with_value(field.name, text)),
            )
        escaped = [n for n in lengths if n > 0]
        if escaped:
            steps.note(
                f"{field.name}: the same astral code point written as a JSON "
                "surrogate-pair escape"
            )
        for n in escaped:
            body = ctx.with_value(field.name, SENTINEL)
            text = substitute(body, '"' + utf16_escape(ASTRAL) * n + '"')
            steps.add(
                f"{field_ident(field.name)}_len_{n}_escaped_astral",
                PERSONA,
                method,
                path,
                ct,
                text,
            )

    for field in model.fields:
        shape = field.shape
        if shape.kind != "list":
            continue
        items = list_items(shape, model.model, field.name, vocab)
        lengths = []
        if shape.min_length is not None:
            lengths += [n for n in (shape.min_length - 1, shape.min_length) if n >= 0]
        if shape.max_length is not None:
            lengths += [shape.max_length, shape.max_length + 1]
        if lengths:
            steps.note(f"{field.name}: {describe(shape)} at its length bounds")
        for n in lengths:
            value = [items[i % len(items)] for i in range(n)]
            steps.add(
                f"{field_ident(field.name)}_len_{n}",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.with_value(field.name, value)),
            )
        head = ctx.refused_item_probes(field.name, 0, items)
        tail = ctx.refused_item_probes(field.name, len(items), items)
        tail = [p for p in tail if p[0] != "null"] or tail
        if head or tail:
            steps.note(f"{field.name}: an item of the wrong type ({describe(shape)})")
        if head:
            label, value = head[0]
            steps.add(
                f"{field_ident(field.name)}_item_{label}_first",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.with_value(field.name, [value])),
            )
        if tail:
            label, value = tail[0]
            steps.add(
                f"{field_ident(field.name)}_item_{label}_after_valid",
                PERSONA,
                method,
                path,
                ct,
                dumps(ctx.with_value(field.name, items + [value])),
            )

    for field in model.fields:
        shape = field.shape
        if shape.kind == "str":
            body = ctx.with_value(field.name, SENTINEL)
        elif shape.kind == "list" and shape.items.kind == "str":
            body = ctx.with_value(field.name, [SENTINEL])
        else:
            continue
        steps.note(f"{field.name}: a lone high surrogate as a JSON escape")
        steps.add(
            f"{field_ident(field.name)}_lone_surrogate",
            PERSONA,
            method,
            path,
            ct,
            substitute(body, '"' + LONE_SURROGATE_JSON + '"'),
        )
        break

    steps.note(f"unknown keys (extra={model.extra}) and key order")
    with_extra = copy.deepcopy(ctx.valid)
    with_extra[EXTRA_FIELD] = "x"
    steps.add(
        "valid_body_plus_extra_field", PERSONA, method, path, ct, dumps(with_extra)
    )
    all_wrong = ctx.all_wrong()
    all_wrong[EXTRA_FIELD] = "x"
    steps.add(
        "every_field_wrong_plus_extra_field",
        PERSONA,
        method,
        path,
        ct,
        dumps(all_wrong),
    )
    first_probes = ctx.refused_probes(first.name)
    if first_probes:
        body = ctx.with_value(first.name, first_probes[0][1])
        text = dumps(body)
        duplicated = f"{text[:-1]},{dumps(first.name)}:{dumps(ctx.valid[first.name])}}}"
        steps.add("duplicate_key_valid_last", PERSONA, method, path, ct, duplicated)

    steps.note("JSON syntax the decoder has to refuse, or quietly accepts")
    cut_points = sorted({1, len(valid_text) // 2, len(valid_text) - 1})
    for n in cut_points:
        steps.add(f"json_truncated_at_{n}", PERSONA, method, path, ct, valid_text[:n])
    if not valid_text.endswith("}"):
        raise RenderError("the example body does not end with an object brace")
    steps.add("json_trailing_comma", PERSONA, method, path, ct, valid_text[:-1] + ",}")
    steps.add(
        "json_single_quotes",
        PERSONA,
        method,
        path,
        ct,
        valid_text.replace('"', "'"),
    )
    steps.add("json_bom_prefix", PERSONA, method, path, ct, BOM + valid_text)
    for label, literal in (("nan", "NaN"), ("infinity", "Infinity")):
        steps.add(
            f"json_{label}_value",
            PERSONA,
            method,
            path,
            ct,
            substitute(ctx.with_value(first.name, SENTINEL), literal),
        )
    steps.add("json_trailing_data", PERSONA, method, path, ct, valid_text + "{}")
    for depth in NEST_DEPTHS:
        nested = "[" * depth + "]" * depth
        steps.add(
            f"json_nested_depth_{depth}",
            PERSONA,
            method,
            path,
            ct,
            substitute(ctx.with_value(first.name, SENTINEL), nested),
        )
    ascii_text = dumps(ctx.valid, ascii_only=True)
    steps.add(
        "json_utf16le_without_bom",
        PERSONA,
        method,
        path,
        ct,
        ascii_text.encode("utf-16-le").decode("ascii"),
    )

    steps.note("content-type spellings around the same valid body")
    for label, value in CONTENT_TYPES:
        headers = () if value is None else (("content-type", value),)
        steps.add(f"content_type_{label}", PERSONA, method, path, headers, valid_text)

    malformed = valid_text[: len(valid_text) // 2]
    invalid = ctx.invalid_text()
    steps.note(
        "precedence: JSON decoding, the actor dependency, then path and body validation"
    )
    steps.add("anonymous_malformed_json", ANONYMOUS, method, path, ct, malformed)
    steps.add("anonymous_invalid_body", ANONYMOUS, method, path, ct, invalid)
    steps.add("owner_malformed_json", PERSONA, method, path, ct, malformed)
    steps.add("owner_invalid_body", PERSONA, method, path, ct, invalid)
    if ir.actor_header:
        # JSON decoding already shows ahead of the actor in the anonymous step
        bad_actor = ct + ((ACTOR_ID_HEADER, NOT_A_UUID),)
        steps.add(
            "bad_actor_id_invalid_body", PERSONA, method, path, bad_actor, invalid
        )
    for name, _shape in ir.path_params:
        bad_path = fill_path(ir, {name: NOT_A_UUID})
        key = ident(name)
        steps.add(
            f"anonymous_bad_{key}_valid_body",
            ANONYMOUS,
            method,
            bad_path,
            ct,
            valid_text,
        )
        steps.add(f"bad_{key}_invalid_body", PERSONA, method, bad_path, ct, invalid)
        steps.add(f"bad_{key}_malformed_json", PERSONA, method, bad_path, ct, malformed)

    for name, shape in ir.path_params:
        steps.note(f"path {name}: {describe(shape)}; the body is the valid one")
        for label, value in uuid_variants(PATH_UUID):
            steps.add(
                f"path_{ident(name)}_{label}",
                PERSONA,
                method,
                fill_path(ir, {name: value}),
                ct,
                valid_text,
            )


def bodyless_steps(ir: RouteIR, steps: Steps) -> None:
    method, path = ir.method, fill_path(ir)
    if ir.path_params:
        for name, shape in ir.path_params:
            steps.note(f"path {name}: {describe(shape)}")
            for label, value in uuid_variants(PATH_UUID):
                steps.add(
                    f"path_{ident(name)}_{label}",
                    PERSONA,
                    method,
                    fill_path(ir, {name: value}),
                )
    else:
        if ir.query_params:
            steps.note("no path or body parameters; query parameters follow")
        else:
            steps.note("no path, query or body parameters to validate")
        steps.add("owner_plain", PERSONA, method, path)
        steps.add("anonymous_plain", ANONYMOUS, method, path)

    steps.note("inputs this route does not declare are ignored by FastAPI")
    query_name = ir.path_params[0][0] if ir.path_params else EXTRA_FIELD
    steps.add("undeclared_query_parameter", PERSONA, method, f"{path}?{query_name}=x")
    steps.add(
        "undeclared_malformed_json_body",
        PERSONA,
        method,
        path,
        (("content-type", JSON_CONTENT_TYPE),),
        '{"x": ',
    )
    if not ir.actor_header:
        steps.add(
            "undeclared_actor_header_not_a_uuid",
            PERSONA,
            method,
            path,
            ((ACTOR_ID_HEADER, NOT_A_UUID),),
        )
        return
    steps.note("precedence: the actor dependency before path validation")
    for name, _shape in ir.path_params:
        bad_path = fill_path(ir, {name: NOT_A_UUID})
        key = ident(name)
        steps.add(f"anonymous_bad_{key}", ANONYMOUS, method, bad_path)
        steps.add(
            f"bad_actor_id_bad_{key}",
            PERSONA,
            method,
            bad_path,
            ((ACTOR_ID_HEADER, NOT_A_UUID),),
        )
    steps.add(
        "bad_actor_id_valid_path",
        PERSONA,
        method,
        path,
        ((ACTOR_ID_HEADER, NOT_A_UUID),),
    )


#: Query strings each query parameter is tried with; kept only where refused.
QUERY_PROBES: tuple[tuple[str, str], ...] = (
    ("empty", ""),
    ("text", "x"),
    ("float", "1.5"),
    ("exponent", "1e2"),
    ("plus_sign", "+1"),
    ("space", " 1"),
    ("zero", "0"),
    ("negative", "-1"),
)


def query_url(path: str, pairs: typing.Sequence[tuple[str, str]]) -> str:
    return (
        path
        + "?"
        + "&".join(
            f"{urllib.parse.quote(name, safe='')}={urllib.parse.quote(value, safe='')}"
            for name, value in pairs
        )
    )


def query_refuses(field: typing.Any, alias: str, value: str) -> bool:
    """FastAPI's own query field decides, as `request_params_to_args` does.

    A present key reaches validation as its string, the empty string included;
    Starlette hands over the last value of a repeated key.
    """
    _value, errors = field.validate(value, {}, loc=("query", alias))
    return bool(errors)


def query_valid_value(alias: str, shape: Shape, field: typing.Any) -> str:
    candidates = [
        str(int(b) + d) for b, d in ((shape.ge, 0), (shape.gt, 1)) if b is not None
    ]
    for value in [*candidates, "1", "x"]:
        if not query_refuses(field, alias, value):
            return value
    raise RenderError(f"query {alias}: no candidate value is accepted")


def query_steps(ir: RouteIR, steps: Steps) -> None:
    method, path = ir.method, fill_path(ir)
    valid = {
        alias: query_valid_value(alias, shape, field)
        for alias, shape, field in ir.query_params
    }
    steps.note("every query parameter at a value its field accepts")
    steps.add("query_all_valid", PERSONA, method, query_url(path, list(valid.items())))
    for alias, shape, field in ir.query_params:
        key = ident(alias)
        probes = list(QUERY_PROBES)
        for label, bound, delta in (
            ("below_ge", shape.ge, -1),
            ("above_le", shape.le, 1),
            ("at_gt", shape.gt, 0),
            ("at_lt", shape.lt, 0),
        ):
            if bound is not None:
                probes.append((label, str(int(bound) + delta)))
        refused = [(lb, v) for lb, v in probes if query_refuses(field, alias, v)]
        if not refused:
            continue
        steps.note(f"query {alias}: {describe(shape)}; strings the query field refuses")
        for label, value in refused:
            steps.add(
                f"query_{key}_{label}",
                PERSONA,
                method,
                query_url(path, [(alias, value)]),
            )
        label, value = refused[0]
        steps.note(f"query {alias}: a repeated key is read from its last value")
        steps.add(
            f"query_{key}_repeated_last_refused",
            PERSONA,
            method,
            query_url(path, [(alias, valid[alias]), (alias, value)]),
        )
        steps.add(
            f"query_{key}_repeated_last_accepted",
            PERSONA,
            method,
            query_url(path, [(alias, value), (alias, valid[alias])]),
        )
        if ir.actor_header:
            steps.note(f"precedence: the actor dependency before query {alias}")
            steps.add(
                f"anonymous_query_{key}_{label}",
                ANONYMOUS,
                method,
                query_url(path, [(alias, value)]),
            )
        for name, _shape in ir.path_params:
            steps.note(f"precedence: path {name} and query {alias} refused together")
            steps.add(
                f"query_{key}_{label}_bad_{ident(name)}",
                PERSONA,
                method,
                query_url(fill_path(ir, {name: NOT_A_UUID}), [(alias, value)]),
            )


# --------------------------------------------------------------------------
# Files


def yaml_plain(char: str) -> bool:
    code = ord(char)
    if code in (0x85, 0x2028, 0x2029, 0xFEFF):
        return False
    return (
        0x20 <= code <= 0x7E
        or 0xA0 <= code <= 0xD7FF
        or 0xE000 <= code <= 0xFFFD
        or 0x10000 <= code <= 0x10FFFF
    )


def yaml_scalar(text: str) -> str:
    if all(yaml_plain(c) for c in text):
        return "'" + text.replace("'", "''") + "'"
    named = {"\t": "t", "\n": "n", "\r": "r", '"': '"', BACKSLASH: BACKSLASH}
    out = ['"']
    for char in text:
        code = ord(char)
        if char in named:
            out.append(BACKSLASH + named[char])
        elif yaml_plain(char):
            out.append(char)
        elif code < 0x100:
            out.append(f"{BACKSLASH}x{code:02x}")
        elif code < 0x10000:
            out.append(f"{BACKSLASH}u{code:04x}")
        else:
            out.append(f"{BACKSLASH}U{code:08x}")
    out.append('"')
    return "".join(out)


def file_stem(method: str, path: str) -> str:
    slug = path.strip("/").replace("{", "").replace("}", "").replace("/", "-")
    return f"{method.lower()}-{slug}"


def header_comment(ir: RouteIR) -> list[str]:
    lines = [
        f"# Generated by {GENERATOR} inside the pinned API image;",
        "# edit the generator, not this file. Requests only: the expected answer is",
        "# whatever the Python reference returns (ADR-0029 §2.4, ADR-0010 §6.1).",
        f"# Route: {ir.method} {ir.path}",
    ]
    for name, shape in ir.path_params:
        lines.append(f"# Path {name}: {describe(shape)}")
    if ir.body is None:
        lines.append("# Body: none declared")
    else:
        lines.append(f"# Body: {ir.body.model} (extra={ir.body.extra})")
        for field in ir.body.fields:
            state = "required" if field.required else f"default {field.default!r}"
            lines.append(f"#   {field.name}: {describe(field.shape)}, {state}")
    actor = "X-Actor-ID via get_actor" if ir.actor_header else "none"
    if not ir.query_params:
        lines.append(f"# Query: none. Actor header: {actor}.")
        return lines
    for alias, shape, field in ir.query_params:
        state = "required" if field.required else f"default {field.default!r}"
        lines.append(f"# Query {alias}: {describe(shape)}, {state}")
    lines.append(f"# Actor header: {actor}.")
    return lines


def render_file(ir: RouteIR, steps: list[Step], prefix: str) -> tuple[str, dict]:
    stem = file_stem(ir.method, ir.path)
    scenario_id = f"{prefix}/{stem}"
    route_id = f"{ir.method} {ir.path}"
    lines = header_comment(ir)
    lines += [
        f"id: {scenario_id}",
        f"routes: [{yaml_scalar(route_id)}]",
        "auth_mode: dev",
        "personas:",
        f"  {PERSONA}: {{}}",
        "steps:",
    ]
    intended_steps = []
    for step in steps:
        if step.note:
            lines.append(f"  # {step.note}")
        lines += [
            f"  - id: {step.id}",
            f"    as: {step.as_}",
            "    request:",
            f"      method: {step.method}",
            f"      path: {yaml_scalar(step.path)}",
        ]
        request: dict[str, typing.Any] = {"method": step.method, "path": step.path}
        if step.headers:
            lines.append("      headers:")
            for name, value in step.headers:
                lines.append(f"        {name}: {yaml_scalar(value)}")
            request["headers"] = dict(step.headers)
        if step.body is not None:
            lines.append(f"      body_raw: {yaml_scalar(step.body)}")
            request["body_raw"] = step.body
        intended_steps.append({"id": step.id, "as": step.as_, "request": request})
    intended = {
        "id": scenario_id,
        "routes": [route_id],
        "auth_mode": "dev",
        "personas": {PERSONA: {}},
        "steps": intended_steps,
    }
    return "\n".join(lines) + "\n", intended


def load_guard() -> typing.Any:
    location = pathlib.Path(__file__).resolve().with_name("repo_guard.py")
    spec = importlib.util.spec_from_file_location("repo_guard", location)
    if spec is None or spec.loader is None:
        raise RenderError(f"cannot load {location}")
    module = importlib.util.module_from_spec(spec)
    sys.dont_write_bytecode = True
    sys.modules[spec.name] = module  # its dataclasses look themselves up here
    spec.loader.exec_module(module)
    return module


def guard_hits(guard: typing.Any, directory: str, name: str, text: str) -> list[str]:
    repo_path = f"{directory}/{name}"
    raw = text.encode("utf-8")
    hits = []
    if len(raw) >= MAX_FILE_BYTES:
        hits.append("not below 1 MiB")
    if guard.is_binary(raw):
        hits.append("the guard reads it as binary")
    for check in ("is_forbidden_path", "is_junk_path", "is_stray_name"):
        if getattr(guard, check)(repo_path):
            hits.append(check)
    if guard.has_export_filename(repo_path):
        hits.append("export-filename")
    findings = guard.content_findings(
        path=repo_path,
        raw=raw,
        file_number=1,
        config=guard.load_config(None),
        digest="",
        line_numbers=None,
        commit=None,
    )
    hits += [f"{f.rule} at line {f.line}" for f in findings]
    for number, line in enumerate(text.splitlines(), 1):
        if HEX64_RE.search(line):
            hits.append(f"64-hex at line {number}")
    return hits


def route_steps(app: typing.Any, method: str, path: str, vocab: dict) -> tuple:
    ir = read_route(app, method, path)
    steps = Steps(f"{method} {path}")
    if ir.body is not None:
        body_steps(ir, steps, vocab)
    else:
        bodyless_steps(ir, steps)
    if ir.query_params:
        query_steps(ir, steps)
    return ir, steps


def render(app: typing.Any, wave_name: str) -> tuple[dict[str, str], dict[str, int]]:
    import yaml

    wave = WAVES[wave_name]
    vocab = vocabulary()
    guard = load_guard()
    files: dict[str, str] = {}
    counts: dict[str, int] = {}
    for method, path in wave.routes:
        ir, steps = route_steps(app, method, path, vocab)
        if not steps.items or len(steps.items) > MAX_STEPS:
            raise RenderError(
                f"{method} {path}: {len(steps.items)} steps, the cap is {MAX_STEPS}"
            )
        text, intended = render_file(ir, steps.items, scenario_prefix(wave_name))
        name = file_stem(method, path) + ".yaml"
        if yaml.safe_load(text) != intended:
            raise RenderError(f"{name}: YAML does not read back as the steps")
        hits = guard_hits(guard, repo_dir(wave_name), name, text)
        if hits:
            raise RenderError(f"{name} would trip the repository guard: {hits}")
        files[name] = text
        counts[f"{method} {path}"] = len(steps.items)
    for method, path, reason in wave.deferred:
        try:
            route_steps(app, method, path, vocab)
        except RenderError as exc:
            if reason in str(exc):
                continue
            raise RenderError(
                f"{method} {path}: deferred for {reason!r}, now refused with: {exc}"
            ) from exc
        raise RenderError(
            f"{method} {path}: deferred for {reason!r} but it renders now; add it to the wave"
        )
    return files, counts


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    parser.add_argument("--wave", required=True, choices=sorted(WAVES))
    parser.add_argument("--out", required=True, type=pathlib.Path)
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--app-root", default="/srv")
    args = parser.parse_args()
    try:
        files, counts = render(bootstrap(args.app_root), args.wave)
    except RenderError as exc:
        print(f"render_parity_422_scenarios: {exc}", file=sys.stderr)
        return 2
    wave = WAVES[args.wave]
    for method, path, reason in wave.deferred:
        print(
            f"render_parity_422_scenarios: deferred {method} {path}: refused ({reason})",
            file=sys.stderr,
        )
    for method, path, reason in wave.excluded:
        print(
            f"render_parity_422_scenarios: excluded {method} {path}: {reason}",
            file=sys.stderr,
        )
    existing = {p.name for p in args.out.glob("*.yaml")} if args.out.is_dir() else set()
    if args.check:
        drift = sorted(
            name
            for name in existing | set(files)
            if name not in files
            or name not in existing
            or (args.out / name).read_text(encoding="utf-8") != files[name]
        )
        for name in drift:
            print(
                f"render_parity_422_scenarios: {name} is out of date", file=sys.stderr
            )
        return 1 if drift else 0
    args.out.mkdir(parents=True, exist_ok=True)
    for name in sorted(existing - set(files)):
        (args.out / name).unlink()
    for name, text in sorted(files.items()):
        (args.out / name).write_text(text, encoding="utf-8")
    for route, count in counts.items():
        print(
            f"render_parity_422_scenarios: {count:3d} steps  {route}", file=sys.stderr
        )
    print(
        f"render_parity_422_scenarios: wrote {len(files)} files to {args.out}",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
