# AGENTS.md

Guidance for AI coding agents and contributors working in this repository.

kea is a personal double-entry accounting tool: a Go CLI/TUI plus an HTTP API (`kea serve`) with an embedded React SPA, storing each ledger in a local SQLite database.

## Language

All code, comments, test names, commit messages, and documentation must be written in English.

## Commands

```bash
make build                                   # builds ./kea
make run                                     # go run ./cmd/kea
go test ./...                                # all Go tests
go test ./internal/service/ -run TestName    # one test
go build ./... && go vet ./...               # compile + vet
scripts/check-docs.sh                        # verify paths referenced in docs
docker compose up -d                         # app on :8080, SPA dev server on :5173
cd spa && npm test                           # SPA tests
```

Full setup, Docker, and deploy details: [docs/development.md](docs/development.md).

## Project layout

- `cmd/` - Cobra commands (`cmd/account`, `cmd/transaction`, `cmd/ledger`, `cmd/kea` is `main`).
- `ui/` - interactive prompts (huh), views (pterm and tablewriter), reconcile TUI (bubbletea).
- `internal/` - `api` (HTTP), `app` (wiring), `service`, `repository`, `store`, `model`, `config`, `ledger`, `backup`, `utils`, `web`.
- `migrations/` - embedded SQL migrations.
- `spa/` - React SPA (Vite, Vitest); built output is embedded via `internal/web`.
- `scripts/`, `docker/` - deploy scripts and dev containers.
- `docs/` - contributor documentation.

## Hard rules

Breaking any of these causes bugs. Details in [docs/domain.md](docs/domain.md) and [docs/architecture.md](docs/architecture.md).

- Amounts are `int64` cents. Convert only with `utils.FormatAmount` / `utils.ParseAmount`.
- A transaction's splits must sum to zero (`ValidateSplitsBalance`).
- Only leaf accounts hold splits.
- Reconciled transactions are immutable; mutations return `ErrReconciled` - check with `errors.Is`.
- System accounts `Equity:OpeningBalances_<CCY>` must not be deleted; build/detect names with `model.OpeningBalancesAccountName` / `model.IsOpeningBalancesAccount`.
- Never sum amounts across currencies.
- Every repository/store method takes `context.Context` first and uses the `*Context` methods of `database/sql`.
- Dependency direction: `cmd`/`ui`/`internal/api` -> `internal/service` -> `internal/repository` <- `internal/store`. `internal/model` imports no kea package. The service never imports the store.
  - Deliberate exceptions: `internal/api` and `cmd/ledger` import `internal/app` to call `app.InitLedgerDB`; `ui/views` imports `ui/prompts`.
- Multi-step writes go through `TransactionManager.ExecTx` (no nesting).
- Wrap errors with `%w`; services return service sentinels (`internal/service/errors.go`), never raw repository errors.
- A new service sentinel needs a case in `mapError` (`internal/api/errors.go`).

## Where to look

| Task | Read first |
|---|---|
| Orientation | [docs/README.md](docs/README.md) -> [docs/architecture.md](docs/architecture.md) |
| Domain rules | [docs/domain.md](docs/domain.md) |
| New migration | [docs/recipes/add-migration.md](docs/recipes/add-migration.md) |
| New service logic / repo method | [docs/recipes/add-service-method.md](docs/recipes/add-service-method.md) |
| New HTTP endpoint | [docs/recipes/add-api-endpoint.md](docs/recipes/add-api-endpoint.md), [docs/http-api.md](docs/http-api.md) |
| New CLI command or flag | [docs/recipes/add-cli-command.md](docs/recipes/add-cli-command.md) |
| New SPA page | [docs/recipes/add-spa-page.md](docs/recipes/add-spa-page.md) · [spa/README.md](spa/README.md) |
| Why it is built this way | [docs/decisions.md](docs/decisions.md) |
| Testing patterns and mocks | [docs/development.md](docs/development.md#testing) |
| Operating the kea CLI | [SKILL.md](SKILL.md) |

## Testing in one paragraph

Service tests are white-box (`package service`) with hand-written in-memory mocks in `internal/service/testhelper_test.go`; store tests use real SQLite via `setupTestDB`; API tests use `httptest` helpers in `internal/api/testhelper_test.go`; SPA tests use Vitest. Add tests at every layer you touch. See [docs/development.md](docs/development.md#testing).

## Keeping docs current

When a change alters a pattern, an endpoint, or a domain rule, update the matching doc under `docs/` in the same commit and run `scripts/check-docs.sh`. New design specs and plans go to `docs/history/superpowers/specs/` and `docs/history/superpowers/plans/`.

## Known issues

- Inside the Docker `app` container (runs as root), `TestSwap_FailedSwapKeepsOldConnection` in `internal/store` fails; it assumes an unprivileged user.
