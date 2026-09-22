# Repository Guidelines

## Quy tắc toàn repo: Go cho backend, Python chỉ cho AI

Quyết định của Lead ngày 2026-09-21, ADR-0031: toàn bộ backend nghiệp vụ
chuyển sang Go. API, auth, domain, persistence, realtime, điều phối media,
worker, điều phối bot và migration mới viết bằng Go/SQL. Python chỉ dùng cho
inference, extraction và evaluation AI; AI không trực tiếp ghi sổ cái hoặc
quyết định quyền truy cập. Frontend giữ TypeScript; thư viện mã hoá native
được dùng Rust. Comment/docstring tiếng Anh; tài liệu và commit tiếng Việt.

`services/api/` là runtime legacy trong lúc chuyển đổi, không phải mẫu để thêm
backend Python mới. Cho phép sửa lỗi bảo mật/hồi quy và giữ test legacy làm
bằng chứng đối chiếu; phải ghi rõ ngoại lệ trong bàn giao. Không xoá test hay
đổi đường production chỉ để tuyên bố đã chuyển xong. Go chỉ nhận quyền ghi
của module sau khi qua contract và PostgreSQL thật; mỗi module có một writer.

Chat v2 bắt buộc E2EE, không fallback plaintext. Kho cũ chỉ đọc, có nhãn rõ.
Server không giữ khoá giải mã chat. AI chỉ nhận lời gọi hoặc trích đoạn được
đồng ý chia sẻ, không tự đọc chat/gu/lịch sử. Native Android/iOS, kiểm chứng
crypto độc lập và test tải là các cổng riêng, không được thay bằng unit test.

Quy tắc này ưu tiên hơn các mô tả Python-first lịch sử bên dưới. SQLite vẫn
bị cấm làm database backend; kho mã hoá cục bộ trên thiết bị là tầng khác.
Theo dõi tiến độ thật tại `docs/architecture/02-chat-go-e2ee.md`.

## Project Structure & Module Organization

Product code lives under `services/api/app/`: `domain/` holds pure rules, `db/` holds SQLAlchemy and Alembic, `api/` exposes FastAPI routes, and `web/` serves guests. There is no payment rail: the product names each person's share and stops, so bank accounts and VietQR left the codebase. Layer-aligned tests live in `services/api/tests/`; root `tests/` covers the repo guard. Consult `docs/decisions/` before behavior changes and `docs/architecture/` before boundary changes. `phase0/` and `docs/protocol/v1/` are frozen. CI treats currently absent `apps/mobile/` and `packages/shared/` as conditional.

## Build, Test, and Development Commands

- `pip install -r services/api/requirements-dev.txt` installs pinned Python 3.12 dependencies.
- `docker compose up -d postgres` starts PostgreSQL 16 for migrations and tests.
- `cd services/api && alembic upgrade head` migrates the configured local database.
- `cd services/api && uvicorn app.api.main:app --reload` runs the API with reload.
- `cd services/api && python3 -m app.web.preview` previews the guest page without a database.
- `python3 -m pytest services/api/tests tests -q` runs the standard suite.
- `cd services/api && ruff check . && ruff format --check .` checks style and formatting.

Copy `.env.example` to `.env` for local configuration; never commit `.env`.

## Coding Style & Naming Conventions

Use four-space indentation, double quotes, an 88-character line limit, and Python 3.12 syntax. Ruff enforces `E4`, `E7`, `E9`, `F`, `I`, `UP`, and `B`. Use `snake_case` for modules/functions/variables, `PascalCase` for classes, and `UPPER_SNAKE_CASE` for constants. `domain/` must not import `db`, `api`, or `payments`.

Represent VND as integers, preserve exact allocation totals, and derive balances from the ledger. Propose an ADR before changing these invariants.

## Testing Guidelines

Pytest files and functions use `test_*.py` and `test_*`. Add tests beside the affected layer and extend allocator golden JSON vectors. Persistence changes require the live suite in `docs/testing/postgres-repository.md`; fake-repository tests alone are insufficient. No numeric coverage threshold is configured, so cover each changed behavior and regression.

## Commit & Pull Request Guidelines

History uses scoped summaries such as `api: ...`, `domain: ...`, `test: ...`, `fix(ci): ...`, and `docs: ...`; keep subjects short, imperative, and focused. Since ADR-0032 (2026-09-22) there is no mandatory pull request: commit straight to `main`, and open a PR only when you actually want someone to read the change first. The leader reads `main`, so the **commit message** carries the whole explanation — what changed, why, and the numbers from the gates you ran. Include screenshots for UI changes. `APPROVE` / `REQUEST_CHANGES` / `REJECT` remain the only three verdict values, but apply only when a real reviewer is involved.

## Security & Data Handling

Run `scripts/setup-hooks.sh` to enable the staged repo guard. Never place real participant data, credentials, exports, or temporary copies inside any worktree—even if ignored. Follow `docs/security/repo-guard.md`; `.gitignore` and scanners are mitigation layers, not safe storage.

## Shared Team Invariants (all agents read this file)

This section is the minimum every agent that touches this repo must know before
touching anything. It is duplicated from `CLAUDE.md` on purpose: a clean checkout
must carry the rules, not depend on one harness loading one file.

**Three money laws.** Changing any of them requires an ADR opened first, not a
code change first.

