# Codebase Documentation — Design

## Goal

Give a returning developer (the project owner, after months away) and AI coding agents a fast, reliable way to understand kea's architecture, domain logic, conventions, and history well enough to add features.

Primary audience: contributors (human and AI). End-user documentation is out of scope.

## Problems Being Solved

1. No big-picture view of layers and request flow (CLI and HTTP → service → store).
2. No written patterns for adding a command, endpoint, SPA page, migration, or service method.
3. Domain logic (double-entry, transaction types, reconciliation, opening balances, multi-currency reports) is only in code.
4. The reasons behind past decisions are buried in ~160 plan/spec files.
5. Dev setup and testing patterns are only partly documented.

Additional facts found during exploration:
- `CLAUDE.md` and `AGENTS.md` are near-duplicates and stale (no mention of `internal/api`, `internal/web`, `spa/`, `kea serve`, `scripts/`).
- No root `README.md`.
- `spa/README.md` is outdated (claims a single `/balances` route).
- Only one Go package has a `// Package` comment.

## Layout

```
AGENTS.md              canonical agent/contributor entry point (~100–150 lines)
CLAUDE.md              contains only "@AGENTS.md"
README.md              new, for humans: what kea is, quick start, links into docs/
SKILL.md               unchanged purpose (agent guide for operating the kea CLI); linked from AGENTS.md
spa/README.md          rewritten: current routes, stack, commands; links to docs/
docs/
  README.md            docs index ("if you want to X, read Y")
  architecture.md
  domain.md
  development.md
  http-api.md
  recipes/
    add-cli-command.md
    add-api-endpoint.md
    add-spa-page.md
    add-migration.md
    add-service-method.md
  decisions.md
  history/             all pre-existing docs/* files, committed
```

`docs/superpowers` is currently gitignored, so only part of the history is tracked. The `.gitignore` entry is removed and the full history (tracked and previously ignored files) is moved into `docs/history/` and committed, so `decisions.md` can link to every source.

The root `TODO` file is gitignored private notes and stays untouched. `.superpowers/` and `.idea/` are untouched.

### AGENTS.md contents

1. What kea is (two sentences).
2. Commands: build, test, run, Docker.
3. Hard rules (breaking them breaks things): amounts are int64 cents; splits sum to zero; only leaf accounts hold transactions; reconciled records are immutable (`ErrReconciled`, check with `errors.Is`); system accounts `Equity:OpeningBalances_<CCY>` must not be deleted; all store methods take `context.Context` and use `*Context` database/sql methods; English only.
4. "Where to look" table: task → doc.
5. Docs maintenance rule: when you change a pattern, endpoint, or domain rule, update the matching doc in the same commit.
6. Known Docker limitation (`TestSwap_FailedSwapKeepsOldConnection` fails as root).

## Doc Contents

**docs/architecture.md**
- ASCII layer diagram: `cmd/` and `internal/api` → `service` → `repository` interfaces → `store` (SQLite); `internal/app` as wiring.
- Two traced request flows with file paths: `kea add` (CLI → huh prompts → service → `ExecTx` → store) and `POST /api/transactions` (router → middleware → handler → service → store).
- Package map: one line per package — what it owns, what it must not import.
- Startup sequence: config loading, ledger registry, backup, migrations, legacy system-account rename.
- How `kea serve` embeds the SPA (`internal/web/dist`).

**docs/domain.md**
- Accounting model: account type tree, leaf-only postings, splits, sum-to-zero.
- Transaction types: definitions, how they are determined and stored, related backfill migrations.
- Opening balances and per-currency system accounts.
- Reconciliation lifecycle: states, immutability, last reconciled balance.
- Reports (balance sheet, income statement, net worth) and multi-currency handling.
- Each concept ends with a "Code lives in:" line.

**docs/development.md**
- Local vs Docker setup, config/data locations, ledger registry.
- Testing patterns: service mocks (`internal/service/testhelper_test.go`), store integration tests, API handler tests, SPA Vitest tests.
- SPA toolchain: Vite dev server, API proxy, Biome, route generation.
- Building the embedded SPA; deploy scripts (launchd, systemd).

**docs/http-api.md**
- Table: method, path, handler, service method, notes.
- Error-response shape and service-error → HTTP-status mapping.
- Shared query parameters.

**docs/recipes/*.md** — uniform shape:
- When to use this recipe.
- Files to touch, in order.
- Worked example referencing a real past commit/PR.
- Conventions (e.g. the cmd flag-handling pattern in `add-cli-command.md`).
- Test checklist.

**docs/decisions.md**
- ~15–30 entries, each: Decision / Why / Where (code) / Source (link into `history/`).

## Writing Conventions

1. Point to code, don't paste it. Reference `path/file.go` plus symbol name; no line numbers. Short snippets only where a pattern must be shown.
2. Every doc starts with a 2–3 line "Read this when…" header.
3. One topic per file; each fact has one home, others link to it.
4. Plain markdown only: headings, tables, ASCII diagrams. No HTML, no Mermaid.
5. Every file path referenced in docs must exist; verified by a script that extracts backticked paths and checks them.
6. Package godoc: each Go package under `internal/` and `cmd/` gets a 1–3 sentence package comment (in `doc.go` or the main file) stating what it owns and which doc explains it.

## Execution

- Branch: `docs/codebase-docs`.
- Order: (1) remove `docs/superpowers` from `.gitignore`, move all existing docs to `docs/history/` and commit them, preserving their subfolder structure (this spec and its plan move too, to `docs/history/superpowers/`; future specs/plans go there as well); (2) `architecture.md`, `domain.md`; (3) `development.md`, `http-api.md`; (4) recipes; (5) `decisions.md`; (6) package comments; (7) rewrite `AGENTS.md`, `CLAUDE.md`, `README.md`, `spa/README.md`; (8) path-check script.
- Subagent-driven execution, roughly one subagent per doc; each doc reviewed for accuracy against code.
- Only code change is comments; verify with `go build ./... && go vet ./...`.

## Out of Scope

- End-user guide.
- Generated API docs (OpenAPI).
- Any behavior change in code.
