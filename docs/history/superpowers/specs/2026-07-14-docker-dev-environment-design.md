# Docker Development Environment — Design

## Goal

Let contributors develop kea (the Go CLI/TUI + API and the `spa/` reconcile frontend)
inside containers, without installing a Go toolchain, a C compiler (required for the
CGO-based SQLite driver), or Node locally.

This is a development-time convenience, not a packaging/deployment feature. It does not
change how kea is built or released; `make build`, `make run`, `go test ./...`, and the
existing `spa-*` Make targets remain the source of truth and keep working unchanged
outside of Docker.

## Non-goals

- Packaging a release image of the `kea` binary for end users.
- Running `kea serve` as a long-lived production service.
- VS Code `.devcontainer/` integration (plain Dockerfiles + compose only).
- Automatic rebuild/hot-reload for the Go service (`air` or similar). Code changes are
  picked up by manually re-running `go run`/`go test` inside the container.

## Architecture

Two independent services in a root-level `docker-compose.yml`, mirroring the existing
separation between the Go backend and the `spa/` frontend toolchain:

- **`app`** — Go dev container. Builds/tests/runs the `kea` CLI/TUI and `kea serve` API.
- **`spa`** — Node dev container. Runs the Vite dev server for the reconcile SPA.

New files:

```
docker/
  app.Dockerfile
  spa.Dockerfile
docker-compose.yml
.dockerignore
```

Neither Dockerfile copies source into the image. Source is bind-mounted by
`docker-compose.yml` so edits made on the host are immediately visible inside the
container — the whole point of a dev container is editing with host tools (editor, git)
while running commands in a container with the right toolchain.

## `app` service

- Base image: `golang:1.25` (Debian-based), **not** `alpine`. `mattn/go-sqlite3`
  requires CGO, and CGO + musl (Alpine's libc) is a common source of build breakage;
  Debian's glibc avoids that class of problem.
- `docker/app.Dockerfile` installs `gcc` and `libc6-dev` (via `apt-get`) so `CGO_ENABLED=1`
  builds succeed. No `go build`/`go mod download` happens at image-build time — the image
  only provisions the toolchain.
- `docker-compose.yml` config for `app`:
  - Bind-mount: repo root → `/workspace`, working directory `/workspace`.
  - Named volume: `/root/go/pkg/mod` — caches downloaded Go modules across
    `docker compose up`/`down` cycles so `go mod download` isn't repeated every time.
  - Named volume: `/root/.config/kea` — this is where `app.GetAppDataDir()` resolves to
    inside the container (`os.UserConfigDir()` → `~/.config` on Linux, joined with
    `kea`). Persists ledger SQLite databases across container recreation.
  - Port mapping: `8080:8080`, matching `config.NewDefault().Server.Port`, so
    `kea serve` is reachable from the host browser at `http://localhost:8080`.
  - Command: idles (`sleep infinity`) — no auto-run. Development happens via
    `docker compose exec app <command>`.

## `spa` service

- Base image: `node:22` (matches the Node version already used in `spa/`, confirmed via
  local `node -v`).
- `docker/spa.Dockerfile` installs no extra system packages — Node/npm is sufficient.
- `docker-compose.yml` config for `spa`:
  - Bind-mount: `spa/` → `/workspace`, working directory `/workspace`.
  - Named volume: `/workspace/node_modules` — isolates native/platform-specific
    dependencies from whatever `node_modules` may already exist on the host, avoiding
    cross-platform binary mismatches.
  - Port mapping: `5173:5173`, the Vite dev server default, which already matches the
    default `cors_origins: ["http://localhost:5173"]` in `config.NewDefault()` — no
    config changes needed for the two services to talk to each other.
  - Command: idles (`sleep infinity`) — no auto-run.

## Networking

Both services join the default compose network. From the host, the API is at
`http://localhost:8080` and the SPA dev server at `http://localhost:5173` (standard
compose port publishing). From inside the `spa` container, the API is reachable at
`http://app:8080` via compose's built-in DNS, if ever needed (the SPA currently talks to
the API via the browser, not container-to-container, so this is incidental rather than
load-bearing).

## `.dockerignore`

Excludes, so the build context stays small and stale artifacts never leak into images:
- `node_modules/`, `spa/node_modules/`
- `spa/dist/`, `internal/web/dist/`
- `.git/`
- Built binaries: `kea`, `kea_test`

## Error handling / edge cases

- No secrets or credentials are involved — the ledger DB is local SQLite. If a
  contributor wants a custom `config.yaml`, they can bind-mount it themselves
  (`kea --config`); no new env-var plumbing is introduced by this change.
- The Go module cache volume caches downloaded modules only, not build output — a
  `go.sum` change is picked up correctly on the next `go mod download`/`go test` inside
  the container, no manual cache eviction needed.
- If a contributor already has a local `~/.config/kea` with ledgers, those are untouched
  since the named volume is a separate Docker-managed volume, not the host directory —
  container and host ledger data are intentionally isolated.

## Verification plan

This is dev infrastructure, so verification means confirming the environment works end
to end, not adding Go tests:

1. `docker compose up -d`
2. `docker compose exec app go test ./...` — confirms the Go toolchain and CGO SQLite
   build succeed inside the container.
3. `docker compose exec app go run ./cmd/kea ledger add demo` then
   `docker compose exec app go run ./cmd/kea serve` — confirms the CLI and SQLite-backed
   API work, and port 8080 is reachable from the host.
4. `docker compose exec spa npm run dev` — confirms the Vite dev server starts, is
   reachable from the host on port 5173, and (per existing CORS config) can call the API
   on port 8080.
