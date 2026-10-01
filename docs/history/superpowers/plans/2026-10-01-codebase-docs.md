# Codebase Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write a layered contributor doc set (entry point → topic docs → recipes → decisions → package comments) so a returning developer or an AI agent can understand kea quickly and add features the way the codebase already does.

**Architecture:** `AGENTS.md` is a short entry point with hard rules and a "where to look" table. Topic docs under `docs/` each answer one kind of question and point at code by path + symbol instead of pasting it. All pre-existing design/plan files move to `docs/history/` as an archive. A shell script verifies that every path the docs mention exists.

**Tech Stack:** Markdown, Bash (`scripts/check-docs.sh`), Go package comments. Codebase: Go (Cobra, huh, pterm, bubbletea, SQLite, golang-migrate, net/http) + React SPA (Vite, TanStack Router/Query, Tailwind, shadcn/ui, Biome, Vitest).

**Spec:** `docs/history/superpowers/specs/2026-10-01-codebase-docs-design.md` (at `docs/superpowers/specs/...` until Task 1 moves it).

---

## Writing Conventions (apply to EVERY doc task)

Every doc written in this plan must follow these rules. Reviewers check them.

1. **Point to code, don't paste it.** Reference code as a backticked repo-relative path plus the symbol, e.g. `internal/service/transaction_ops.go` (`TransactionService.CreateTransaction`). Never use line numbers. Snippets only where a pattern must be shown, max ~10 lines.
2. **Header.** Every doc starts with an H1 title, then a blockquote of 2–3 lines: `> **Read this when:** …` and `> **Related:** …` (links to sibling docs).
3. **One home per fact.** If a fact belongs to another doc (see table below), link to it instead of re-explaining.
4. **Plain markdown only:** headings, lists, tables, fenced ASCII diagrams. No HTML, no Mermaid, no emoji.
5. **Verified facts only.** Read the code before writing about it. Every backticked path must exist (`scripts/check-docs.sh` checks it). Every named symbol must exist (`grep -rn "func (.*) SymbolName\|func SymbolName\|type SymbolName" <dir>`). If code and an old plan disagree, the code wins.
6. **English only.** Concise, present tense, no marketing language.
7. **Paths in backticks must be repo-relative and start with one of:** `cmd/`, `internal/`, `ui/`, `migrations/`, `spa/`, `scripts/`, `docker/`, `docs/`. Root files (`go.mod`, `Makefile`, `docker-compose.yml`) are written as-is.

**Fact ownership** (where each topic lives):

| Topic | Home |
|---|---|
| Layers, request flow, package map, startup, SPA embedding | `docs/architecture.md` |
| Accounting rules, tx types, opening balances, reconcile, reports, currencies | `docs/domain.md` |
| Setup, data dirs, testing patterns, SPA toolchain, deploy scripts | `docs/development.md` |
| HTTP endpoints, error mapping, query params | `docs/http-api.md` |
| Step-by-step "how to add X" | `docs/recipes/*.md` |
| Why past choices were made | `docs/decisions.md` |
| Doc index | `docs/README.md` |
| Hard rules + where to look | `AGENTS.md` |

---

## File Map

| Action | Path |
|---|---|
| Move | `docs/2026-*.md`, `docs/web-layer/`, `docs/superpowers/` → `docs/history/` (same sub-structure) |
| Create | `scripts/check-docs.sh` |
| Create | `docs/architecture.md`, `docs/domain.md`, `docs/development.md`, `docs/http-api.md`, `docs/decisions.md`, `docs/README.md` |
| Create | `docs/recipes/add-migration.md`, `add-service-method.md`, `add-api-endpoint.md`, `add-cli-command.md`, `add-spa-page.md` |
| Create/Modify | package comments in every Go package (see Task 13) |
| Rewrite | `AGENTS.md`, `CLAUDE.md`, `spa/README.md` |
| Create | `README.md` |

---

### Task 1: Move existing docs into `docs/history/`

**Files:**
- Move: `docs/2026-06-17-income-statement-layout-tweaks-design.md`, `docs/2026-06-17-income-statement-layout-tweaks-plan.md`, `docs/2026-06-28-webui-dashboard-design.md`, `docs/2026-06-28-webui-dashboard-plan.md` → `docs/history/`
- Move: `docs/web-layer/` → `docs/history/web-layer/`
- Move: `docs/superpowers/` → `docs/history/superpowers/` (includes this plan and its spec; some files are tracked, some were previously gitignored and are untracked)

- [ ] **Step 1: Record counts before moving**

```bash
find docs -type f -name '*.md' | wc -l
```
Write down the number (N). Expected: ~165.

- [ ] **Step 2: Move with plain `mv` (handles tracked and untracked alike), then stage**

```bash
mkdir -p docs/history
mv docs/2026-*.md docs/history/
mv docs/web-layer docs/history/web-layer
mv docs/superpowers docs/history/superpowers
git add -A docs
```
Git detects renames for tracked files automatically.

- [ ] **Step 3: Verify nothing was lost**

```bash
find docs -type f -name '*.md' | wc -l
ls docs
git status --short docs | grep -v '^R' | head
```
Expected: same count N as Step 1; `ls docs` prints only `history`; the last command lists only `A` (newly added, previously ignored) lines, no `D` lines.

- [ ] **Step 4: Commit**

```bash
git commit -m "docs: move design and plan history into docs/history

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Doc path checker script

**Files:**
- Create: `scripts/check-docs.sh`

- [ ] **Step 1: Write the script**

```bash
#!/usr/bin/env bash
# check-docs.sh verifies that every repo path and relative link referenced in
# the contributor docs exists. Run from anywhere: scripts/check-docs.sh
set -euo pipefail
cd "$(dirname "$0")/.."

