# Return Updated Entity from Mutation Methods — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Change `RenameAccount` to accept a full new name (not a segment) and return `*model.Account`; change `UpdateAccountMetadata` to return `*model.Account`. Update all callers and tests.

**Architecture:** Modify the two service methods in-place, re-fetch the account after successful repo mutation to return updated state. Update three caller interfaces (`EditProvider`, `accountMigrator`, and their test mocks) plus the CLI call sites.

**Tech Stack:** Go, testify (require/assert)

---

### Task 1: Update `RenameAccount` signature and logic

**Files:**
- Modify: `internal/service/account_ops.go:209-239`

- [ ] **Step 1: Change `RenameAccount` to accept full new name and return `*model.Account`**

Replace the method at lines 209-239 with:

```go
func (as *AccountService) RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error) {
	acc, err := as.repo.GetAccountByName(ctx, oldName)
	if err != nil {
		return nil, err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return nil, fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	segment := newFullName
	if idx := strings.LastIndex(newFullName, ":"); idx >= 0 {
		segment = newFullName[idx+1:]
	}

	if err := as.ValidateAccountName(segment); err != nil {
		return nil, validationWrap("name", "invalid account name", err)
	}

	exists, err := as.repo.AccountExists(ctx, newFullName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, validationErrorf("name", "account %q already exists", newFullName)
	}

	if err := as.repo.RenameAccount(ctx, acc.Name, newFullName); err != nil {
		return nil, err
	}

	return as.repo.GetAccountByName(ctx, newFullName)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/service/...`
Expected: compilation errors from callers (tests, cmd layer) — that's fine, we fix those next.

- [ ] **Step 3: Commit**

```bash
git add internal/service/account_ops.go
git commit -m "refactor: change RenameAccount to accept full name and return *model.Account (#105)"
```

---

### Task 2: Update `UpdateAccountMetadata` signature

**Files:**
- Modify: `internal/service/account_service.go:117-128`

- [ ] **Step 1: Change `UpdateAccountMetadata` to return `*model.Account`**

Replace the method at lines 117-128 with:

```go
func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) (*model.Account, error) {
	acc, err := as.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return nil, fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	if err := as.repo.UpdateAccountMetadata(ctx, accountID, description, isHidden); err != nil {
		return nil, err
	}

	return as.repo.GetAccountByID(ctx, accountID)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/account_service.go
git commit -m "refactor: change UpdateAccountMetadata to return *model.Account (#105)"
```

---

### Task 3: Update `RenameAccount` tests

**Files:**
- Modify: `internal/service/account_ops_test.go:575-670`

All tests must change from `err := svc.RenameAccount(...)` to `acc, err := svc.RenameAccount(...)` (or `_, err :=` for error cases). Success cases must assert the returned account has the correct new name. The second argument changes from a segment to a full name.

- [ ] **Step 1: Update "leaf account renamed" test (line 576)**

```go
t.Run("leaf account renamed with correct full name constructed", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    got, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:Savings")
    require.NoError(t, err)

    require.Len(t, accRepo.renameCalls, 1)
    assert.Equal(t, "Assets:Bank", accRepo.renameCalls[0].old)
    assert.Equal(t, "Assets:Savings", accRepo.renameCalls[0].new)

    assert.Equal(t, "Assets:Savings", got.Name)
    assert.Equal(t, int64(1), got.ID)
})
```

- [ ] **Step 2: Update "repo called with old full name" test (line 589)**

```go
t.Run("repo called with old full name and new full name", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    got, err := svc.RenameAccount(context.Background(), "Assets:Bank:Checking", "Assets:Bank:Current")
    require.NoError(t, err)

    require.Len(t, accRepo.renameCalls, 1)
    assert.Equal(t, "Assets:Bank:Checking", accRepo.renameCalls[0].old)
    assert.Equal(t, "Assets:Bank:Current", accRepo.renameCalls[0].new)

    assert.Equal(t, "Assets:Bank:Current", got.Name)
})
```

- [ ] **Step 3: Update "segment containing colon rejected" test (line 602)**

The new full name `"Assets:Bad:Name"` has a valid structure (colon is allowed in the full path). We need a segment that itself contains a colon after the last `:` — but that's impossible with a full name. Instead, this test validates that the extracted leaf segment is valid. A name like `"Assets:Bank:"` has an empty trailing segment:

```go
t.Run("empty trailing segment rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    _, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:")
    require.Error(t, err)
    assert.Empty(t, accRepo.renameCalls)
})
```

- [ ] **Step 4: Update "empty segment rejected" test (line 612)**

