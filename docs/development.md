# Development

> **Read this when:** you set up the project, run or test it, work on the SPA, or deploy `kea serve`.
>
> **Related:** [architecture.md](architecture.md) · [recipes/](recipes/)

## Prerequisites

- Go 1.25.4 or newer (`go.mod`).
- A C toolchain (`gcc`, libc headers). The SQLite driver is `github.com/mattn/go-sqlite3`, which needs `CGO_ENABLED=1`. `docker/app.Dockerfile` installs `gcc` and sets this for you.
- Node 22 and npm for the SPA in `spa/` (`docker/spa.Dockerfile` uses `node:22`). Only needed if you touch the SPA or build the embedded bundle.

Docker removes all three requirements; see [Run with Docker](#run-with-docker).

## Run locally

```bash
make build                  # go build ./cmd/kea, writes ./kea (gitignored)
make run                    # go run ./cmd/kea
go run ./cmd/kea <args>     # any subcommand, e.g. `account list`, `serve`
```

`make build` writes `./kea`; the older `kea_test` name is gone.

On first run with no ledger configured, startup creates `config.yaml` and a `default` ledger (`kea.db`) in the data directory, then asks for a default currency. A non-interactive run falls back to USD with a warning. Startup order is in [architecture.md](architecture.md#startup-sequence).

Other make targets: `make test-race` (`go test -race ./...`), `make spa-install`, `make spa-dev`, `make spa-build`, `make build-all`, `make spa-clean`. Dependencies: `go mod tidy`.

## Run with Docker

`docker-compose.yml` defines two dev containers that bind-mount the repo, so host edits apply immediately:

- `app` (Go, `docker/app.Dockerfile`) runs `go run ./cmd/kea serve` on `localhost:8080`. It binds `0.0.0.0` through `KEA_SERVER_HOST` and `KEA_SERVER_PORT`, and bootstraps a `default` ledger on first run like a local run does.
- `spa` (Node, `docker/spa.Dockerfile`) runs `npm install` then the Vite dev server on `localhost:5173`, proxying `/api` to `http://app:8080`.

```bash
docker compose up -d                              # start both containers
docker compose exec app go test ./...             # run Go tests inside the container
docker compose exec app go run ./cmd/kea <args>   # CLI/TUI, e.g. `ledger add`, `account list`
docker compose down                               # stop both containers
```

Ledger data (`config.yaml`, `ledgers.yaml`, SQLite files) lives in `./data` at the repo root (gitignored), mounted at `/root/.config/kea` in the container. To reuse an existing data directory, copy `.env.example` to `.env` and set `KEA_DATA_DIR` (for example your host `~/.config/kea`).

Known limitation: the `app` container runs as root, so `TestSwap_FailedSwapKeepsOldConnection` in `internal/store/sqlite_swap_test.go` fails there. It assumes an unprivileged user; it is not a Docker bug.

## Config and data locations

The data directory is `<UserConfigDir>/kea`, from `GetAppDataDir` in `internal/app/app.go`:

| OS | Directory |
| --- | --- |
| macOS | `~/Library/Application Support/kea` |
| Linux | `~/.config/kea` (honors `XDG_CONFIG_HOME`) |
| Fallback | `~/.kea` when the user config dir cannot be determined |

The deploy scripts and Docker setup print or mount `~/.config/kea`; on macOS the Go default above applies unless the data was moved there.

Files in that directory:

- `config.yaml` is created by `createDefaultConfig` in `cmd/root.go` and loaded by `initConfig` via viper. Pass `--config <file>` (`-c`) to use another file.
- `ledgers.yaml` is the ledger registry, read and written by `internal/ledger/registry.go`. It maps ledger names to DB paths and records the active one. A fresh install registers `kea.db` in the same directory as `default`.
- `*.db` are SQLite files, one per ledger. `internal/store` opens them and applies the embedded migrations in `migrations/`.
- `backups/` is written by `backup.Run` in `internal/backup/backup.go` before the store opens. It keeps daily (7), weekly (4) and monthly (12) snapshots named `<db>_<tier>_<label>.db`, taken with the SQLite online backup API.

`config.yaml` keys that matter (`internal/config/config.go`):

| Key | Default | Notes |
| --- | --- | --- |
| `defaults.currency` | empty | ISO 4217 code; set by the first-run wizard |
| `display.hide_decimals` | `false` | Persisted by `saveDisplayHideDecimals` in `cmd/save_config.go` |
| `server.host` | `localhost` | `kea serve` bind address |
| `server.port` | `8080` | `kea serve` port |
| `server.cors_origins` | `http://localhost:5173` | Allowed origins for the dev SPA |
| `database.path` | empty | Loaded and `~`-expanded by `initConfig`, but the store opens the active ledger's path from `ledgers.yaml`, so it has no effect on which DB is used |

Environment overrides: viper uses the `KEA_` prefix with dots replaced by underscores, so `KEA_SERVER_HOST`, `KEA_SERVER_PORT` and `KEA_DEFAULTS_CURRENCY` override the file. `KEA_LEDGER` overrides the active ledger name from `ledgers.yaml` (read in `internal/ledger/registry.go`). `NO_COLOR` or `--no-color` disables colored output.

Config writes use `rewriteYAMLKey` in `cmd/save_config.go`, which edits a single key in place so viper defaults and env overrides are never written back to the file.

## Testing

Each layer has its own test style. Pick the lowest layer that can express the behavior.

### Service tests (mocks)

Files in `internal/service/*_test.go` use `package service` (white-box) with hand-written mocks from `internal/service/testhelper_test.go`. No database is involved; storage is in-memory maps.

- Mocks: `mockAccountRepo`, `mockTransactionRepo`, `mockCombinedRepo`, `mockTransactionManager`.
- Error injection: maps and fields on the mocks, e.g. `getByIDErr map[int64]error`, `getByNameErr map[string]error`, `createErr`.
- Call recorders: slices such as `renameCalls` and `updateMetadataCalls` on `mockAccountRepo`.
- Factories: `newMockAccountRepo()`, `newMockTransactionRepo()`, `newTestAccountService(accRepo, txRepo)`, `newTestTransactionService(accRepo, txRepo)`, `defaultConfig()`.

The arrange/act/assert style, from `TestGetAccountByID` in `internal/service/account_service_test.go`:

```go
t.Run("passes through other errors", func(t *testing.T) {
	accRepo := newMockAccountRepo()
	dbErr := fmt.Errorf("connection refused")
	accRepo.getByIDErr[42] = dbErr
	svc := newTestAccountService(accRepo, newMockTransactionRepo())

	acc, err := svc.GetAccountByID(context.Background(), 42)

	assert.Nil(t, acc)
	require.Error(t, err)
	assert.Equal(t, dbErr, err)
})
```

Tests use `testify` (`assert`, `require`) and table-style `t.Run` subtests.

### Store tests (real SQLite)

`internal/store/*_test.go` use the external package `store_test`. The helper `setupTestDB` in `internal/store/sqlite_account_test.go` creates a `store.Store` over a file in `t.TempDir()` with `store.NewStore(dbPath, migrations.FS)`, which runs every migration from `migrations/`, and registers `Close` with `t.Cleanup`. Other store test files call it directly. Use this layer for SQL, constraints and migration behavior.

### API tests

`internal/api/*_test.go` use `package api` and `net/http/httptest`. `internal/api/testhelper_test.go` provides:

- `newServerWithStore(t)` and `newServerForWrite(t)` build a `Server` over a real temp-dir SQLite store and return an `httptest.Server` fronting `srv.routes()` plus the `*service.Service`. Variants set the currency or `display.hide_decimals`.
- `newTestServerWithLedger(t)` wires a real on-disk `ledger.Registry` in a temp dir and a fake switch function that records calls.
- `seedAccount` and `seedTransaction` create fixtures through the service layer.

Tests issue real HTTP requests against `ts.URL` and assert on status and JSON. Endpoint contracts are in [http-api.md](http-api.md).

### cmd tests

`cmd/*_test.go` use `package cmd`. They test runner structs and helpers directly rather than executing the binary: small fakes implement the narrow interface a runner depends on (for example `fakeInfoProvider` and `capturingView` in `cmd/info_test.go` drive `infoRunner.Run`), and `cmd/save_config_test.go` exercises YAML rewriting against temp files. `cmd/serve_test.go` only checks the command shape. Prefer testing `Run` methods and pure helpers over full cobra execution.

### SPA tests

Vitest with jsdom and Testing Library, in `spa/src/test/` (config in the `test` block of `spa/vite.config.ts`).

- `spa/src/test/setup.tsx` loads `@testing-library/jest-dom`, runs `cleanup` after each test and shims `localStorage` when Node provides a non-Storage global. It is deliberately thin; do not import the route tree there.
- `spa/src/test/test-app.tsx` provides `makeTestApp(initialPath)`, which renders the real route tree in a memory router with a fresh `QueryClient`, and `withServerConfig` for component tests that need server config.
- API calls are mocked per test file with `vi.mock('@/lib/api', ...)` or `vi.mock('@/lib/accounts', ...)`, spreading `vi.importActual` and replacing only the needed functions with `vi.fn().mockResolvedValue(...)`. See `spa/src/test/accounts.list.test.tsx`. There is no network mocking layer.
- File names follow `<area>.<topic>.test.tsx`.

### Commands

```bash
go test ./...                                       # all Go packages
go test ./internal/service/...                      # one package tree
go test ./internal/service/ -run TestGetAccountByID # one test
go test ./internal/service/ -v -run TestGetAccountByID
go test -race ./...                                 # same as `make test-race`

cd spa && npm test                                  # Vitest, single run
cd spa && npm run test:watch                        # Vitest, watch mode
cd spa && npm run check                             # Biome lint and format check
```

## SPA toolchain

The SPA is Vite, React, TypeScript, TanStack Router and Query, Tailwind and shadcn/ui primitives. npm scripts in `spa/package.json`:

| Script | Purpose |
| --- | --- |
| `npm run dev` | Vite dev server on `:5173` |
| `npm run build` | `tsc -b && vite build`, writes `internal/web/dist` |
| `npm run preview` | Serve a production build locally |
| `npm test` | Vitest, single run |
| `npm run test:watch` | Vitest, watch mode |
| `npm run check` | `biome check src` |
| `npm run check:write` | `biome check --write src` (auto-fix and format) |

Dev workflow: run `make run` (API on `:8080`) in one terminal and `make spa-dev` in another, then open `http://localhost:5173`. In `spa/vite.config.ts` the dev server proxies `/api` to `VITE_API_PROXY_TARGET`, defaulting to `http://localhost:8080`. Docker sets it to `http://app:8080`. The dev origin must be listed in `server.cors_origins`; the default already includes it.

Biome (`spa/biome.json`) enforces 2-space indentation, single quotes, semicolons, trailing commas and a 100-column width. It ignores `src/components/ui` (generated shadcn files) and `src/routeTree.gen.ts`.

Routing is file-based. Route files live in `spa/src/routes/` (`accounts.$id.edit.tsx`, `reports.balance-sheet.tsx`, and so on). The `TanStackRouterVite` plugin regenerates `spa/src/routeTree.gen.ts` whenever the dev server or a build runs, and the generated file is committed. After adding or renaming a route file, run `npm run dev` or `npm run build` once and commit the updated tree. Path alias `@` maps to `spa/src`.

## Building the embedded SPA

`kea serve` serves the SPA from `internal/web/dist` through `//go:embed all:dist` in `internal/web/embed.go`.

```bash
make build-all     # spa-build (npm run build) then go build; writes ./kea
make spa-build     # bundle only
make spa-clean     # remove the bundle and restore the placeholder index.html
```

The bundle is not committed. `.gitignore` excludes `internal/web/dist/*` except `internal/web/dist/index.html`, a placeholder that keeps `go:embed` compiling on a fresh checkout. Earlier history contains `build(spa): refresh embedded bundle` commits from before the bundle was ignored; do not recreate that pattern. A plain `make build` without a prior `make spa-build` serves only the placeholder. Run `npm install` first (`make spa-install`) on a fresh clone. How the server serves the files is in [architecture.md](architecture.md#how-kea-serve-ships-the-spa).

## Deploying `kea serve`

All three scripts run `make build-all`, install the binary, register a service that runs `kea serve`, and are idempotent: re-run after pulling to rebuild and redeploy. Configure host and port in `config.yaml` in the service user's data directory.

### `scripts/install-launchd.sh` (macOS)

Run as your normal user, not root: `./scripts/install-launchd.sh`. It installs the binary to `/usr/local/bin/kea` with `sudo`, renders `scripts/kea.plist.template` into `~/Library/LaunchAgents/com.kea.serve.plist` and loads it, so the server starts at login and restarts on crash. Logs go to `~/Library/Logs/kea/kea.log`. Stop with `launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.kea.serve.plist`; start again with `launchctl bootstrap` and the same arguments. The script has no uninstall mode; after stopping, delete the plist and the binary by hand.

### `scripts/install-systemd.sh` (Linux, system service)

Run with root: `sudo ./scripts/install-systemd.sh`. It builds as the invoking user, installs `/usr/local/bin/kea`, creates a `kea` system user with home `/var/lib/kea`, installs `scripts/kea.service` to `/etc/systemd/system/kea.service` (hardened with `ProtectSystem=strict`) and runs `systemctl enable --now kea`. Data lives in `/var/lib/kea/.config/kea`. Logs: `journalctl -u kea -f`. No uninstall mode; use `systemctl disable --now kea` and remove the unit file.

### `scripts/install-systemd-user.sh` (Linux, user service)

Run as your normal user: `./scripts/install-systemd-user.sh`. It installs the binary to `~/.local/bin/kea` and `scripts/kea-user.service` to `~/.config/systemd/user/kea.service`, then enables it with `systemctl --user`. Data lives in `~/.config/kea`. Logs: `journalctl --user -u kea -f`. The service starts at login by default; run `sudo loginctl enable-linger $USER` to start at boot and survive logout. No uninstall mode; use `systemctl --user disable --now kea` and remove the unit file.

## Docs maintenance

- Run `scripts/check-docs.sh` after editing any doc. It verifies that backticked repo paths and relative links in `AGENTS.md`, `README.md`, `spa/README.md`, `docs/*.md` and the recipe docs under `docs/` exist.
- Update the matching doc in the same commit as a pattern, endpoint or domain rule change: layers and startup in [architecture.md](architecture.md), domain rules in [domain.md](domain.md), endpoints in [http-api.md](http-api.md), how-tos in [recipes/](recipes/), and rationale in [decisions.md](decisions.md).
- New design specs go to `docs/history/superpowers/specs/` and plans to `docs/history/superpowers/plans/`.