shopt -s nullglob
docs=(AGENTS.md README.md spa/README.md docs/*.md docs/recipes/*.md)
fail=0

for doc in "${docs[@]}"; do
  [ -f "$doc" ] || continue
  dir=$(dirname "$doc")

  # Backticked repo paths such as `internal/service/errors.go` or `migrations/0011_*`.
  while IFS= read -r p; do
    [ -z "$p" ] && continue
    p=${p%%:*}                      # drop a ":line" or ":Symbol" suffix
    if ! compgen -G "$p" >/dev/null; then
      echo "$doc: missing path \`$p\`"
      fail=1
    fi
  done < <(grep -oE '`(cmd|internal|ui|migrations|spa|scripts|docker|docs)/[^` ]*`' "$doc" | tr -d '`' | sort -u || true)

  # Relative markdown links such as [domain](domain.md) or [x](../cmd/root.go).
  while IFS= read -r l; do
    l=${l%%#*}
    [ -z "$l" ] && continue
    case "$l" in http://*|https://*|mailto:*) continue ;; esac
    if [ ! -e "$dir/$l" ]; then
      echo "$doc: broken link ($l)"
      fail=1
    fi
  done < <(grep -oE '\]\([^) ]+\)' "$doc" | sed -E 's/^\]\(//; s/\)$//' | sort -u || true)
done

if [ "$fail" -eq 0 ]; then
  echo "check-docs: all referenced paths exist"
fi
exit "$fail"
```

- [ ] **Step 2: Make it executable and prove it catches a bad path**

```bash
chmod +x scripts/check-docs.sh
mkdir -p docs/recipes
printf '# t\n\n`internal/nope/missing.go` and `internal/service/errors.go` and [x](nope.md)\n' > docs/zz-selftest.md
scripts/check-docs.sh; echo "exit=$?"
```
Expected output:
```
docs/zz-selftest.md: missing path `internal/nope/missing.go`
docs/zz-selftest.md: broken link (nope.md)
exit=1
```

- [ ] **Step 3: Prove it passes on good input**

```bash
printf '# t\n\n`internal/service/errors.go` `migrations/0011_*` [h](history)\n' > docs/zz-selftest.md
scripts/check-docs.sh; echo "exit=$?"
rm docs/zz-selftest.md
```
Expected: `check-docs: all referenced paths exist` and `exit=0`.

- [ ] **Step 4: Commit**

```bash
git add scripts/check-docs.sh
git commit -m "docs: add script that verifies paths referenced in docs

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: `docs/architecture.md`

**Files:**
- Create: `docs/architecture.md`

**Read before writing:** `cmd/kea/main.go`, `cmd/root.go`, `cmd/add.go`, `cmd/add_actions.go`, `cmd/serve.go`, `internal/app/app.go`, `internal/service/service.go`, `internal/repository/interfaces.go`, `internal/store/sqlite.go`, `internal/api/server.go`, `internal/api/router.go`, `internal/api/middleware.go`, `internal/api/handler.go`, `internal/api/transactions_write.go`, `internal/api/spa.go`, `internal/web/embed.go`, `internal/ledger/registry.go`, `internal/backup/backup.go`, `internal/config/config.go`, `migrations/embed.go`, `ui/` (skim). Use `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` to get real import edges.

- [ ] **Step 1: Write the doc with exactly these sections**

```markdown
# Architecture

> **Read this when:** you need the big picture — which layer owns what, how a CLI command or HTTP request reaches the database, or how the binary starts.
> **Related:** [domain.md](domain.md) · [http-api.md](http-api.md) · [recipes/](recipes/)

## Layers
<ASCII diagram: cmd/ + ui/ (CLI/TUI) and internal/api (HTTP) on top → internal/service → internal/repository (interfaces) → internal/store (SQLite). internal/app wires them; internal/model shared by all; migrations/ embedded into store. 1 short paragraph on the dependency direction rule.>

## Package map
<Table: Package | Owns | Must not import. One row per package: cmd, cmd/kea, cmd/account, cmd/ledger, cmd/transaction, ui, ui/prompts, ui/views, ui/reconcile, internal/api, internal/app, internal/service, internal/repository, internal/store, internal/model, internal/config, internal/ledger, internal/backup, internal/utils, internal/web, migrations. Derive "must not import" from the actual import graph — state the rule the code already follows (e.g. model imports nothing internal; service never imports store).>

## The service facade
<service.Service, svc.Account()/Transaction()/Config(); where report and reconcile methods live; why (point to decisions.md if relevant).>

## Request flow: CLI (`kea add`)
<Numbered trace, each step "file (Symbol) — what happens": Cobra command → runner/flags → ui/prompts wizard → model input struct → service method → validation → ExecTx → store methods → result rendered by ui/views.>

## Request flow: HTTP (`POST /api/transactions`)
<Numbered trace: server/router → middleware chain (list each middleware) → handler → request decoding/params → service → store → JSON response and error mapping (link http-api.md for the table).>

## Transactions and context
<ExecTx / TransactionManager, DBTX interface over *sql.DB and *sql.Tx, context threading rule.>

## Startup sequence
<Numbered: main → root PersistentPreRun (config via viper, ledger registry, active ledger, backup, open DB + migrations, migrateLegacySysAcc) → command. Note `kea serve` differences (registry watcher, ledger switching at runtime).>

## Multiple ledgers
<What a ledger is (named SQLite DB), registry file, active selection, how serve swaps connections. Code lives in: …>

## How `kea serve` ships the SPA
<spa build output → internal/web/dist → go:embed → internal/api/spa.go fallback routing. Link development.md for the build command.>
```
Replace each `<…>` with real content from the code. No `<…>` may remain.

- [ ] **Step 2: Verify**

```bash
scripts/check-docs.sh
grep -n '<' docs/architecture.md | grep -v '^.*`' || true
```
Expected: checker passes; second command prints no leftover placeholders. Spot-check 5 symbols named in the doc with `grep -rn`.

- [ ] **Step 3: Commit**

```bash
git add docs/architecture.md
git commit -m "docs: add architecture overview

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `docs/domain.md`

**Files:**
- Create: `docs/domain.md`

**Read before writing:** `internal/model/*.go`, `internal/service/account_validation.go`, `internal/service/account_ops.go`, `internal/service/transaction_validation.go`, `internal/service/transaction_classifier.go`, `internal/service/transaction_ops.go`, `internal/service/reconcile_ops.go`, `internal/service/report_service.go`, `internal/service/errors.go`, `internal/utils/amount.go`, `migrations/*.up.sql`, `cmd/root.go` (`migrateLegacySysAcc`), `SKILL.md` (Core Concepts — for wording; code wins on conflict). History for intent: `docs/history/superpowers/plans/2026-04-01-per-currency-opening-balances.md`, `2026-04-16-reconciliation.md`, `2026-04-17-last-reconciled-balance.md`, `2026-04-23-stored-transaction-type.md`, `2026-05-08-fix-cross-currency-report-totals.md`, and `docs/history/superpowers/specs/2026-06-25-regular-transaction-attribute-design.md`.

- [ ] **Step 1: Write the doc with exactly these sections**

```markdown
# Domain Model

> **Read this when:** you touch money, accounts, transactions, reconciliation, or reports — or need to know which invariant a change might break.
> **Related:** [architecture.md](architecture.md) · [decisions.md](decisions.md)

## Double-entry in one paragraph
<Plain-language explanation + a worked example: buying lunch for 12.50 → splits table with account, amount in cents, sign.>

## Amounts
<int64 cents, sign conventions per account type, FormatAmount/ParseAmount behavior incl. trailing-zero trimming. Code lives in: …>

## Accounts
<Types A/L/C/R/E table (code, name, root, normal sign); colon paths; parent/leaf; only leaves hold splits; hidden accounts; currency per account; validation rules (circular parent etc.). Code lives in: …>

## Transactions and splits
<Fields, sum-to-zero (ValidateSplitsBalance), external_id, status/cleared. Code lives in: …>

## Transaction types
<List every type the code defines, how the classifier determines it, that it is stored (not computed at read time), backfill migrations involved. Code lives in: …>

## Regular attribute
<What Regular means, which types it applies to, NULL/true/false invariant and its DB CHECK, error values. Code lives in: …>

## Opening balances
<Per-currency system accounts, name helpers, split direction for asset vs liability, legacy rename at startup, cannot be deleted. Code lives in: …>

## Reconciliation
<Lifecycle: unreconciled → reconciled; per-split reconciled flag; last reconciled balance per account; balance-mismatch gate; immutability + ErrReconciled. Code lives in: …>

## Reports
<Balance sheet, income statement, net worth series, income/expense breakdown, regular/irregular subtotals; how multiple currencies are kept separate (never summed across currencies). Code lives in: …>

## Errors you will meet
<Table: sentinel error | meaning | where returned. From internal/service/errors.go and internal/repository/errors.go. Note errors.Is usage.>
```
Replace every `<…>`; none may remain.

- [ ] **Step 2: Verify**

```bash
scripts/check-docs.sh
```
Expected: pass. Spot-check: every error in the table exists in `internal/service/errors.go` or `internal/repository/errors.go`; every transaction type listed matches the constants in `internal/model`.

- [ ] **Step 3: Commit**

```bash
git add docs/domain.md
git commit -m "docs: add domain model reference

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: `docs/development.md`

**Files:**
- Create: `docs/development.md`

**Read before writing:** `Makefile`, `docker-compose.yml`, `docker/app.Dockerfile`, `docker/spa.Dockerfile`, `.env.example`, current `AGENTS.md` (Commands + Docker sections — move that content here), `internal/config/config.go`, `cmd/root.go`, `cmd/save_config.go`, `internal/ledger/registry.go`, `internal/backup/*.go`, `internal/service/testhelper_test.go`, `internal/store/*_test.go` (skim for setup helper), `internal/api/testhelper_test.go`, `spa/package.json`, `spa/vite.config.ts`, `spa/biome.json`, `spa/src/test/` (skim), `scripts/*`.

- [ ] **Step 1: Write the doc with exactly these sections**

```markdown
# Development

> **Read this when:** you set up the project, run or test it, work on the SPA, or deploy `kea serve`.
> **Related:** [architecture.md](architecture.md) · [recipes/](recipes/)

## Prerequisites
<Go version from go.mod, CGO/SQLite note if applicable, Node version for spa.>

## Run locally
<make build / make run / go run ./cmd/kea <args>; first-run bootstrap of the default ledger.>

## Run with Docker
<Moved from AGENTS.md: containers, ports, ./data bind mount, KEA_DATA_DIR, the root-user test limitation.>

## Config and data locations
<Config dir, config.yaml keys that matter, ledgers.yaml, DB files, backups dir; which code reads each.>

## Testing
### Service tests (mocks)
<testhelper_test.go mocks, error maps, call recorders, factory helpers; a 6–10 line example of the arrange/act/assert style copied from a real test.>
### Store tests (real SQLite)
<How store tests create a DB and run migrations; helper name and file.>
### API tests
<API test helper, how handlers are exercised (httptest).>
### cmd tests
<How cmd tests are structured (runner with injected service?) — verify from cmd/*_test.go.>
### SPA tests
<Vitest + Testing Library, location, how API calls are mocked, command.>
### Commands
<go test ./..., single package, single test, -v; npm test; lint.>

## SPA toolchain
<npm scripts, Vite dev server and API proxy target (and the Docker override), Biome lint/format, TanStack Router file-based routes and routeTree.gen.ts regeneration.>

## Building the embedded SPA
<Exact command(s) that produce internal/web/dist and when to commit the bundle (check git log for "build(spa): refresh embedded bundle" commits).>

## Deploying `kea serve`
<One short subsection each for scripts/install-launchd.sh, scripts/install-systemd.sh, scripts/install-systemd-user.sh: what it installs, how to run, where logs go.>

## Docs maintenance
<Run scripts/check-docs.sh after editing docs; update the matching doc in the same commit as a pattern/endpoint/rule change; new specs and plans go to docs/history/superpowers/specs and docs/history/superpowers/plans.>
```
Replace every `<…>`; none may remain.

- [ ] **Step 2: Verify**

```bash
scripts/check-docs.sh
```
Expected: pass. Run every command shown in "Commands" that is cheap (`go test ./internal/utils/...`) to confirm it works as written.

- [ ] **Step 3: Commit**

```bash
git add docs/development.md
git commit -m "docs: add development guide

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `docs/http-api.md`

**Files:**
- Create: `docs/http-api.md`

**Read before writing:** `internal/api/router.go` (source of truth for routes), every handler file in `internal/api/`, `internal/api/errors.go`, `internal/api/params.go`, `internal/api/middleware.go`, `internal/api/config.go`.

- [ ] **Step 1: List all routes mechanically so none are missed**

```bash
grep -nE '"(GET|POST|PUT|PATCH|DELETE) /' internal/api/router.go
```
Every line of output must become a row in the endpoint table.

- [ ] **Step 2: Write the doc with exactly these sections**

```markdown
# HTTP API

> **Read this when:** you call, change, or add an endpoint served by `kea serve`, or wire the SPA to the backend.
> **Related:** [recipes/add-api-endpoint.md](recipes/add-api-endpoint.md) · [architecture.md](architecture.md)

## Conventions
<Base path, JSON shapes, amount representation on the wire, date format, ledger selection per request (header/param?), CORS/auth if any.>

## Endpoints
<One table per resource group (Health, Ledgers, Accounts, Transactions, Reconcile, Reports, Config): Method | Path | Handler (file + func) | Service call | Notes.>

## Query parameters
<Shared params from params.go: pagination, date ranges, filters; parsing rules and defaults.>

## Errors
<Error response JSON shape; table: service/repository error → HTTP status → error code string; special cases like 409 balance_mismatch with its extra fields.>
```
Replace every `<…>`; none may remain.

- [ ] **Step 3: Verify**

```bash
scripts/check-docs.sh
grep -cE '"(GET|POST|PUT|PATCH|DELETE) /' internal/api/router.go
grep -cE '^\| (GET|POST|PUT|PATCH|DELETE) ' docs/http-api.md
```
Expected: checker passes; the two counts are equal.

- [ ] **Step 4: Commit**

```bash
git add docs/http-api.md
git commit -m "docs: add HTTP API reference

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Recipe tasks (Tasks 7–11): shared shape

Every recipe file uses exactly this skeleton:

```markdown
# Recipe: <title>

> **Read this when:** <one line>
> **Related:** <links>

## When to use
<1–3 bullets; also when NOT to use (link the better recipe).>

## Steps
<Numbered. Each step: file(s) to touch → what to add → the existing example to imitate (path + symbol).>

## Worked example
<The listed commits, in order, one line each: `<sha>` — what it did and why it is the model to copy. Readers run `git show <sha>`.>

## Conventions
<Bullets: naming, error handling, wrapping, context, file placement, test file naming.>

## Checklist
- [ ] <test 1 to add>
- [ ] <…>
- [ ] Update the matching doc (http-api.md / domain.md / …) in the same commit
- [ ] `go test ./...` (and `npm test` in spa/ for SPA work) passes
```

Each recipe task: read the listed commits with `git show <sha>` and the listed files, write the recipe, run `scripts/check-docs.sh` (expected: pass), confirm no `<…>` remains, commit with `git add docs/recipes/<file> && git commit -m "docs: add <name> recipe" ` plus the Co-Authored-By trailer.

### Task 7: `docs/recipes/add-migration.md`

**Files:** Create `docs/recipes/add-migration.md`

**Worked example commits:** `5bc4e65` (add regular column with CHECK), `b613312` (migration round-trip + CHECK tests), `277ad17` (fix CHECK NULL handling — the lesson: test NULL cases). Also look at `0006_backfill_transaction_type` as a backfill example.

**Read:** `migrations/embed.go`, `migrations/0011_*`, the store code that runs migrations (find with `grep -rn "migrate\." internal/store cmd internal/app`), the migration tests in `internal/store`.

**Must cover:** numbering scheme `NNNN_name.{up,down}.sql`; down migrations required; SQLite limits on ALTER (and how existing migrations work around them); data backfills as separate migrations; how migrations run at startup and in tests; updating `internal/model` + store scan/insert code afterwards.

- [ ] **Step 1:** Read the commits and files above.
- [ ] **Step 2:** Write the recipe using the shared shape.
- [ ] **Step 3:** `scripts/check-docs.sh` → pass; no `<…>` left.
- [ ] **Step 4:** Commit: `docs: add add-migration recipe`.

### Task 8: `docs/recipes/add-service-method.md`

**Files:** Create `docs/recipes/add-service-method.md`

**Worked example commits:** `e2275aa` (model field), `a8a87aa` (new sentinel errors), `14ecedc` (validation invariant), `ae1e9d3` (defaulting through create/update), `8303a63` (store persists column), `88e8472` (propagate to list item).

**Read:** `internal/service/service.go`, `internal/service/errors.go`, `internal/service/transaction_ops.go`, `internal/service/transaction_validation.go`, `internal/service/testhelper_test.go`, `internal/repository/interfaces.go`, `internal/model/input.go`.

**Must cover:** where to put the method (`*_ops.go` vs `*_validation.go` vs `report_service.go`); input structs in `internal/model/input.go`; adding a repository method (interface → store implementation → mock in `testhelper_test.go`); using `ExecTx` for multi-step writes; sentinel errors and `%w` wrapping; `ErrReconciled` guard for anything that mutates; white-box test style.

- [ ] **Step 1:** Read the commits and files above.
- [ ] **Step 2:** Write the recipe using the shared shape.
- [ ] **Step 3:** `scripts/check-docs.sh` → pass; no `<…>` left.
- [ ] **Step 4:** Commit: `docs: add add-service-method recipe`.

### Task 9: `docs/recipes/add-api-endpoint.md`

**Files:** Create `docs/recipes/add-api-endpoint.md`

**Worked example commits:** `61ca0f9` (custom error type for a domain gate), `4dbea42` (GET endpoint), `b2fa44f` (POST preview), `c879a26` (POST commit), `e002496` (tightened tests), `126a7a0` (new query param), `8f9140f` (round-trip a new field through create/update).

**Read:** `internal/api/router.go`, `internal/api/handler.go`, `internal/api/errors.go`, `internal/api/params.go`, `internal/api/reconcile.go`, `internal/api/reconcile_test.go`, `internal/api/testhelper_test.go`.

**Must cover:** registering the route; handler signature and how it gets the service for the active ledger; decoding/validating input; params helpers; error mapping (add new mapping in `errors.go` when introducing a new service error); response shape; tests with the API test helper; then the SPA side (link `add-spa-page.md`) and updating `docs/http-api.md`.

- [ ] **Step 1:** Read the commits and files above.
- [ ] **Step 2:** Write the recipe using the shared shape.
- [ ] **Step 3:** `scripts/check-docs.sh` → pass; no `<…>` left.
- [ ] **Step 4:** Commit: `docs: add add-api-endpoint recipe`.

### Task 10: `docs/recipes/add-cli-command.md`

**Files:** Create `docs/recipes/add-cli-command.md`

**Worked example commits:** `970195a` (new `account search` command), `36b7e8b` (flag shorthand fix), `43f1077` (new flag + interactive prompt on `kea add`), `e8f2331` (list filter flag + new column).

**Read:** `cmd/root.go`, `cmd/account/account.go`, `cmd/account/search.go`, `cmd/account/list.go`, `cmd/account/delete.go`, `cmd/add.go`, `cmd/add_actions.go`, `cmd/add_types.go`, `ui/prompts/` (skim), `ui/views/` (skim), `ui/views/json.go`, one cmd test file.

**Must cover:** command registration (subcommand groups `cmd/account`, `cmd/transaction`, `cmd/ledger`); the `*_types.go` / `*_actions.go` split; how a runner gets the service; **the three flag-handling patterns**, verbatim rules:
- Pattern A — store the whole flags struct in the runner (`runner { svc, flags *xxxFlags }`): 3+ flags used in several places in `Run()`. Examples: `account list`, `transaction list`.
- Pattern B — copy individual fields into the runner (`runner { svc, json bool, yes bool }`): 1–2 flags. Examples: `account delete`, `transaction show`, `transaction clear`, `info`.
- Pattern C — pass flags directly to `Run(flags, cmd)`: the command needs `cmd.Flags().Changed()` to choose interactive vs flag mode. Examples: `add`, `account create`.
- Never mix patterns within one command.

Also: interactive prompts in `ui/prompts` vs rendering in `ui/views`; `--json` output via `ui/views/json*.go`; non-interactive flag mode for agents (and updating `SKILL.md`).

- [ ] **Step 1:** Read the commits and files above; verify each named example still uses the stated pattern.
- [ ] **Step 2:** Write the recipe using the shared shape.
- [ ] **Step 3:** `scripts/check-docs.sh` → pass; no `<…>` left.
- [ ] **Step 4:** Commit: `docs: add add-cli-command recipe`.

### Task 11: `docs/recipes/add-spa-page.md`

**Files:** Create `docs/recipes/add-spa-page.md`

**Worked example commits:** `be472c0` (API client + response types), `be6a4b9` (carry error details through ApiError), `ea195b4` / `fc47d88` (components), `7f80bd3` (route + workspace component), `c7fc50e` (chooser route + test), `9e4ee0b` (sidebar link), `1d34109` (per-ledger filter memory). Also `cda1cdd` (refresh embedded bundle).

**Read:** `spa/src/main.tsx`, `spa/src/routes/__root.tsx`, `spa/src/routes/reconcile.tsx`, `spa/src/routes/reconcile.index.tsx`, `spa/src/routes/reconcile.$id.tsx`, `spa/src/lib/` (API client, query keys, formatting, filter memory), `spa/src/components/` (layout/sidebar, `ui/` shadcn primitives), `spa/src/test/` (one test), `spa/package.json`.

**Must cover:** file-based route naming (`a.$id.tsx`, `a.index.tsx`, layout routes) and `routeTree.gen.ts` regeneration; API client functions + TypeScript types mirroring Go JSON; TanStack Query keys and invalidation after mutations; ledger-switch handling; amount formatting helpers (cents); shadcn/ui primitives location; sidebar entry; per-ledger filter memory; tests; refreshing the embedded bundle.

- [ ] **Step 1:** Read the commits and files above.
- [ ] **Step 2:** Write the recipe using the shared shape.
- [ ] **Step 3:** `scripts/check-docs.sh` → pass; no `<…>` left.
- [ ] **Step 4:** Commit: `docs: add add-spa-page recipe`.

---

### Task 12: `docs/decisions.md`

**Files:**
- Create: `docs/decisions.md`

**Read before writing:** every file in `docs/history/superpowers/specs/`, `docs/history/web-layer/`, and `docs/history/*.md`; skim `docs/history/superpowers/plans/` titles (`ls`) and open any whose title suggests an architectural choice (e.g. `2026-04-26-fix-store-error-boundary.md`, `2026-04-28-context-threading.md`, `2026-05-13-atomic-create-account-with-balance.md`, `2026-05-14-issue-79-input-structs.md`, `2026-05-20-split-type-trust-boundary.md`, `2026-05-23-chunk-in-clauses.md`, `2026-06-05-registry-mutex-locking.md`, `2026-06-09-runtime-state-off-config.md`, `2026-04-15-multi-ledger.md`, `2026-04-14-db-backup.md`, `2026-03-24-json-output.md`). Confirm each decision still holds in current code before writing it.

- [ ] **Step 1: Write the doc**

```markdown
# Decisions

> **Read this when:** you wonder why something is built the way it is, or before reversing an existing choice.
> **Related:** [architecture.md](architecture.md) · [domain.md](domain.md) · [history/](history/)

Entries are grouped by area. Each one is still true in the current code; superseded choices are omitted.

## <Area, e.g. Storage>

### <Short decision title>
- **Decision:** <one sentence>
- **Why:** <1–3 sentences>
- **Where:** `<path>` (`<Symbol>`)
- **Source:** [<history file name>](history/<path>)
```
Write 15–30 entries across areas: Storage, Domain, Service layer, CLI, HTTP API, SPA, Operations. Every entry must have a Source link that resolves.

- [ ] **Step 2: Verify**

```bash
scripts/check-docs.sh
grep -c '^### ' docs/decisions.md
```
Expected: checker passes (every Source link resolves); entry count between 15 and 30.

- [ ] **Step 3: Commit**

```bash
git add docs/decisions.md
git commit -m "docs: add decision log distilled from design history

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Package comments

**Files:** one package comment per Go package. Put it in a new `doc.go` for packages with several files; for single-file packages put it directly above `package` in that file. `internal/web/embed.go` already has one — leave it.

| Package | File | Draft comment (verify against code, adjust if inaccurate) |
|---|---|---|
| `cmd/kea` | `cmd/kea/main.go` | `// Command kea is the entry point of the kea CLI, TUI, and HTTP server. See docs/architecture.md.` |
| `cmd` | `cmd/doc.go` | `// Package cmd defines the root Cobra command and top-level kea subcommands (add, info, reconcile, report, serve). It loads config, opens the active ledger, and calls the service layer. See docs/recipes/add-cli-command.md.` |
| `cmd/account` | `cmd/account/doc.go` | `// Package account implements the "kea account" subcommands (create, edit, delete, list, search). See docs/recipes/add-cli-command.md.` |
| `cmd/transaction` | `cmd/transaction/doc.go` | `// Package transaction implements the "kea transaction" subcommands (list, show, edit, clear, delete). See docs/recipes/add-cli-command.md.` |
| `cmd/ledger` | `cmd/ledger/doc.go` | `// Package ledger implements the "kea ledger" subcommands (add, list, switch, remove) that manage named ledger databases.` |
| `ui` | `ui/doc.go` | `// Package ui holds shared terminal styles and layout helpers for the CLI views and prompts.` |
| `ui/prompts` | `ui/prompts/doc.go` | `// Package prompts contains the interactive huh forms used by CLI commands to collect input.` |
| `ui/views` | `ui/views/doc.go` | `// Package views renders command output as pterm tables and text, or as JSON when --json is set.` |
| `ui/reconcile` | `ui/reconcile/doc.go` | `// Package reconcile implements the interactive terminal UI for reconciling an account.` |
| `internal/api` | `internal/api/doc.go` | `// Package api implements the HTTP server behind "kea serve": routing, middleware, JSON handlers, and error mapping. See docs/http-api.md.` |
| `internal/app` | `internal/app/app.go` | `// Package app wires a store and the service layer together for one ledger database.` |
| `internal/backup` | `internal/backup/doc.go` | `// Package backup copies the ledger database before startup so a bad run can be rolled back.` |
| `internal/config` | `internal/config/config.go` | `// Package config defines kea's user configuration and its defaults.` |
| `internal/ledger` | `internal/ledger/registry.go` | `// Package ledger manages the registry of named ledger databases and which one is active.` |
| `internal/model` | `internal/model/doc.go` | `// Package model defines kea's domain types. It contains no business logic and imports no other kea package. See docs/domain.md.` |
| `internal/repository` | `internal/repository/doc.go` | `// Package repository declares the storage interfaces the service layer depends on and the errors they return.` |
| `internal/service` | `internal/service/doc.go` | `// Package service implements kea's business rules: accounts, transactions, reconciliation, and reports. See docs/domain.md and docs/recipes/add-service-method.md.` |
| `internal/store` | `internal/store/doc.go` | `// Package store implements the repository interfaces on SQLite. All methods take a context and work over either *sql.DB or *sql.Tx.` |
| `internal/utils` | `internal/utils/doc.go` | `// Package utils holds pure helpers such as amount formatting and parsing (amounts are int64 cents).` |
| `migrations` | `migrations/embed.go` | `// Package migrations embeds kea's SQL schema migrations for golang-migrate. See docs/recipes/add-migration.md.` |

`doc.go` content format (example for `internal/service`):

```go
// Package service implements kea's business rules: accounts, transactions,
// reconciliation, and reports. See docs/domain.md and
// docs/recipes/add-service-method.md.
package service
```

- [ ] **Step 1: Check for existing comments before writing**

```bash
for f in cmd/kea/main.go internal/app/app.go internal/config/config.go internal/ledger/registry.go migrations/embed.go; do echo "== $f"; sed -n '1,8p' "$f"; done
```
If a file already has a comment directly above `package`, merge rather than duplicate.

- [ ] **Step 2: Verify each draft against its package, then add all comments.** Read each package's exported API (`go doc ./internal/<pkg>`) and correct any draft that misstates what the package does (e.g. confirm `ui/reconcile` is a TUI, `internal/backup` timing, `ui/views` uses pterm). Wrap at ~80 columns.

- [ ] **Step 3: Verify build and docs**

```bash
gofmt -l cmd internal ui migrations
go build ./... && go vet ./...
for p in $(go list ./cmd/... ./internal/... ./ui/... ./migrations/...); do go doc "$p" | head -1; done
```
Expected: `gofmt -l` prints nothing; build and vet succeed; every package prints a non-empty synopsis line starting with `package` and followed by the comment.

- [ ] **Step 4: Commit**

```bash
git add cmd internal ui migrations
git commit -m "docs: add package comments to every Go package

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: `docs/README.md` (docs index)

**Files:**
- Create: `docs/README.md`

- [ ] **Step 1: Write the index**

```markdown
# kea Docs

> **Read this when:** you are new to (or returning to) the codebase and need to find the right doc.
> **Related:** [../AGENTS.md](../AGENTS.md) · [../SKILL.md](../SKILL.md)

## Start here
1. [architecture.md](architecture.md) — layers, request flow, package map, startup.
2. [domain.md](domain.md) — the accounting rules every change must respect.
3. [development.md](development.md) — setup, running, testing, deploying.

## If you want to…

| I want to… | Read |
|---|---|
| Understand how a command or request reaches the DB | [architecture.md](architecture.md) |
| Know what a transaction type / reconcile / opening balance is | [domain.md](domain.md) |
| Add or change a database column or table | [recipes/add-migration.md](recipes/add-migration.md) |
| Add business logic or a repository method | [recipes/add-service-method.md](recipes/add-service-method.md) |
| Add or change an HTTP endpoint | [recipes/add-api-endpoint.md](recipes/add-api-endpoint.md) · [http-api.md](http-api.md) |
| Add a CLI command or flag | [recipes/add-cli-command.md](recipes/add-cli-command.md) |
| Add a web UI page | [recipes/add-spa-page.md](recipes/add-spa-page.md) |
| Know why something was built this way | [decisions.md](decisions.md) |
| Read the original design of a past feature | [history/](history/) |
| Operate the kea CLI as an agent | [../SKILL.md](../SKILL.md) |

## Layout
- `docs/*.md` — current reference; kept in sync with code.
- `docs/recipes/` — step-by-step guides built from real past commits.
- `docs/history/` — archived specs and plans. Point-in-time records; they may be out of date. New specs and plans go to `docs/history/superpowers/specs/` and `docs/history/superpowers/plans/`.

## Keeping docs current
Run `scripts/check-docs.sh` after editing docs. When a change alters a pattern, endpoint, or domain rule, update the matching doc in the same commit.
```

- [ ] **Step 2: Verify**

```bash
scripts/check-docs.sh
```
Expected: pass.

- [ ] **Step 3: Commit**

```bash
git add docs/README.md
git commit -m "docs: add docs index

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Rewrite `AGENTS.md` and `CLAUDE.md`

**Files:**
- Rewrite: `AGENTS.md`
- Rewrite: `CLAUDE.md`

- [ ] **Step 1: Write `AGENTS.md`** (target 100–150 lines). Use this content, verifying each command and rule against the code; the Docker detail now lives in `docs/development.md`.

```markdown
# AGENTS.md

Guidance for AI coding agents and contributors working in this repository.

kea is a personal double-entry accounting tool: a Go CLI/TUI plus an HTTP API (`kea serve`) with an embedded React SPA, storing each ledger in a local SQLite database.

## Language

All code, comments, test names, commit messages, and documentation must be written in English.

## Commands

```bash
make build                                   # builds ./kea_test
make run                                     # go run ./cmd/kea
go test ./...                                # all Go tests
go test ./internal/service/ -run TestName    # one test
go build ./... && go vet ./...               # compile + vet
scripts/check-docs.sh                        # verify paths referenced in docs
docker compose up -d                         # app on :8080, spa dev server on :5173
cd spa && npm test                           # SPA tests
```
Full setup, Docker, and deploy details: [docs/development.md](docs/development.md).

## Hard rules

Breaking any of these causes bugs. Details in [docs/domain.md](docs/domain.md) and [docs/architecture.md](docs/architecture.md).

- Amounts are `int64` cents. Convert only with `utils.FormatAmount` / `utils.ParseAmount`.
- A transaction's splits must sum to zero (`ValidateSplitsBalance`).
- Only leaf accounts hold splits.
- Reconciled transactions are immutable; mutations return `ErrReconciled` — check with `errors.Is`.
- System accounts `Equity:OpeningBalances_<CCY>` must not be deleted; build/detect names with `model.OpeningBalancesAccountName` / `model.IsOpeningBalancesAccount`.
- Never sum amounts across currencies.
- Every repository/store method takes `context.Context` first and uses the `*Context` methods of `database/sql`.
- Dependency direction: `cmd`/`ui`/`internal/api` → `internal/service` → `internal/repository` ← `internal/store`. `internal/model` imports no kea package. The service never imports the store.
- Multi-step writes go through `TransactionManager.ExecTx`.
- Wrap errors with `%w`; use the sentinel errors in `internal/service/errors.go` and `internal/repository/errors.go`.

## Where to look

| Task | Read first |
|---|---|
| Orientation | [docs/README.md](docs/README.md) → [docs/architecture.md](docs/architecture.md) |
| Domain rules | [docs/domain.md](docs/domain.md) |
| New migration | [docs/recipes/add-migration.md](docs/recipes/add-migration.md) |
| New service logic / repo method | [docs/recipes/add-service-method.md](docs/recipes/add-service-method.md) |
| New HTTP endpoint | [docs/recipes/add-api-endpoint.md](docs/recipes/add-api-endpoint.md), [docs/http-api.md](docs/http-api.md) |
| New CLI command or flag | [docs/recipes/add-cli-command.md](docs/recipes/add-cli-command.md) |
| New SPA page | [docs/recipes/add-spa-page.md](docs/recipes/add-spa-page.md) |
| Why it is built this way | [docs/decisions.md](docs/decisions.md) |
| Testing patterns and mocks | [docs/development.md](docs/development.md#testing) |
| Operating the kea CLI | [SKILL.md](SKILL.md) |

## Testing in one paragraph

Service tests are white-box (`package service`) with hand-written in-memory mocks in `internal/service/testhelper_test.go`; store tests use real SQLite; API tests use `httptest`; SPA tests use Vitest. Add tests at every layer you touch. See [docs/development.md](docs/development.md#testing).

## Keeping docs current

When a change alters a pattern, an endpoint, or a domain rule, update the matching doc under `docs/` in the same commit and run `scripts/check-docs.sh`. New design specs and plans go to `docs/history/superpowers/specs/` and `docs/history/superpowers/plans/`.

## Known issues

- Inside the Docker `app` container (runs as root), `TestSwap_FailedSwapKeepsOldConnection` in `internal/store` fails; it assumes an unprivileged user.
```
Adjust the "Commands" block if `make build` output name or the SPA test command differ (check `Makefile` and `spa/package.json`). Ensure the anchor links `#testing` match the heading in `docs/development.md`.

- [ ] **Step 2: Replace `CLAUDE.md` with an import**

```markdown
@AGENTS.md
```
(single line, trailing newline)

- [ ] **Step 3: Verify**

```bash
scripts/check-docs.sh
wc -l AGENTS.md
cat CLAUDE.md
```
Expected: checker passes; AGENTS.md is 100–150 lines; CLAUDE.md is `@AGENTS.md`.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md CLAUDE.md
git commit -m "docs: rewrite AGENTS.md as short entry point; CLAUDE.md imports it

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 16: Root `README.md` and `spa/README.md`

**Files:**
- Create: `README.md`
- Rewrite: `spa/README.md`

- [ ] **Step 1: Write `README.md`**

```markdown
# kea

kea is a personal double-entry accounting tool. Use it from the terminal (CLI and TUI) or run `kea serve` for a web UI. Each ledger is a local SQLite database.

## Quick start

```bash
make build
./kea_test --help
./kea_test serve      # web UI at http://localhost:8080
```
Or with Docker: `docker compose up -d` (API on :8080, SPA dev server on :5173).

## Documentation

- [docs/README.md](docs/README.md) — start here: architecture, domain rules, development, recipes, decisions.
- [AGENTS.md](AGENTS.md) — hard rules and a task-to-doc index (for AI agents and contributors).
- [SKILL.md](SKILL.md) — how to operate the kea CLI as an agent.

## License

See [LICENSE](LICENSE).
```
Verify the quick-start commands actually work (`make build && ./kea_test --help`).

- [ ] **Step 2: Rewrite `spa/README.md`**

Read `spa/package.json`, `spa/src/routes/`, `spa/vite.config.ts`. Write:

```markdown
# kea SPA

React SPA served by `kea serve` (embedded into the Go binary) and developed with the Vite dev server.

> Patterns for adding pages: [../docs/recipes/add-spa-page.md](../docs/recipes/add-spa-page.md). Toolchain details: [../docs/development.md](../docs/development.md#spa-toolchain).

## Stack
<bullets from package.json: Vite, React, TypeScript, TanStack Router, TanStack Query, Tailwind, shadcn/ui, Biome, Vitest + Testing Library — plus any notable libs (charts, grid layout).>

## Routes
<table: URL | route file | purpose — one row per top-level page: /dashboard, /balances, /accounts, /transactions, /reconcile, /reports (+ sub-reports), /settings.>

## Commands
<npm install, npm run dev, npm test, lint/format, build (and that the build writes into internal/web/dist) — exact script names from package.json.>
```
Replace every `<…>`; none may remain.

- [ ] **Step 3: Verify**

```bash
scripts/check-docs.sh
```
Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add README.md spa/README.md
git commit -m "docs: add root README and refresh SPA README

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 17: Final verification

- [ ] **Step 1: Whole-set checks**

```bash
scripts/check-docs.sh
go build ./... && go vet ./...
go test ./...
grep -rnE '<[^>`]*…|TBD|TODO' AGENTS.md README.md spa/README.md docs/*.md docs/recipes/*.md || echo "no placeholders"
```
Expected: checker passes; build/vet/tests pass (the Docker-root test failure does not apply on the host); "no placeholders".

- [ ] **Step 2: Cold-read test with a fresh agent**

Dispatch a fresh subagent with no context and only this prompt: "Read AGENTS.md and follow its links as needed. Then answer: (1) Which files would you touch to add a `notes` text field to accounts, end to end (DB, service, API, CLI, SPA)? (2) What happens if you try to edit a reconciled transaction? (3) Which flag-handling pattern should a new 2-flag command use?" Compare its answers with the code. Any wrong or missing answer points to a doc gap: fix that doc, rerun `scripts/check-docs.sh`, and commit `docs: fix gaps found in cold-read review`.

- [ ] **Step 3: Report** the list of docs created, line counts (`wc -l AGENTS.md README.md docs/*.md docs/recipes/*.md`), and the cold-read result to the user.