```go
t.Run("empty name rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    _, err := svc.RenameAccount(context.Background(), "Assets:Bank", "")
    require.Error(t, err)
    assert.Empty(t, accRepo.renameCalls)
})
```

- [ ] **Step 5: Update "new full name already exists" test (line 622)**

```go
t.Run("new full name already exists is rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Savings", Type: model.AccountTypeAsset})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    _, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:Savings")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "already exists")
    assert.Empty(t, accRepo.renameCalls)
})
```

- [ ] **Step 6: Update "system account is rejected" test (line 634)**

```go
t.Run("system account is rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    addOpeningBalanceAccount(accRepo)
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    _, err := svc.RenameAccount(context.Background(), model.OpeningBalancesAccountName("USD"), "Equity:Other")
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotEditable))
    assert.Empty(t, accRepo.renameCalls)
})
```

- [ ] **Step 7: Update "account not found" test (line 645)**

```go
t.Run("account not found returns error", func(t *testing.T) {
    svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
    _, err := svc.RenameAccount(context.Background(), "Assets:Ghost", "Assets:NewName")
    require.Error(t, err)
})
```

- [ ] **Step 8: Update "legacy opening-balances" test (line 651)**

```go
t.Run("legacy opening-balances account renamed to currency-suffixed full path", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{
        ID:   1,
        Name: model.LegacyOpeningBalancesName,
        Type: model.AccountTypeEquity,
    })
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    got, err := svc.RenameAccount(context.Background(), model.LegacyOpeningBalancesName, model.OpeningBalancesAccountName("USD"))
    require.NoError(t, err)

    require.Len(t, accRepo.renameCalls, 1)
    assert.Equal(t, model.LegacyOpeningBalancesName, accRepo.renameCalls[0].old)
    assert.Equal(t, model.OpeningBalancesAccountName("USD"), accRepo.renameCalls[0].new)

    assert.Equal(t, model.OpeningBalancesAccountName("USD"), got.Name)
})
```

- [ ] **Step 9: Run all rename tests**

Run: `go test ./internal/service/ -run TestRenameAccount -v`
Expected: all 8 tests pass.

- [ ] **Step 10: Commit**

```bash
git add internal/service/account_ops_test.go
git commit -m "test: update RenameAccount tests for new full-name signature (#105)"
```

---

### Task 4: Update `UpdateAccountMetadata` tests

**Files:**
- Modify: `internal/service/account_ops_test.go:676-718`

- [ ] **Step 1: Update "description and hidden updated correctly" test (line 677)**

```go
t.Run("description and hidden updated correctly", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Description: "old desc", IsHidden: false})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    got, err := svc.UpdateAccountMetadata(context.Background(), 1, "new desc", true)
    require.NoError(t, err)

    require.Len(t, accRepo.updateMetadataCalls, 1)
    call := accRepo.updateMetadataCalls[0]
    assert.Equal(t, int64(1), call.id)
    assert.Equal(t, "new desc", call.description)
    assert.True(t, call.isHidden)

    assert.Equal(t, "new desc", got.Description)
    assert.True(t, got.IsHidden)
    assert.Equal(t, int64(1), got.ID)
})
```

- [ ] **Step 2: Update "system account is rejected" test (line 692)**

```go
t.Run("system account is rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 99, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    _, err := svc.UpdateAccountMetadata(context.Background(), 99, "desc", false)
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotEditable))
    assert.Empty(t, accRepo.updateMetadataCalls)
})
```

- [ ] **Step 3: Update "account not found" test (line 703)**

```go
t.Run("account not found returns error", func(t *testing.T) {
    svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
    _, err := svc.UpdateAccountMetadata(context.Background(), 999, "desc", false)
    require.Error(t, err)
})
```

- [ ] **Step 4: Update "repo error propagated" test (line 709)**

```go
t.Run("repo error propagated", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
    accRepo.updateMetadataErr = errors.New("db error")
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    _, err := svc.UpdateAccountMetadata(context.Background(), 1, "desc", false)
    require.Error(t, err)
})
```

- [ ] **Step 5: Run all metadata tests**

