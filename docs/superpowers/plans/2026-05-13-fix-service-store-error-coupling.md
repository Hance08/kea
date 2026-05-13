# Fix Service→Store Error Coupling (Issue #62) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all `internal/store` imports from `internal/service/` by defining error sentinels in `internal/repository/` and having the store wrap its errors with those sentinels.

**Architecture:** Currently `service` imports `store` to check `store.ErrRecordNotFound` and `store.ErrAccountExists`. The fix introduces `repository.ErrNotFound` and `repository.ErrAlreadyExists` in a new `internal/repository/errors.go`. The store wraps its errors with these repository-level sentinels (double-wrapping so both `store.ErrX` and `repository.ErrX` are detectable via `errors.Is`). The service then checks against `repository.ErrX` and drops the `store` import entirely. Test mocks also switch to repository sentinels.

**Tech Stack:** Go standard library (`errors`, `fmt`), existing test mocks in `internal/service/testhelper_test.go`.

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Create | `internal/repository/errors.go` | Add `ErrNotFound`, `ErrAlreadyExists` |
| Modify | `internal/store/errors.go` | Wrap store sentinels with repository sentinels |
| Modify | `internal/service/account_service.go` | Replace `store.ErrRecordNotFound` → `repository.ErrNotFound`, drop `store` import |
| Modify | `internal/service/account_ops.go` | Replace `store.ErrAccountExists` → `repository.ErrAlreadyExists`, replace `store.ErrRecordNotFound` → `repository.ErrNotFound`, drop `store` import |
| Modify | `internal/service/testhelper_test.go` | Replace `store.ErrAccountExists` / `store.ErrRecordNotFound` → repository equivalents, drop `store` import |

---

## Task 1: Create repository-level error sentinels

**Files:**
- Create: `internal/repository/errors.go`

- [ ] **Step 1: Create the file**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package repository

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)
```

- [ ] **Step 2: Verify the package compiles**

Run: `go build ./internal/repository/...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/repository/errors.go
git commit -m "feat: add ErrNotFound and ErrAlreadyExists sentinels to repository package"
```

---

## Task 2: Wrap store errors with repository sentinels

**Files:**
- Modify: `internal/store/errors.go`

- [ ] **Step 1: Write a test that verifies double-wrapping**

Create a test inline to verify the relationship. In a terminal:

```bash
go test ./internal/store/ -run TestErrorWrapping -v
```

This will fail because the test doesn't exist yet. Add it to a new file `internal/store/errors_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"errors"
	"testing"

	"github.com/hance08/kea/internal/repository"
)

func TestErrorSentinelsWrapRepositoryErrors(t *testing.T) {
	tests := []struct {
		name      string
		storeErr  error
		repoErr   error
	}{
		{"ErrRecordNotFound wraps repository.ErrNotFound", ErrRecordNotFound, repository.ErrNotFound},
		{"ErrAccountExists wraps repository.ErrAlreadyExists", ErrAccountExists, repository.ErrAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.storeErr, tt.repoErr) {
				t.Errorf("%v should wrap %v", tt.storeErr, tt.repoErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/store/ -run TestErrorSentinelsWrapRepositoryErrors -v`
Expected: FAIL — store errors don't wrap repository errors yet.

- [ ] **Step 3: Update store error sentinels to wrap repository sentinels**

Replace `internal/store/errors.go` with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"fmt"

	"github.com/hance08/kea/internal/repository"
)

var (
	ErrAccountExists       = fmt.Errorf("account already exists: %w", repository.ErrAlreadyExists)
	ErrRecordNotFound      = fmt.Errorf("record not found: %w", repository.ErrNotFound)
	ErrConstraintViolation = fmt.Errorf("database constraint violation")
)
```

Now `errors.Is(ErrRecordNotFound, repository.ErrNotFound)` is true, and `errors.Is(ErrAccountExists, repository.ErrAlreadyExists)` is true. Any error chain wrapping `store.ErrRecordNotFound` is also detectable via `repository.ErrNotFound`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/store/ -run TestErrorSentinelsWrapRepositoryErrors -v`
Expected: PASS.

- [ ] **Step 5: Run all store tests to check for regressions**

Run: `go test ./internal/store/... -v 2>&1 | tail -20`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/errors.go internal/store/errors_test.go
git commit -m "refactor: wrap store error sentinels with repository-level sentinels"
```

---

## Task 3: Update service layer to use repository sentinels

**Files:**
- Modify: `internal/service/account_service.go`
- Modify: `internal/service/account_ops.go`

- [ ] **Step 1: Update `account_service.go`**

In `internal/service/account_service.go`, line 36, replace:
```go
if errors.Is(err, store.ErrRecordNotFound) {
```
with:
```go
if errors.Is(err, repository.ErrNotFound) {
```

Then update the import block — remove `"github.com/hance08/kea/internal/store"` (the `repository` import already exists):

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/utils"
)
```

- [ ] **Step 2: Update `account_ops.go`**

In `internal/service/account_ops.go`:

Line 31 — replace:
```go
if errors.Is(err, store.ErrAccountExists) {
```
with:
```go
if errors.Is(err, repository.ErrAlreadyExists) {
```

Line 95 — replace:
```go
if !errors.Is(err, store.ErrRecordNotFound) {
```
with:
```go
if !errors.Is(err, repository.ErrNotFound) {
```

Delete the comment on line 92 (`// so GetAccountByName here returns store.ErrRecordNotFound directly (no service translation).`).

Then update the import block — remove `"github.com/hance08/kea/internal/store"`, add `"github.com/hance08/kea/internal/repository"`:

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/service/...`
Expected: no output (success).

- [ ] **Step 4: Commit**

```bash
git add internal/service/account_service.go internal/service/account_ops.go
git commit -m "refactor: replace store error checks with repository sentinels in service layer"
```

---

## Task 4: Update test mocks to use repository sentinels

**Files:**
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Update mock error returns**

In `internal/service/testhelper_test.go`:

Line 75 — replace:
```go
return 0, fmt.Errorf("account %q already exists: %w", name, store.ErrAccountExists)
```
with:
```go
return 0, fmt.Errorf("account %q already exists: %w", name, repository.ErrAlreadyExists)
```

Line 106 — replace:
```go
return nil, fmt.Errorf("account %q not found: %w", name, store.ErrRecordNotFound)
```
with:
```go
return nil, fmt.Errorf("account %q not found: %w", name, repository.ErrNotFound)
```

Then update the import block — remove `"github.com/hance08/kea/internal/store"` (the `repository` import already exists):

```go
import (
	"context"
	"errors"
	"fmt"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)
```

- [ ] **Step 2: Run all service tests**

Run: `go test ./internal/service/... -v 2>&1 | tail -30`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/service/testhelper_test.go
git commit -m "refactor: update test mocks to use repository error sentinels"
```

---

## Task 5: Final verification

- [ ] **Step 1: Confirm no `internal/store` imports remain in `internal/service/`**

```bash
grep -r '"github.com/hance08/kea/internal/store"' internal/service/
```

Expected: no output.

- [ ] **Step 2: Run all tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 3: Run build**

```bash
make build
```

Expected: `./kea_test` binary produced with no errors.
