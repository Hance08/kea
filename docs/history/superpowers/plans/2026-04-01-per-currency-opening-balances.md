# Per-Currency Opening Balances Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single `Equity:OpeningBalances` system account with per-currency sibling accounts (e.g. `Equity:OpeningBalances_USD`, `Equity:OpeningBalances_TWD`) so that opening balance splits are always recorded in the account's actual currency and each equity account has a meaningful single-currency balance.

**Architecture:** Add two pure helpers to `internal/model/types.go` (`OpeningBalancesAccountName`, `IsOpeningBalancesAccount`), update `createOpeningBalance` to resolve/auto-create the correct per-currency equity account inside the DB transaction, update all guards and init logic to use the new names, and run a one-time startup migration that renames the legacy `Equity:OpeningBalances` to `Equity:OpeningBalances_<default_currency>`.

**Tech Stack:** Go, SQLite (`database/sql`), `golang-migrate`, `testify`

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/model/types.go` | Modify | Remove `SystemAccountOpeningBalance`; add `OpeningBalancesAccountName` and `IsOpeningBalancesAccount` |
| `internal/model/types_test.go` | Create | Tests for the two new helpers |
| `internal/repository/interfaces.go` | Modify | Add `RenameAccount` to `AccountRepository` |
| `internal/store/sqlite_account.go` | Modify | Implement `RenameAccount` |
| `internal/service/testhelper_test.go` | Modify | Add `RenameAccount` to `mockAccountRepo` |
| `internal/service/account_ops.go` | Modify | `createOpeningBalance` uses per-currency account; `DeleteAccountByName` uses `IsOpeningBalancesAccount` |
| `internal/service/account_ops_test.go` | Modify | Update/add tests for new behaviour |
| `cmd/root.go` | Modify | `initSysAcc` creates per-currency account; add `migrateLegacySysAcc` |
| `internal/service/transaction_classifier_test.go` | Modify | Replace hardcoded `"Equity:OpeningBalances"` strings |

---

## Task 1: Add model helpers

**Files:**
- Modify: `internal/model/types.go`
- Create: `internal/model/types_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/model/types_test.go`:

```go
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpeningBalancesAccountName(t *testing.T) {
	assert.Equal(t, "Equity:OpeningBalances_USD", OpeningBalancesAccountName("USD"))
	assert.Equal(t, "Equity:OpeningBalances_TWD", OpeningBalancesAccountName("TWD"))
	assert.Equal(t, "Equity:OpeningBalances_TWD", OpeningBalancesAccountName("twd"))
}