Run: `go test ./internal/service/ -run TestUpdateAccountMetadata -v`
Expected: all 4 tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_ops_test.go
git commit -m "test: update UpdateAccountMetadata tests for *model.Account return (#105)"
```

---

### Task 5: Update caller interfaces and CLI call sites

**Files:**
- Modify: `cmd/account/edit_types.go:16-17`
- Modify: `cmd/account/edit_actions.go:149-178`
- Modify: `cmd/root.go:183,225`
- Modify: `cmd/root_test.go:57-67`

- [ ] **Step 1: Update `EditProvider` interface in `edit_types.go`**

Change lines 16-17 from:

```go
RenameAccount(ctx context.Context, oldName, newSegment string) error
UpdateAccountMetadata(ctx context.Context, id int64, description string, isHidden bool) error
```

to:

```go
RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error)
UpdateAccountMetadata(ctx context.Context, id int64, description string, isHidden bool) (*model.Account, error)
```

- [ ] **Step 2: Update `applyChanges` in `edit_actions.go`**

Replace the method at lines 149-178 with:

```go
func (r *editRunner) applyChanges(ctx context.Context, acc *model.Account, input editInput) (string, error) {
	finalName := acc.Name

	if input.newName != nil {
		var newFullName string
		if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
			newFullName = acc.Name[:idx+1] + *input.newName
		} else {
			newFullName = *input.newName
		}
		renamed, err := r.svc.RenameAccount(ctx, acc.Name, newFullName)
		if err != nil {
			return "", fmt.Errorf("failed to rename account: %w", err)
		}
		finalName = renamed.Name
		acc = renamed
	}

	if input.description != nil || input.isHidden != nil {
		desc := acc.Description
		hidden := acc.IsHidden
		if input.description != nil {
			desc = *input.description
		}
		if input.isHidden != nil {
			hidden = *input.isHidden
		}
		if _, err := r.svc.UpdateAccountMetadata(ctx, acc.ID, desc, hidden); err != nil {
			return "", fmt.Errorf("failed to update account: %w", err)
		}
	}

	return finalName, nil
}
```

- [ ] **Step 3: Update `accountMigrator` interface in `root.go`**

Change line 183 from:

```go
RenameAccount(ctx context.Context, oldName, newSegment string) error
```

to:

```go
RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error)
```

- [ ] **Step 4: Update `migrateLegacySysAccWith` call in `root.go`**

Change line 225 from:

```go
if err := acc.RenameAccount(ctx, model.LegacyOpeningBalancesName, leafSegment); err != nil {
```

to:

```go
if _, err := acc.RenameAccount(ctx, model.LegacyOpeningBalancesName, fullTargetName); err != nil {
```

Also remove the now-unused `leafSegment` variable (lines 196-197). The `idx` and `leafSegment` lines can be deleted since `fullTargetName` is passed directly.

- [ ] **Step 5: Update `mockAccountMigrator.RenameAccount` in `root_test.go`**

Change the mock method (line 57) from:

```go
func (m *mockAccountMigrator) RenameAccount(_ context.Context, oldName, newSegment string) error {
	if m.renameErr != nil {
		return m.renameErr
	}
	m.renameCalls = append(m.renameCalls, struct{ old, new string }{oldName, newSegment})
	acc := m.accounts[oldName]
	delete(m.accounts, oldName)
	acc.Name = newSegment
	m.accounts[newSegment] = acc
	return nil
}
```

to:

```go
func (m *mockAccountMigrator) RenameAccount(_ context.Context, oldName, newFullName string) (*model.Account, error) {
	if m.renameErr != nil {
		return nil, m.renameErr
	}
	m.renameCalls = append(m.renameCalls, struct{ old, new string }{oldName, newFullName})
	acc := m.accounts[oldName]
	delete(m.accounts, oldName)
	acc.Name = newFullName
	m.accounts[newFullName] = acc
	return acc, nil
}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/account/edit_types.go cmd/account/edit_actions.go cmd/root.go cmd/root_test.go
git commit -m "refactor: update callers for new RenameAccount/UpdateAccountMetadata signatures (#105)"
```

---

### Task 6: Update `root_test.go` assertions for full-name calls

**Files:**
- Modify: `cmd/root_test.go`

The existing `migrateLegacySysAcc` tests assert `renameCalls` with segment values. After the signature change, they should assert full names instead.

- [ ] **Step 1: Find and update rename assertions in root_test.go**

Any assertion like:

```go
assert.Equal(t, "OpeningBalances_USD", m.renameCalls[0].new)
```

should become:

```go
assert.Equal(t, "Equity:OpeningBalances_USD", m.renameCalls[0].new)
```

And any assertion checking the mock's account map by segment key should use the full name key.

- [ ] **Step 2: Run root tests**

Run: `go test ./cmd/ -run TestMigrateLegacy -v`
Expected: all migration tests pass.

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: all tests pass — no compilation errors, no test failures.

- [ ] **Step 4: Commit**

```bash
git add cmd/root_test.go
git commit -m "test: fix migration test assertions for full-name RenameAccount (#105)"
```
