# Fix Store Error Boundary (Issue #24) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all `internal/store` imports from `cmd/` by introducing service-level error sentinels and an `app.InitLedgerDB` helper.

**Architecture:** Service methods currently pass store-level errors (`store.ErrRecordNotFound`, `store.ErrAccountExists`) straight through to callers. The fix adds two new sentinels (`service.ErrNotFound`, `service.ErrAlreadyExists`) and wraps store errors at the three public service methods that cmd currently imports store to check. The `store.NewStore` call in cmd is replaced by a new `app.InitLedgerDB` helper so that ledger initialization also stays behind the app boundary.

**Tech Stack:** Go standard library (`errors`, `fmt`), existing test mocks in `internal/service/testhelper_test.go`.

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Modify | `internal/service/errors.go` | Add `ErrNotFound`, `ErrAlreadyExists` |
| Modify | `internal/service/account_service.go` | Wrap `store.ErrRecordNotFound` in `GetAccountByName` |
| Modify | `internal/service/account_ops.go` | Wrap `store.ErrAccountExists` in `CreateAccount` |
| Modify | `internal/service/testhelper_test.go` | Fix mock to return `store.ErrAccountExists` |
| Modify | `internal/service/account_ops_test.go` | Add `ErrNotFound` and `ErrAlreadyExists` wrapping tests |
| Modify | `internal/app/app.go` | Add `InitLedgerDB(path, migrations) error` |
| Modify | `cmd/ledger/add.go` | Replace `store.NewStore` with `app.InitLedgerDB`, drop store import |
| Modify | `cmd/root.go` | Replace `store.ErrRecordNotFound` → `service.ErrNotFound`, drop store import |
| Modify | `cmd/account/create.go` | Replace `store.ErrAccountExists` → `service.ErrAlreadyExists`, drop store import |

---

## Task 1: Add service-level error sentinels

**Files:**
- Modify: `internal/service/errors.go`

Current content:
```go
package service

import "errors"

var (
    ErrReconciled = errors.New("transaction has been reconciled")
    ErrNotEditable = errors.New("operation denied on protected record")
)
```

- [ ] **Step 1: Add `ErrNotFound` and `ErrAlreadyExists`**

Replace the entire file:

```go
package service

import "errors"

var (
	ErrReconciled    = errors.New("transaction has been reconciled")
	ErrNotEditable   = errors.New("operation denied on protected record")
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("record already exists")
)
```

- [ ] **Step 2: Verify the package still compiles**

```bash
go build ./internal/service/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/service/errors.go
git commit -m "feat: add ErrNotFound and ErrAlreadyExists to service error sentinels"
```

---

## Task 2: Wrap `store.ErrAccountExists` in `CreateAccount`

**Files:**
- Modify: `internal/service/testhelper_test.go` (fix mock)
- Modify: `internal/service/account_ops.go` (wrapping)
- Modify: `internal/service/account_ops_test.go` (new test)

- [ ] **Step 1: Write the failing test**

Open `internal/service/account_ops_test.go`. Inside `TestCreateAccount`, add a new sub-test after the existing `"repo failure returns error"` sub-test:

```go
t.Run("duplicate account name returns ErrAlreadyExists", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    svc := newTestAccountService(accRepo, newMockTransactionRepo())
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})

    _, err := svc.CreateAccount("Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrAlreadyExists), "expected ErrAlreadyExists, got: %v", err)
})
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/service/ -run TestCreateAccount/duplicate_account_name_returns_ErrAlreadyExists -v
```

Expected: FAIL — the error is `errors.New("account already exists")` from the mock, not `ErrAlreadyExists`.

- [ ] **Step 3: Fix the mock to return `store.ErrAccountExists`**

In `internal/service/testhelper_test.go`, locate `mockAccountRepo.CreateAccount`. The duplicate-check branch currently returns a plain `errors.New`. Change it to wrap `store.ErrAccountExists`:

