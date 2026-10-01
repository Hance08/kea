# Account Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add partial-name search for accounts with pagination, enabling autocomplete in a future web layer.

**Architecture:** New `AccountFilter` model + `SearchAccounts` method threaded through repository → store → service → cmd. Follows the existing `TransactionFilter`/`FilterTransactions` pattern exactly. Case-insensitive substring match via `LOWER(name) LIKE '%' || LOWER(?) || '%'` in SQLite.

**Tech Stack:** Go, SQLite, cobra CLI, pterm views

---

### Task 1: AccountFilter Model

**Files:**
- Create: `internal/model/account_filter.go`

- [ ] **Step 1: Create the AccountFilter struct**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type AccountFilter struct {
	Query    *string
	Type     *AccountType
	Currency *string
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/model/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/model/account_filter.go
git commit -m "feat(model): add AccountFilter struct (#115)"
```

---

### Task 2: Repository Interface

**Files:**
- Modify: `internal/repository/interfaces.go:12-26`

- [ ] **Step 1: Add SearchAccounts to AccountRepository interface**

Add this method to the `AccountRepository` interface, after `UpdateAccountMetadata`:

```go
SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error)
```

- [ ] **Step 2: Verify it compiles (expect failures from Store/mocks not implementing yet)**

Run: `go build ./internal/repository/...`
Expected: compiles (interface only). Store and mocks will fail — that's expected and fixed in later tasks.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/interfaces.go
git commit -m "feat(repository): add SearchAccounts to AccountRepository interface (#115)"
```

---

### Task 3: Store Implementation

**Files:**
- Modify: `internal/store/sqlite_account.go`

- [ ] **Step 1: Write the failing test**

There is no existing `sqlite_account_test.go` — the store layer is tested via integration tests.
Skip to implementation since `SearchAccounts` follows the exact same dynamic-SQL pattern as `FilterTransactions` in `sqlite_transaction.go:579-663`.

- [ ] **Step 2: Implement SearchAccounts on Store**

Add the following method to `internal/store/sqlite_account.go`, after the `UpdateAccountMetadata` method:

```go
func (s *Store) SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts = normalizeListOpts(opts)

	var whereClauses []string
	var args []any

	if filter.Query != nil {
		escaped := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(*filter.Query)
		whereClauses = append(whereClauses, `LOWER(name) LIKE '%' || LOWER(?) || '%' ESCAPE '\'`)
		args = append(args, escaped)
	}
	if filter.Type != nil {
		whereClauses = append(whereClauses, "type = ?")
		args = append(args, string(*filter.Type))
	}
	if filter.Currency != nil {
		whereClauses = append(whereClauses, "currency = ?")
		args = append(args, *filter.Currency)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	result := &model.ListResult[*model.Account]{
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	if opts.IncludeCount {
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM accounts %s", whereSQL)
		if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.TotalCount); err != nil {
			return nil, fmt.Errorf("failed to count search results: %w", err)
		}
	}

	query := fmt.Sprintf(
		"SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts %s ORDER BY name LIMIT ? OFFSET ?",
		whereSQL,
	)
	queryArgs := append(args, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to search accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, err := s.scanAccounts(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*model.Account{}
	}
	result.Items = items
	return result, nil
}
```

Note: add `"strings"` to the import block if not already present.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/store/...`
Expected: compiles (mock not updated yet, but store package builds independently)

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite_account.go
git commit -m "feat(store): implement SearchAccounts with dynamic SQL (#115)"
```

---

### Task 4: Mock + Service Layer

**Files:**
- Modify: `internal/service/testhelper_test.go`
- Modify: `internal/service/account_service.go`

- [ ] **Step 1: Add SearchAccounts to mockAccountRepo**

In `internal/service/testhelper_test.go`, add a new field to `mockAccountRepo`:

```go
searchErr error
```

Add it after the `updateMetadataErr error` field (around line 48).

Then add the mock implementation after the `UpdateAccountMetadata` method (after line 205):

```go
func (m *mockAccountRepo) SearchAccounts(_ context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	var items []*model.Account
	for _, acc := range m.accountsByID {
		if filter.Query != nil && !strings.Contains(strings.ToLower(acc.Name), strings.ToLower(*filter.Query)) {
			continue
		}
		if filter.Type != nil && acc.Type != *filter.Type {
			continue
		}
		if filter.Currency != nil && acc.Currency != *filter.Currency {
			continue
		}
		items = append(items, acc)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	total := len(items)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return &model.ListResult[*model.Account]{
		Items:      items[offset:end],
		TotalCount: total,
		Limit:      limit,
		Offset:     offset,
	}, nil
}
```

- [ ] **Step 2: Verify mocks compile**

Run: `go build ./internal/service/...`
Expected: compiles (the `mockAccountRepo` now satisfies `AccountRepository`)

- [ ] **Step 3: Write the failing service test**

Create a new test file `internal/service/account_search_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestSearchAccounts(t *testing.T) {
	t.Run("filters by query substring", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestAccountService(accRepo, txRepo)

		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank:Savings", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})

		query := "Bank"
		result, err := svc.SearchAccounts(context.Background(), model.AccountFilter{Query: &query}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result.Items))
		}
	})

	t.Run("filters hidden and system accounts by default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestAccountService(accRepo, txRepo)

		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Hidden", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Equity:OpeningBalances_USD", Type: model.AccountTypeEquity, Currency: "USD"})

		query := ""
		result, err := svc.SearchAccounts(context.Background(), model.AccountFilter{Query: &query}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 1 {
			t.Fatalf("expected 1 result (hidden and system excluded), got %d", len(result.Items))
		}
		if result.Items[0].Name != "Assets:Bank:Checking" {
			t.Fatalf("expected Assets:Bank:Checking, got %s", result.Items[0].Name)
		}
	})

	t.Run("returns repo error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestAccountService(accRepo, txRepo)
		accRepo.searchErr = errors.New("db error")

		result, err := svc.SearchAccounts(context.Background(), model.AccountFilter{}, model.ListOptions{Limit: 10})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if result != nil {
			t.Fatalf("expected nil result on error")
		}
	})
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestSearchAccounts -v`
Expected: FAIL — `svc.SearchAccounts` does not exist yet

- [ ] **Step 5: Implement SearchAccounts on AccountService**

Add to `internal/service/account_service.go`, after the `UpdateAccountMetadata` method:

```go
func (as *AccountService) SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error) {
	result, err := as.repo.SearchAccounts(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	filtered := make([]*model.Account, 0, len(result.Items))
	for _, acc := range result.Items {
		if acc.IsHidden || model.IsOpeningBalancesAccount(acc.Name) {
			continue
		}
		filtered = append(filtered, acc)
	}
	result.Items = filtered
	return result, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestSearchAccounts -v`
Expected: PASS (all 3 subtests)

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/service/testhelper_test.go internal/service/account_search_test.go internal/service/account_service.go
git commit -m "feat(service): add SearchAccounts with hidden/system account filtering (#115)"
```

---

### Task 5: CLI Search Command

**Files:**
- Create: `cmd/account/search.go`
- Modify: `cmd/account/account.go`

This command has 5 flags (query, type, currency, limit, json), so it uses **Pattern A** (store whole flags struct in runner).

- [ ] **Step 1: Create the search command**

Create `cmd/account/search.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package account

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type searchFlags struct {
	Query    string
	Type     string
	Currency string
	Limit    int
	JSON     bool
}

type AccountSearchProvider interface {
	SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error)
	GetAccountBalance(ctx context.Context, id int64) (int64, error)
	GetAccountBalanceFormatted(ctx context.Context, id int64) (string, error)
}

type searchRunner struct {
	svc   AccountSearchProvider
	flags *searchFlags
}

func NewSearchCmd(svc *service.Service) *cobra.Command {
	flags := &searchFlags{}

	cmd := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"s", "find"},
		Short:   "Search accounts by partial name match.",
		Long: `Search for accounts whose name contains the given query string.
The search is case-insensitive and matches anywhere in the account name.
Hidden accounts and system accounts are excluded from results.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				flags.Query = args[0]
			}
			runner := &searchRunner{
				svc:   svc.Account(),
				flags: flags,
			}
			return runner.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Filter by account type (A, L, C, R, E)")
	cmd.Flags().StringVarP(&flags.Currency, "currency", "c", "", "Filter by currency code")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "n", 20, "Maximum number of results")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "Output as JSON")

	return cmd
}

func (r *searchRunner) Run(ctx context.Context) error {
	filter := model.AccountFilter{}
	if r.flags.Query != "" {
		filter.Query = &r.flags.Query
	}
	if r.flags.Type != "" {
		at := model.AccountType(r.flags.Type)
		filter.Type = &at
	}
	if r.flags.Currency != "" {
		filter.Currency = &r.flags.Currency
	}

	opts := model.ListOptions{
		Limit:        r.flags.Limit,
		IncludeCount: true,
	}

	result, err := r.svc.SearchAccounts(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("failed to search accounts: %w", err)
	}

	if r.flags.JSON {
		items := make([]views.JSONAccount, 0, len(result.Items))
		for _, acc := range result.Items {
			bal, err := r.svc.GetAccountBalance(ctx, acc.ID)
			if err != nil {
				return fmt.Errorf("failed to get balance for %s: %w", acc.Name, err)
			}
			items = append(items, views.ToJSONAccount(acc, bal))
		}
		return views.WriteJSON(items)
	}

	if len(result.Items) == 0 {
		fmt.Println("No accounts found.")
		return nil
	}

	return views.NewAccountListView().Render(result.Items, func(id int64) (string, error) {
		return r.svc.GetAccountBalanceFormatted(ctx, id)
	})
}
```

- [ ] **Step 2: Register the search subcommand**

In `cmd/account/account.go`, add the new command after the existing `AddCommand` calls:

```go
accountCmd.AddCommand(NewSearchCmd(svc))
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/...`
Expected: compiles

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 5: Commit**

```bash
git add cmd/account/search.go cmd/account/account.go
git commit -m "feat(cmd): add 'account search' command with partial name matching (#115)"
```

---

### Task 6: Integration Verification

- [ ] **Step 1: Build and smoke-test**

Run: `make build`
Expected: builds successfully

- [ ] **Step 2: Run full test suite one final time**

Run: `go test ./...`
Expected: all tests pass