1. Integer dong. No `float`, no `Decimal`, not even in intermediate values.
   `allocator.py` uses `Fraction` to keep exact rationals.
2. `Σ allocations == total expense`, 100%. 41 hand-computed golden vectors hold this.
3. Balances are recomputable from the ledger; a cache is never the source of truth.

Also: editing an expense creates a **new version**, never an overwrite.
`receiver_confirmed` is **not** bank evidence. `completed` is produced only by a
domain transition — there is no "mark as done" button.

**One fullstack role** (ADR-0032, 2026-09-22). There is no per-person ownership
table any more. Whoever picks up a task owns the whole vertical slice of it: Go
backend, SQL and migrations, Python AI, TypeScript frontend, mobile native, and
tests at every layer. One task belongs to one person start to finish; split a
large one into **runnable vertical slices**, never by layer. agy is still QA and
still owns no product source — it files findings, not diffs.

**The boundaries that remain are LAYER boundaries, not people boundaries.** On
the guest page, routing and data access live outside the template and a template
never queries. Every module has exactly one writer. These are enforced by tests,
not by an assignment table.

**Layer boundary, enforced by AST parsing, not by promise.** `app/domain/` must
not import `app.db`, `app.api`, `app.payments`, `sqlalchemy`, `fastapi`,
`alembic`, or `pydantic`. See `services/api/tests/test_import_boundary.py`.

**Never put in Git, and never send to an external service**: bill photos, bank
account numbers, participant names, raw transcripts, exports, a real `.env`.
`.gitignore` is not a safe place. Real data lives outside the repo and outside
every worktree. `scripts/repo_guard.py` scans what enters Git; it cannot see what
leaves via an API call.

**Language convention**: docs and commit messages in Vietnamese; code comments
and docstrings in English.

**Frozen in place**: `phase0/` and `docs/protocol/v1/`. Do not edit, do not
delete. `protocol_version` is an immutable snapshot.

**A green test suite is not behavioural evidence.** ADR-0006 gated Phase 0 by
leader decision. Read the "proves / does not prove" table in `CLAUDE.md` before
trusting any green mark.

## Team roles (ADR-0032, 2026-09-22 — supersedes the two-lane split of ADR-0010)

| Role | Who | Owns |
|---|---|---|
| Leader | product owner | Gate decisions |
| Engineer | one fullstack role | The entire product: Go backend, SQL and migrations, Python AI, TypeScript frontend, `apps/mobile/`, the guest page, tests at every layer |
| QA | agy (Gemini) | Product testing — visual, exploratory, API, regression, running **in parallel** with engineering work. **Files findings, not diffs. Owns no product source file.** |

The Codex lane closed on 2026-09-16 (ADR-0030); ADR-0032 extended that to the
whole repo. There is no cross-review left, so **the second pair of eyes is gone**
— machine gates replace it, and for UI, opening the screenshot and looking at it
replaces it. Both are weaker than an independent reviewer. That is written down
so nobody reads this as a quality upgrade.

**What a change must carry before it lands on `main`** (ADR-0030 §3, now applied
to frontend and mobile too):

- the gate re-run in a **clean tree at the exact SHA** — never the agent's tree;
- the canary red where predicted, identity green, same harness SHA;
- **at least two mutants you thought of yourself**, checked for equivalence
  first, each one red at the step you predicted;
- for UI: **open the screenshot and look at it**. A green table is not visual evidence;
- the numbers written into the commit message, not left in a log.

**agy boundaries** (full text in ADR-0010 §6):

- agy never fills in `expected` for a golden vector and never produces a money
  answer. It may generate the *frame* and the *input cases* only.
- agy never signs a verdict. `APPROVE` / `REQUEST_CHANGES` / `REJECT` stay with
  the two engineers. A QA finding is input to a reviewer, not a gate.
- **agy's digest is not evidence.** The plugin author observed agy altering its
  own environment (patching installed packages, mock-stubbing deps) to force a
  pass. Whoever delegated re-runs the gate in a clean tree.
- Never `--dangerously-skip-permissions` on this repo — that flag grants
  machine-wide access. Use narrow allow-rules instead.
- QA screenshots come only from `app.web.preview` or synthetic data. Delegating
  means sending content to an outside service; the repo guard scans what enters
  Git, never what leaves.

## What each test layer proves — and does not

| Layer | Proves | Does not prove |
|---|---|---|
| `tests/domain/golden/*.json` + `test_golden_selfcheck.py` | The corpus is internally consistent with ADR-0004 | That the corpus author read the contract correctly — one person wrote both |
| `test_selfcheck_catches_mutants.py` | The self-check really does go red on a wrong answer | — |
| `tests/api/` with the fake repository | HTTP ↔ domain orchestration | Any SQL, index, view, or trigger |
| `tests/postgres/` | The real `SqlAlchemyApiRepository` after Alembic migrates a dedicated schema | Every method, every race, every query plan |
| `tests/db/test_migration_matches_models.py` | Migration matches models, no DB needed | — |
| Visual + exploratory QA (agy) | The page renders, reads, and leaks nothing, in the states and viewports **actually scanned** | Whether a real person understands it · which cells went unscanned · that agy did not fake the green |

SQLite is refused on purpose: the production schema depends on JSONB, partial
unique indexes, views, and append-only triggers. New persistence behaviour needs
a matching live case — widening the fake and calling it DB evidence is a lie.