```go
func (m *mockAccountRepo) CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	if _, exists := m.accountsByName[name]; exists {
		return 0, fmt.Errorf("account %q already exists: %w", name, store.ErrAccountExists)
	}
	id := m.nextID
	m.nextID++
	acc := &model.Account{
		ID:          id,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
	}
	m.accountsByName[name] = acc
	m.accountsByID[id] = acc
	return id, nil
}
```

- [ ] **Step 4: Re-run to confirm still failing (mock now correct, service not yet wrapping)**

```bash
go test ./internal/service/ -run TestCreateAccount/duplicate_account_name_returns_ErrAlreadyExists -v
```

Expected: FAIL — `errors.Is(err, ErrAlreadyExists)` is still false because the service hasn't wrapped it yet.

- [ ] **Step 5: Implement wrapping in `CreateAccount`**

In `internal/service/account_ops.go`, locate the `CreateAccount` method. The section after validation that calls `as.repo.CreateAccount` currently looks like:

```go
newID, err := as.repo.CreateAccount(name, accType, currency, description, parentID)
if err != nil {
    return nil, err
}
```

Replace it with:

```go
newID, err := as.repo.CreateAccount(name, accType, currency, description, parentID)
if err != nil {
    if errors.Is(err, store.ErrAccountExists) {
        return nil, fmt.Errorf("account %q: %w", name, ErrAlreadyExists)
    }
    return nil, err
}
```

(`account_ops.go` already imports `"github.com/hance08/kea/internal/store"` and `"errors"` — no import changes needed.)

- [ ] **Step 6: Run the full `TestCreateAccount` suite to verify it passes**

```bash
go test ./internal/service/ -run TestCreateAccount -v
```