func TestIsOpeningBalancesAccount(t *testing.T) {
	assert.True(t, IsOpeningBalancesAccount("Equity:OpeningBalances_USD"))
	assert.True(t, IsOpeningBalancesAccount("Equity:OpeningBalances_TWD"))
	assert.False(t, IsOpeningBalancesAccount("Equity:OpeningBalances"))
	assert.False(t, IsOpeningBalancesAccount("Equity:Retained"))
	assert.False(t, IsOpeningBalancesAccount(""))
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/model/... -run "TestOpeningBalancesAccountName|TestIsOpeningBalancesAccount" -v
```

Expected: `FAIL` — functions not defined yet.

- [ ] **Step 3: Add the helpers to `internal/model/types.go`; remove the old constant**

In `internal/model/types.go`, remove:
```go
SystemAccountOpeningBalance = "Equity:OpeningBalances"
```

Add at the bottom of the file (after existing constants):
```go
func OpeningBalancesAccountName(currency string) string {
	return "Equity:OpeningBalances_" + strings.ToUpper(currency)
}

func IsOpeningBalancesAccount(name string) bool {
	return strings.HasPrefix(name, "Equity:OpeningBalances_")
}
```

Add `"strings"` to the import block in `internal/model/types.go`:
```go
import "strings"
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/model/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Fix compilation errors caused by removing `SystemAccountOpeningBalance`**

The constant is referenced in three files. Temporarily replace each reference with the literal string `"Equity:OpeningBalances"` (we will fix them properly in later tasks):

`cmd/root.go:104`:
```go
_, err := svc.Account().GetAccountByName("Equity:OpeningBalances")
```

`cmd/root.go:113`:
```go
"Equity:OpeningBalances",
```

`internal/service/account_ops.go:57`:
```go
openingBalanceAccount, err := as.repo.GetAccountByName("Equity:OpeningBalances")
```

`internal/service/account_ops.go:59`:
```go
return fmt.Errorf("can not find %q account, failed to set initial balance", "Equity:OpeningBalances")
```

`internal/service/account_ops.go:96`:
```go
if acc.Name == "Equity:OpeningBalances" {
```

- [ ] **Step 6: Confirm project compiles and all tests pass**

```bash
go build ./... && go test ./...
```

Expected: no compile errors, all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/model/types.go internal/model/types_test.go cmd/root.go internal/service/account_ops.go
git commit -m "feat: add OpeningBalancesAccountName and IsOpeningBalancesAccount helpers"
```

---

## Task 2: Add `RenameAccount` to repository interface and store

**Files:**
- Modify: `internal/repository/interfaces.go`
- Modify: `internal/store/sqlite_account.go`
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Check the current `AccountRepository` interface**

Read `internal/repository/interfaces.go` and find the `AccountRepository` interface. Add `RenameAccount` to it:

```go
RenameAccount(oldName, newName string) error
```

- [ ] **Step 2: Implement `RenameAccount` in the store**

At the bottom of `internal/store/sqlite_account.go`, add:

```go
func (s *Store) RenameAccount(oldName, newName string) error {
	res, err := s.db.Exec(
		`UPDATE accounts SET name = ? WHERE name = ?`,
		newName, oldName,
	)
	if err != nil {
		return fmt.Errorf("failed to rename account %q to %q: %w", oldName, newName, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("account %q not found", oldName)
	}
	return nil
}
```

- [ ] **Step 3: Add `RenameAccount` to `mockAccountRepo` in `internal/service/testhelper_test.go`**

Add the method after `DeleteAccount`:

```go
func (m *mockAccountRepo) RenameAccount(oldName, newName string) error {
	acc, ok := m.accountsByName[oldName]
	if !ok {
		return fmt.Errorf("account %q not found", oldName)
	}
	delete(m.accountsByName, oldName)
	acc.Name = newName
	m.accountsByName[newName] = acc
	return nil
}
```

- [ ] **Step 4: Confirm project compiles and all tests pass**

```bash
go build ./... && go test ./...
```

Expected: no errors, all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/interfaces.go internal/store/sqlite_account.go internal/service/testhelper_test.go
git commit -m "feat: add RenameAccount to AccountRepository interface and SQLite store"
```

---

## Task 3: Update `createOpeningBalance` to use per-currency equity account

**Files:**
- Modify: `internal/service/account_ops.go`
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write the new failing tests in `account_ops_test.go`**

Add a new test block after `TestCreateAccountWithBalance_SplitDirection`. The existing helper `addOpeningBalanceAccount` injects the old `Equity:OpeningBalances` — we will use a new helper instead.

Add at the top of the test file, near `addOpeningBalanceAccount`:

```go
func addOpeningBalancesForCurrency(accRepo *mockAccountRepo, currency string) {
	accRepo.addAccount(&model.Account{
		ID:       99,
		Name:     model.OpeningBalancesAccountName(currency),
		Type:     model.AccountTypeEquity,
		Currency: currency,
	})
}
```

Add a new test block:

```go
func TestCreateAccountWithBalance_CurrencyRouting(t *testing.T) {
	t.Run("account currency TWD uses Equity:OpeningBalances_TWD", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalancesForCurrency(accRepo, "TWD")
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance("Assets:TWDBank", model.AccountTypeAsset, "TWD", "", nil, 30000)
		require.NoError(t, err)

		splits := txRepo.splits[1]
		require.Len(t, splits, 2)

		for _, s := range splits {
			assert.Equal(t, "TWD", s.Currency, "all splits must use TWD")
		}

		var assetSplit *model.Split
		for _, s := range splits {
			if s.AccountID == acc.ID {
				assetSplit = s
			}
		}
		require.NotNil(t, assetSplit)
		assert.Equal(t, int64(30000), assetSplit.Amount)
	})

	t.Run("account with empty currency falls back to system default (USD)", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalancesForCurrency(accRepo, "USD")
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance("Assets:Bank", model.AccountTypeAsset, "", "", nil, 5000)
		require.NoError(t, err)

		splits := txRepo.splits[1]
		require.Len(t, splits, 2)
		for _, s := range splits {
			assert.Equal(t, "USD", s.Currency)
		}
		_ = acc
	})

	t.Run("auto-creates Equity:OpeningBalances_TWD when missing", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		// no TWD equity account pre-seeded
		svc := newTestAccountService(accRepo, txRepo)

		_, err := svc.CreateAccountWithBalance("Assets:TWDBank", model.AccountTypeAsset, "TWD", "", nil, 30000)
		require.NoError(t, err)

		// equity account should now exist
		equityName := model.OpeningBalancesAccountName("TWD")
		equityAcc, err := accRepo.GetAccountByName(equityName)
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeEquity, equityAcc.Type)
		assert.Equal(t, "TWD", equityAcc.Currency)
	})
}
```

- [ ] **Step 2: Run new tests to confirm they fail**

```bash
go test ./internal/service/... -run TestCreateAccountWithBalance_CurrencyRouting -v
```

Expected: FAIL — behaviour not implemented yet.

- [ ] **Step 3: Rewrite `createOpeningBalance` in `internal/service/account_ops.go`**

Replace the entire function (lines 54–88) with:

```go
func (as *AccountService) createOpeningBalance(account *model.Account, amountInCents int64) error {
	currency := account.Currency
	if currency == "" {
		currency = as.config.Defaults.Currency
	}

	equityAccountName := model.OpeningBalancesAccountName(currency)

	var balanceAmount, equityAmount int64
	switch account.Type {
	case model.AccountTypeAsset:
		balanceAmount = amountInCents
		equityAmount = -amountInCents
	case model.AccountTypeLiability:
		balanceAmount = -amountInCents
		equityAmount = amountInCents
	default:
		return fmt.Errorf("only Assets(A) and Liabilities(L) accounts can set a balance")
	}

	tx := model.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: model.OpeningAccountMemo,
		Status:      model.StatusCleared,
	}

	return as.tm.ExecTx(context.Background(), func(repo repository.Repository) error {
		equityAcc, err := repo.GetAccountByName(equityAccountName)
		if err != nil {
			newID, err := repo.CreateAccount(
				equityAccountName,
				model.AccountTypeEquity,
				currency,
				"Opening Balances (System Account)",
				nil,
			)
			if err != nil {
				return fmt.Errorf("failed to create %q: %w", equityAccountName, err)
			}
			equityAcc = &model.Account{ID: newID}
		}

		splits := []model.Split{
			{AccountID: account.ID, Amount: balanceAmount, Currency: currency, Memo: model.OpeningAccountMemo},
			{AccountID: equityAcc.ID, Amount: equityAmount, Currency: currency, Memo: model.OpeningAccountMemo},
		}
		_, err = repo.CreateTransactionWithSplits(tx, splits)
		return err
	})
}
```

Also remove the now-unused literal `"Equity:OpeningBalances"` from lines 57–59 (the old lookup that was temporarily left in Step 5 of Task 1). The function no longer references that string.

- [ ] **Step 4: Run all service tests**

```bash
go test ./internal/service/... -v
```

Expected: all tests PASS including the new `TestCreateAccountWithBalance_CurrencyRouting` cases.

- [ ] **Step 5: Update the existing helper and one stale test name in `account_ops_test.go`**

The original `addOpeningBalanceAccount` helper is still used by `TestCreateAccountWithBalance_SplitDirection`. Update it so it seeds the correctly named account:

```go
func addOpeningBalanceAccount(accRepo *mockAccountRepo) {
	accRepo.addAccount(&model.Account{
		ID:       99,
		Name:     model.OpeningBalancesAccountName("USD"),
		Type:     model.AccountTypeEquity,
		Currency: "USD",
	})
}
```

Also update the test name string in `t.Run("missing Equity:OpeningBalances account returns error", ...)` to `"missing Equity:OpeningBalances_USD account returns error"`.

- [ ] **Step 6: Run all tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "fix: createOpeningBalance uses account currency, auto-creates per-currency equity account"
```

---

## Task 4: Update `DeleteAccountByName` protection guard

**Files:**
- Modify: `internal/service/account_ops.go`
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write a new failing test**

In `account_ops_test.go`, inside `TestDeleteAccountByName`, add:

```go
t.Run("any Equity:OpeningBalances_* account is rejected", func(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 10, Name: "Equity:OpeningBalances_TWD", Type: model.AccountTypeEquity})
	svc := newTestAccountService(accRepo, newMockTransactionRepo())

	err := svc.DeleteAccountByName("Equity:OpeningBalances_TWD")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotEditable))
})
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/service/... -run "TestDeleteAccountByName/any_Equity" -v
```

Expected: FAIL — the guard still checks the old literal name.

- [ ] **Step 3: Update the guard in `account_ops.go`**

Replace (lines ~95–97):
```go
if acc.Name == "Equity:OpeningBalances" {
    return fmt.Errorf("account %q is a system account and cannot be deleted: %w", acc.Name, ErrNotEditable)
}
```

With:
```go
if model.IsOpeningBalancesAccount(acc.Name) {
    return fmt.Errorf("account %q is a system account and cannot be deleted: %w", acc.Name, ErrNotEditable)
}
```

- [ ] **Step 4: Also update the existing guard test**

In `TestDeleteAccountByName`, update `"system account Equity:OpeningBalances rejected"`:

```go
t.Run("system account Equity:OpeningBalances_USD rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    addOpeningBalanceAccount(accRepo) // seeds Equity:OpeningBalances_USD
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    err := svc.DeleteAccountByName(model.OpeningBalancesAccountName("USD"))
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotEditable))
})
```

- [ ] **Step 5: Run all tests**

```bash
go test ./internal/service/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "fix: protect all Equity:OpeningBalances_* accounts from deletion"
```

---

## Task 5: Update `initSysAcc` and add startup migration in `cmd/root.go`

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Update `initSysAcc` to create the per-currency account**

Replace the body of `initSysAcc` (lines 103–124):

```go
func initSysAcc(svc *service.Service, cfg *config.Config) error {
	if err := migrateLegacySysAcc(svc, cfg); err != nil {
		return err
	}

	targetName := model.OpeningBalancesAccountName(cfg.Defaults.Currency)
	_, err := svc.Account().GetAccountByName(targetName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrRecordNotFound) {
		return fmt.Errorf("failed to check system account: %w", err)
	}

	_, err = svc.Account().CreateAccount(
		targetName,
		model.AccountTypeEquity,
		cfg.Defaults.Currency,
		"Opening Balances (System Account)",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create system account: %w", err)
	}

	return nil
}
```

- [ ] **Step 2: Add `migrateLegacySysAcc` below `initSysAcc`**

```go
// migrateLegacySysAcc renames the legacy "Equity:OpeningBalances" account to
// "Equity:OpeningBalances_<currency>" the first time a user upgrades.
// It is a no-op when the legacy account does not exist.
func migrateLegacySysAcc(svc *service.Service, cfg *config.Config) error {
	const legacyName = "Equity:OpeningBalances"
	targetName := model.OpeningBalancesAccountName(cfg.Defaults.Currency)

	// Nothing to migrate if legacy account is already gone.
	_, err := svc.Account().GetAccountByName(legacyName)
	if errors.Is(err, store.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check legacy system account: %w", err)
	}

	// Target already exists — legacy was already migrated.
	_, err = svc.Account().GetAccountByName(targetName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrRecordNotFound) {
		return fmt.Errorf("failed to check target system account: %w", err)
	}

	if err := svc.Account().RenameAccount(legacyName, targetName); err != nil {
		return fmt.Errorf("failed to migrate legacy system account: %w", err)
	}

	return nil
}
```

- [ ] **Step 3: Expose `RenameAccount` through `AccountService`**

Add a thin wrapper to `internal/service/account_service.go`:

```go
func (as *AccountService) RenameAccount(oldName, newName string) error {
	return as.repo.RenameAccount(oldName, newName)
}
```

- [ ] **Step 4: Confirm project compiles and all tests pass**

```bash
go build ./... && go test ./...
```

Expected: no errors, all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go internal/service/account_service.go
git commit -m "feat: initSysAcc creates per-currency account; add startup migration for legacy Equity:OpeningBalances"
```

