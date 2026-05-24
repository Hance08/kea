# RenameAccount ExecTx Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `Store.RenameAccount` use the `DBTX` interface so it can participate in `ExecTx` calls, then wrap the service-layer rename in `ExecTx` for atomicity.

**Architecture:** Remove the self-managed transaction from the store method and use `s.db` (the `DBTX` interface) like every other store method. The service layer wraps the rename + read-back in `ExecTx`.

**Tech Stack:** Go, SQLite, testify

---

## File Structure

- **Modify:** `internal/store/sqlite_account.go` — refactor `RenameAccount` to use `s.db`
- **Modify:** `internal/service/account_ops.go` — wrap rename in `ExecTx`
- **Modify:** `internal/store/sqlite_account_test.go` — add composability test
- **Modify:** `internal/service/account_ops_test.go` — verify `ExecTx` is used

---

### Task 1: Refactor store `RenameAccount` to use `s.db`

**Files:**
- Modify: `internal/store/sqlite_account.go:261-298`

- [ ] **Step 1: Write failing store test for ExecTx composability**

Add to `internal/store/sqlite_account_test.go`:

```go
func TestRenameAccount_InsideExecTx(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Assets:Bank:Checking", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		return repo.RenameAccount(ctx, "Assets:Bank", "Assets:CU")
	})
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:CU")
	assert.Contains(t, names, "Assets:CU:Checking")
	assert.NotContains(t, names, "Assets:Bank")
	assert.NotContains(t, names, "Assets:Bank:Checking")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestRenameAccount_InsideExecTx -v`
Expected: FAIL with "store is already in a transaction"

- [ ] **Step 3: Refactor `RenameAccount` to use `s.db`**

Replace `internal/store/sqlite_account.go` lines 261-298 with:

```go
// RenameAccount updates the name of an account and cascades the rename to all descendants.
func (s *Store) RenameAccount(ctx context.Context, oldName, newName string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, err := s.db.ExecContext(ctx, `UPDATE accounts SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return fmt.Errorf("failed to rename account %q: %w", oldName, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account %q not found: %w", oldName, ErrRecordNotFound)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE accounts SET name = ? || substr(name, length(?) + 1)
		 WHERE substr(name, 1, length(? || ':')) = ? || ':'`,
		newName, oldName, oldName, oldName,
	)
	if err != nil {
		return fmt.Errorf("failed to cascade rename from %q to %q: %w", oldName, newName, err)
	}

	return nil
}
```

- [ ] **Step 4: Run all RenameAccount store tests**

Run: `go test ./internal/store/ -run TestRenameAccount -v`
Expected: All 4 tests pass (LIKEWildcardsInName, DeepNesting, SiblingUnaffected, InsideExecTx)

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite_account.go internal/store/sqlite_account_test.go
git commit -m "fix: refactor RenameAccount to use DBTX interface (#110)

Remove self-managed transaction from Store.RenameAccount. Use s.db
(the DBTX interface) so the method can participate in ExecTx calls."
```

---

### Task 2: Wrap service-layer `RenameAccount` in `ExecTx`

**Files:**
- Modify: `internal/service/account_ops.go:255-298`

- [ ] **Step 1: Update `RenameAccount` to use `ExecTx`**

Replace `internal/service/account_ops.go` lines 255-298 with:

```go
func (as *AccountService) RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error) {
	acc, err := as.repo.GetAccountByName(ctx, oldName)
	if err != nil {
		return nil, err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return nil, fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	oldPrefix := ""
	if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
		oldPrefix = acc.Name[:idx+1]
	}

	newPrefix := ""
	segment := newFullName
	if idx := strings.LastIndex(newFullName, ":"); idx >= 0 {
		newPrefix = newFullName[:idx+1]
		segment = newFullName[idx+1:]
	}

	if newPrefix != oldPrefix {
		return nil, validationErrorf("name", "rename cannot change parent path")
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

	var renamed *model.Account
	err = as.tm.ExecTx(ctx, func(repo repository.Repository) error {
		if err := repo.RenameAccount(ctx, acc.Name, newFullName); err != nil {
			return err
		}
		got, err := repo.GetAccountByName(ctx, newFullName)
		if err != nil {
			return err
		}
		renamed = got
		return nil
	})
	if err != nil {
		return nil, err
	}
	return renamed, nil
}
```

- [ ] **Step 2: Run existing service tests**

Run: `go test ./internal/service/ -run TestRenameAccount -v`
Expected: All 8 subtests pass

- [ ] **Step 3: Commit**

```bash
git add internal/service/account_ops.go
git commit -m "fix: wrap service RenameAccount in ExecTx for atomicity (#110)

The rename and read-back now run inside a single transaction,
matching the pattern used by CreateAccountWithBalance."
```

---

### Task 3: Run full test suite

- [ ] **Step 1: Run all tests**

Run: `go test ./...`
Expected: All packages pass

- [ ] **Step 2: Commit if any fixups were needed (skip if clean)**
