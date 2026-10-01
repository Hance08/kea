# Docker Development Environment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let contributors develop kea's Go backend and `spa/` frontend inside containers, without installing a Go toolchain, a C compiler (for the CGO-based SQLite driver), or Node locally.

**Architecture:** A root-level `docker-compose.yml` defines two independent services — `app` (Go, Debian-based, with `gcc`/`libc6-dev` for CGO) and `spa` (Node 22) — each bind-mounting its source from the host and idling on startup so developers drive commands via `docker compose exec`. Named volumes cache the Go module cache, npm's `node_modules`, and the kea ledger config directory across container recreation.

**Tech Stack:** Docker, Docker Compose, `golang:1.25` base image, `node:22` base image.

---

## Reference: full file contents

These are the exact contents each task writes. Later tasks assume these are correct — do not improvise different content.

**`docker/app.Dockerfile`:**
```dockerfile
FROM golang:1.25

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1

WORKDIR /workspace

CMD ["sleep", "infinity"]
```

**`docker/spa.Dockerfile`:**
```dockerfile
FROM node:22

WORKDIR /workspace

CMD ["sleep", "infinity"]
```

**`docker-compose.yml`:**
```yaml
services:
  app:
    build:
      context: .
      dockerfile: docker/app.Dockerfile
    working_dir: /workspace
    volumes:
      - .:/workspace
      - go-mod-cache:/root/go/pkg/mod
      - kea-config:/root/.config/kea
    ports:
      - "8080:8080"

  spa:
    build:
      context: .
      dockerfile: docker/spa.Dockerfile
    working_dir: /workspace
    volumes:
      - ./spa:/workspace
      - spa-node-modules:/workspace/node_modules
    ports:
      - "5173:5173"

volumes:
  go-mod-cache:
  kea-config:
  spa-node-modules:
```

**`.dockerignore`:**
```
node_modules/
spa/node_modules/
spa/dist/
internal/web/dist/
.git/
kea
kea_test
```

---

### Task 1: Add `.dockerignore`

**Files:**
- Create: `.dockerignore`

- [ ] **Step 1: Write the file**

Create `.dockerignore` at the repo root with exactly this content:

```
node_modules/
spa/node_modules/
spa/dist/
internal/web/dist/
.git/
kea
kea_test
```

- [ ] **Step 2: Verify it's picked up**

Run: `git status --porcelain .dockerignore`
Expected: `?? .dockerignore` (new untracked file)

- [ ] **Step 3: Commit**

```bash
git add .dockerignore
git commit -m "chore(docker): add .dockerignore for dev containers"
```

---

### Task 2: Add the `app` service Dockerfile

**Files:**
- Create: `docker/app.Dockerfile`

- [ ] **Step 1: Write the file**

Create `docker/app.Dockerfile` with exactly this content:

```dockerfile
FROM golang:1.25

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1

WORKDIR /workspace

CMD ["sleep", "infinity"]
```

- [ ] **Step 2: Build the image standalone to verify it's valid**

Run: `docker build -f docker/app.Dockerfile -t kea-app-dev .`
Expected: build completes with `Successfully tagged kea-app-dev:latest` (or the buildx equivalent final `naming to docker.io/library/kea-app-dev` line). No errors from the `apt-get install` step.

- [ ] **Step 3: Verify gcc is present in the image**

Run: `docker run --rm kea-app-dev gcc --version`
Expected: prints a gcc version string (e.g. `gcc (Debian ...) 12.2.0`), not "command not found".

- [ ] **Step 4: Commit**

```bash
git add docker/app.Dockerfile
git commit -m "feat(docker): add Go dev container image"
```

---

### Task 3: Add the `spa` service Dockerfile

**Files:**
- Create: `docker/spa.Dockerfile`

- [ ] **Step 1: Write the file**

Create `docker/spa.Dockerfile` with exactly this content:

```dockerfile
FROM node:22

WORKDIR /workspace

CMD ["sleep", "infinity"]
```

- [ ] **Step 2: Build the image standalone to verify it's valid**

Run: `docker build -f docker/spa.Dockerfile -t kea-spa-dev .`
Expected: build completes with `Successfully tagged kea-spa-dev:latest` (or buildx equivalent). No errors.

- [ ] **Step 3: Verify node/npm are present**

Run: `docker run --rm kea-spa-dev node -v && docker run --rm kea-spa-dev npm -v`
Expected: prints `v22.x.x` for node and an npm version string.

- [ ] **Step 4: Commit**

```bash
git add docker/spa.Dockerfile
git commit -m "feat(docker): add Node dev container image"
```

---

### Task 4: Add `docker-compose.yml`

**Files:**
- Create: `docker-compose.yml`

- [ ] **Step 1: Write the file**

Create `docker-compose.yml` at the repo root with exactly this content:

```yaml
services:
  app:
    build:
      context: .
      dockerfile: docker/app.Dockerfile
    working_dir: /workspace
    volumes:
      - .:/workspace
      - go-mod-cache:/root/go/pkg/mod
      - kea-config:/root/.config/kea
    ports:
      - "8080:8080"

  spa:
    build:
      context: .
      dockerfile: docker/spa.Dockerfile
    working_dir: /workspace
    volumes:
      - ./spa:/workspace
      - spa-node-modules:/workspace/node_modules
    ports:
      - "5173:5173"

volumes:
  go-mod-cache:
  kea-config:
  spa-node-modules:
```

- [ ] **Step 2: Validate the compose file syntax**

Run: `docker compose config`
Expected: prints the fully-resolved compose configuration with no errors (both `app` and `spa` services listed, three named volumes listed under `volumes:`).

- [ ] **Step 3: Start both services**

Run: `docker compose up -d`
Expected: both `app` and `spa` containers build (if not already built by Tasks 2/3) and start; `docker compose ps` shows both as `running`/`Up`.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml
git commit -m "feat(docker): add docker-compose for app + spa dev services"
```

---

### Task 5: Verify the `app` service end-to-end

**Files:** none (verification only — no files change in this task).

- [ ] **Step 1: Run the Go test suite inside the container**

Run: `docker compose exec app go test ./...`
Expected: all packages report `ok`, confirming the Go toolchain and CGO-based `mattn/go-sqlite3` build correctly inside the container. If this fails with a cgo/gcc-related error, re-check Task 2's Dockerfile — `gcc`/`libc6-dev` must be installed before this will pass.

- [ ] **Step 2: Build the kea binary inside the container**

Run: `docker compose exec app go build ./cmd/kea`
Expected: exits 0, produces a `kea` binary in `/workspace` (visible on the host too, via the bind mount, at the repo root — note this will be caught by `.gitignore`/`.dockerignore`, not committed).

- [ ] **Step 3: Create a ledger and start the server**

Run: `docker compose exec app go run ./cmd/kea ledger add demo`
Expected: confirms a ledger named `demo` was created (exit 0).

Run (in the background, e.g. a separate terminal or `docker compose exec -d app go run ./cmd/kea serve`):
Expected: the process starts listening; check via `docker compose logs app` for a startup log line, and no immediate crash.

- [ ] **Step 4: Confirm the API is reachable from the host**

Run: `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8080/`
Expected: an HTTP status code is returned (not a connection-refused error), confirming port 8080 is correctly published from the `app` container to the host.

- [ ] **Step 5: Stop the background serve process**

Run: `docker compose exec app pkill -f "go run ./cmd/kea serve"` (or `Ctrl+C` if run in a foreground terminal).
Expected: process stops; `curl http://localhost:8080/` now fails to connect.

---

### Task 6: Verify the `spa` service end-to-end

**Files:** none (verification only — no files change in this task).

- [ ] **Step 1: Install SPA dependencies inside the container**

Run: `docker compose exec spa npm install`
Expected: exits 0, populates the `spa-node-modules` named volume (visible via `docker compose exec spa ls node_modules | head`).

- [ ] **Step 2: Start the Vite dev server**

Run (backgrounded): `docker compose exec -d spa npm run dev -- --host 0.0.0.0`
Expected: Vite must bind to `0.0.0.0` (not just `localhost`) inside the container for the port mapping to reach it from the host — check `docker compose logs spa` for the "Local:"/"Network:" startup banner confirming it's listening.

- [ ] **Step 3: Confirm the SPA dev server is reachable from the host**

Run: `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:5173/`
Expected: HTTP 200.

- [ ] **Step 4: Confirm CORS is configured for cross-service calls**

With the `app` service's `kea serve` running (Task 5, Step 3) and the `spa` dev server running (Step 2 above), open `http://localhost:5173` in a browser and check the network tab for any API call to `http://localhost:8080` — it should succeed without a CORS error, since `config.NewDefault().Server.CORSOrigins` already includes `http://localhost:5173`.

- [ ] **Step 5: Tear down**

Run: `docker compose down`
Expected: both containers stop and are removed; named volumes (`go-mod-cache`, `kea-config`, `spa-node-modules`) persist for next time (confirm with `docker volume ls | grep kea`).

---

## Notes for the implementing engineer

- Tasks 1–4 are pure file-creation/config tasks with a build/validate step, not TDD in the unit-test sense — there's no Go/JS code being added, so "the test" is "the image builds and the tool inside it works."
- Tasks 5–6 are full end-to-end verification against the running compose stack and should be done after Tasks 1–4 are all committed.
- Do not add a VS Code `.devcontainer/`, an `air`-based hot-reload setup, or a production/release Dockerfile — all three were explicitly scoped out during design (see `docs/superpowers/specs/2026-07-14-docker-dev-environment-design.md`).