Expected: all sub-tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go internal/service/testhelper_test.go
git commit -m "feat: wrap store.ErrAccountExists as service.ErrAlreadyExists in CreateAccount"
```

---

## Task 3: Wrap `store.ErrRecordNotFound` in `GetAccountByName`

**Files:**
- Modify: `internal/service/account_service.go` (wrapping)
- Modify: `internal/service/account_ops_test.go` (new test)

- [ ] **Step 1: Write the failing test**

In `internal/service/account_ops_test.go`, add a new top-level test function:

```go
func TestGetAccountByName_NotFound(t *testing.T) {
	repo := newMockAccountRepo()
	svc := newTestAccountService(repo, newMockTransactionRepo())

	_, err := svc.GetAccountByName("Assets:Nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/service/ -run TestGetAccountByName_NotFound -v
```

Expected: FAIL — the error wraps `store.ErrRecordNotFound` but not `service.ErrNotFound`.

- [ ] **Step 3: Implement wrapping in `GetAccountByName`**

In `internal/service/account_service.go`, the current method is:

```go
func (as *AccountService) GetAccountByName(name string) (*model.Account, error) {
	return as.repo.GetAccountByName(name)
}
```

Replace it with:

```go
func (as *AccountService) GetAccountByName(name string) (*model.Account, error) {
	acc, err := as.repo.GetAccountByName(name)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, err
	}
	return acc, nil
}
```

- [ ] **Step 4: Add the required imports to `account_service.go`**

`account_service.go` currently does not import `errors` or `internal/store`. Add them. The updated import block:

```go
import (
	"errors"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/utils"
)
```

- [ ] **Step 5: Run the new test and the full service suite**

```bash
go test ./internal/service/ -run TestGetAccountByName_NotFound -v
go test ./internal/service/... -v 2>&1 | tail -20
```

Expected: `TestGetAccountByName_NotFound` PASS; all other tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_service.go internal/service/account_ops_test.go
git commit -m "feat: wrap store.ErrRecordNotFound as service.ErrNotFound in GetAccountByName"
```

---

## Task 4: Add `app.InitLedgerDB` and update `cmd/ledger/add.go`

**Files:**
- Modify: `internal/app/app.go` (new exported function)
- Modify: `cmd/ledger/add.go` (use app helper, drop store import)

- [ ] **Step 1: Add `InitLedgerDB` to `internal/app/app.go`**

Append the following function to the bottom of `internal/app/app.go`:

```go
// InitLedgerDB creates a new SQLite database at path, runs all migrations, then closes it.
// It is used by the ledger add command to initialise a fresh database file.
func InitLedgerDB(path string, migrations fs.FS) error {
	s, err := store.NewStore(path, migrations)
	if err != nil {
		return err
	}
	return s.Close()
}
```

(`app.go` already imports `"io/fs"` and `"github.com/hance08/kea/internal/store"` — no import changes needed.)

- [ ] **Step 2: Update `cmd/ledger/add.go` to call `app.InitLedgerDB`**

In `cmd/ledger/add.go`, replace `defaultDBInit`:

```go
func defaultDBInit(path string, migrations fs.FS) error {
	return app.InitLedgerDB(path, migrations)
}
```

- [ ] **Step 3: Update imports in `cmd/ledger/add.go`**

Remove `"github.com/hance08/kea/internal/store"` and add `"github.com/hance08/kea/internal/app"`.

Full updated import block:

```go
import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hance08/kea/internal/app"
	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 4: Build to verify no compilation errors**

```bash
make build
```

Expected: `./kea_test` binary produced with no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go cmd/ledger/add.go
git commit -m "refactor: expose app.InitLedgerDB to remove store dependency from cmd/ledger"
```

---

## Task 5: Update `cmd/root.go` to use `service.ErrNotFound`

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Replace the three `store.ErrRecordNotFound` references**

In `cmd/root.go`, there are three occurrences — two in `initSysAcc`/`migrateLegacySysAcc` functions:

**Line 145** (`initSysAcc`):
```go
// Before:
if !errors.Is(err, store.ErrRecordNotFound) {
// After:
if !errors.Is(err, service.ErrNotFound) {
```

**Line 172** (`migrateLegacySysAcc`):
```go
// Before:
if errors.Is(err, store.ErrRecordNotFound) {
// After:
if errors.Is(err, service.ErrNotFound) {
```

**Line 184** (`migrateLegacySysAcc`):
```go
// Before:
if !errors.Is(err, store.ErrRecordNotFound) {
// After:
if !errors.Is(err, service.ErrNotFound) {
```

- [ ] **Step 2: Remove `"github.com/hance08/kea/internal/store"` from the import block**

The `store` import in `cmd/root.go` is no longer needed. Remove it. The `service` package is already imported.

- [ ] **Step 3: Build to verify**

```bash
make build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "refactor: replace store.ErrRecordNotFound with service.ErrNotFound in cmd/root.go"
```

---

## Task 6: Update `cmd/account/create.go` to use `service.ErrAlreadyExists`

**Files:**
- Modify: `cmd/account/create.go`

- [ ] **Step 1: Replace `store.ErrAccountExists` with `service.ErrAlreadyExists`**

In `cmd/account/create.go`, line 79:

```go
// Before:
if errors.Is(err, store.ErrAccountExists) {
// After:
if errors.Is(err, service.ErrAlreadyExists) {
```

- [ ] **Step 2: Remove `"github.com/hance08/kea/internal/store"` from the import block**

The `store` import is no longer needed. Remove it. The `service` package is already imported.

- [ ] **Step 3: Build to verify**

```bash
make build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/account/create.go
git commit -m "refactor: replace store.ErrAccountExists with service.ErrAlreadyExists in cmd/account/create.go"
```

---

## Task 7: Final verification

- [ ] **Step 1: Confirm no `internal/store` imports remain in `cmd/`**

```bash
grep -r '"github.com/hance08/kea/internal/store"' cmd/
```

Expected: no output.

- [ ] **Step 2: Run all tests**

```bash
go test ./...
```

Expected: all tests PASS with no failures or compilation errors.

- [ ] **Step 3: Run build one final time**

```bash
make build
```

Expected: `./kea_test` binary produced with no errors.