---

## Task 6: Update classifier test strings

**Files:**
- Modify: `internal/service/transaction_classifier_test.go`

- [ ] **Step 1: Replace hardcoded `"Equity:OpeningBalances"` in classifier tests**

In `internal/service/transaction_classifier_test.go`:

Line 78 — update the split helper call:
```go
splitWithMemo(model.OpeningBalancesAccountName("USD"), model.AccountTypeEquity, -5000, model.OpeningAccountMemo),
```

Lines 274 and 279 — update the account setup and split:
```go
accRepo.addAccount(&model.Account{ID: 2, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
```
```go
{AccountName: model.OpeningBalancesAccountName("USD"), Amount: -5000},
```

- [ ] **Step 2: Run all tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/service/transaction_classifier_test.go
git commit -m "test: update classifier tests to use OpeningBalancesAccountName helper"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Opening balance splits use `account.Currency` | Task 3 |
| Fall back to system default when currency is empty | Task 3 |
| Per-currency equity accounts are single-currency | Task 3 |
| Auto-create equity account when missing (no user prompt) | Task 3 |
| All `Equity:OpeningBalances_*` accounts are undeletable | Task 4 |
| `initSysAcc` creates `Equity:OpeningBalances_<currency>` | Task 5 |
| Startup migration renames legacy account | Task 5 |
| Migration is no-op on second run | Task 5 (both checks in `migrateLegacySysAcc`) |
| `OpeningBalancesAccountName` helper | Task 1 |
| `IsOpeningBalancesAccount` helper | Task 1 |
| `RenameAccount` in repository and store | Task 2 |

All spec requirements are covered. No gaps found.
