# Fix SIGTERM Signal Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Handle SIGTERM in addition to SIGINT so process managers (systemd, Docker, launchd) trigger graceful shutdown instead of a hard kill.

**Architecture:** Single-line change in `cmd/root.go` — add `syscall.SIGTERM` to the `signal.NotifyContext` call and import `"syscall"`.

**Tech Stack:** Go stdlib (`os/signal`, `syscall`)

---

### Task 1: Add SIGTERM to signal handler

**Files:**
- Modify: `cmd/root.go:6-15` (imports), `cmd/root.go:136` (signal line)

- [ ] **Step 1: Add `syscall` import**

In `cmd/root.go`, add `"syscall"` to the stdlib import block (after `"os/signal"`):

```go
import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	// ... third-party imports unchanged
)
```

- [ ] **Step 2: Add `syscall.SIGTERM` to `signal.NotifyContext`**

Change line 136 from:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
```

to:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
```

- [ ] **Step 3: Build to verify compilation**

Run: `make build`
Expected: clean build, no errors.

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all tests pass, no regressions.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go
git commit -m "fix: handle SIGTERM for graceful shutdown (#129)"
```
