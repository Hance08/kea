# Hidden Account Filtering in Service Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move hidden-account filtering from the cmd layer into the service layer so any consumer (CLI, future web API) gets consistent filtering without duplicating logic.

**Architecture:** Add a `ListAccountsOptions` struct to the service layer with `Type` and `ShowHidden` fields. Introduce a single `ListAccounts` method on `AccountService` that delegates to the existing repo methods and applies hidden-account filtering. Update `cmd/account/list.go` to call the new method instead of doing its own filtering. Remove the old `filterHiddenAccounts` and the `AccountListProvider` interface.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add `ListAccounts` to the service layer (with tests)

**Files:**
- Create: `internal/service/account_service_test.go` (append new tests)
- Modify: `internal/service/account_service.go:28-56`

**Context:** Currently `GetAllAccounts` (line 28) and `GetAccountsByType` (line 54) are thin pass-throughs to the repo. We'll add a `ListAccountsOptions` struct and a `ListAccounts` method that combines both call paths + hidden filtering. The existing `GetAllAccounts`/`GetAccountsByType` stay unchanged — other callers may need them.

The mock `mockAccountRepo` in `testhelper_test.go` already implements `GetAllAccounts` (line 92) and `GetAccountsByType` (line 127), and the `addAccount` helper (line 65) lets us inject accounts with `IsHidden: true`. No mock changes needed.

- [ ] **Step 1: Write failing tests for `ListAccounts`**

Add to `internal/service/account_service_test.go`:

```go
func TestListAccounts(t *testing.T) {
	mkAccRepo := func() *mockAccountRepo {
		r := newMockAccountRepo()
		r.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		r.addAccount(&model.Account{ID: 2, Name: "Assets:Hidden", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		r.addAccount(&model.Account{ID: 3, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
		return r
	}

	t.Run("hides hidden accounts by default", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{})

		require.NoError(t, err)
		assert.Len(t, accounts, 2)
		for _, acc := range accounts {
			assert.False(t, acc.IsHidden)
		}
	})

	t.Run("shows hidden accounts when ShowHidden is true", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{ShowHidden: true})

		require.NoError(t, err)
		assert.Len(t, accounts, 3)
	})

	t.Run("filters by type", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())
		at := model.AccountTypeAsset

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{Type: &at})

		require.NoError(t, err)
		assert.Len(t, accounts, 1)
		assert.Equal(t, "Assets:Cash", accounts[0].Name)
	})

	t.Run("filters by type with ShowHidden", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())
		at := model.AccountTypeAsset

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{
			Type:       &at,
			ShowHidden: true,
		})

		require.NoError(t, err)
		assert.Len(t, accounts, 2)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestListAccounts -v`
Expected: compilation error — `ListAccountsOptions` and `ListAccounts` are undefined.

- [ ] **Step 3: Implement `ListAccountsOptions` and `ListAccounts`**

Add to `internal/service/account_service.go` after the existing imports and before `GetAllAccounts`:

```go
type ListAccountsOptions struct {
	Type       *model.AccountType
	ShowHidden bool
}

func (as *AccountService) ListAccounts(ctx context.Context, opts ListAccountsOptions) ([]*model.Account, error) {
	var accounts []*model.Account
	var err error

	if opts.Type != nil {
		accounts, err = as.repo.GetAccountsByType(ctx, *opts.Type)
	} else {
		accounts, err = as.repo.GetAllAccounts(ctx)
	}
	if err != nil {
		return nil, err
	}

	if !opts.ShowHidden {
		filtered := make([]*model.Account, 0, len(accounts))
		for _, acc := range accounts {
			if !acc.IsHidden {
				filtered = append(filtered, acc)
			}
		}
		accounts = filtered
	}

	return accounts, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestListAccounts -v`
Expected: all 4 subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_service.go internal/service/account_service_test.go
git commit -m "feat: add ListAccounts with hidden filtering to AccountService (#108)"
```

---

### Task 2: Update `cmd/account/list.go` to use the new service method

**Files:**
- Modify: `cmd/account/list.go`

**Context:** The `listRunner` currently calls `GetAllAccounts`/`GetAccountsByType` and then `filterHiddenAccounts`. Replace that with a single call to `ListAccounts`. The `AccountListProvider` interface (line 22-27) needs updating to expose `ListAccounts` instead of the two separate methods. The `filterHiddenAccounts` method (line 94-102) can be deleted.

- [ ] **Step 1: Update the `AccountListProvider` interface**

Replace the interface at `cmd/account/list.go:22-27` with:

```go
type AccountListProvider interface {
	ListAccounts(ctx context.Context, opts service.ListAccountsOptions) ([]*model.Account, error)
	GetAccountBalance(ctx context.Context, id int64) (int64, error)
	GetAccountBalanceFormatted(ctx context.Context, id int64) (string, error)
}
```

Add `"github.com/hance08/kea/internal/service"` to the imports (already present).

- [ ] **Step 2: Simplify the `Run` method**

Replace the body of `Run` (lines 59-92) with:

```go
func (r *listRunner) Run(ctx context.Context) error {
	opts := service.ListAccountsOptions{
		ShowHidden: r.flags.ShowHidden,
	}
	if r.flags.Type != "" {
		at := model.AccountType(r.flags.Type)
		opts.Type = &at
	}

	accounts, err := r.svc.ListAccounts(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	if r.flags.JSON {
		items := make([]views.JSONAccount, 0, len(accounts))
		for _, acc := range accounts {
			bal, err := r.svc.GetAccountBalance(ctx, acc.ID)
			if err != nil {
				return fmt.Errorf("failed to get balance for %s: %w", acc.Name, err)
			}
			items = append(items, views.ToJSONAccount(acc, bal))
		}
		return views.WriteJSON(items)
	}
	return views.NewAccountListView().Render(accounts, func(id int64) (string, error) {
		return r.svc.GetAccountBalanceFormatted(ctx, id)
	})
}
```

- [ ] **Step 3: Delete `filterHiddenAccounts`**

Remove lines 94-102 entirely (the `filterHiddenAccounts` method).

- [ ] **Step 4: Verify compilation**

Run: `go build ./cmd/...`
Expected: clean build with no errors.

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/account/list.go
git commit -m "refactor: use service.ListAccounts in cmd/account list (#108)"
```
