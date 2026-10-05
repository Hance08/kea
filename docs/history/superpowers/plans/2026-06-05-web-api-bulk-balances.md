# `GET /api/balances` — Bulk Account-Balance Snapshot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `GET /api/balances` so the SPA dashboard can fetch every account's metadata + point-in-time balance in one round-trip, with `as_of`-default-now, `include_hidden`-default-false, no pagination, no embedded totals.

**Architecture:** New model type `AccountBalance` + new service method `AccountService.GetAccountBalancesBulk(ctx, asOf, includeHidden)` that composes `GetAllAccounts` and `GetAllAccountBalances` and normalizes currency. New handler `handleListBalances` parses query params via existing `parseInt64Query`/`parseBoolQuery` helpers, wraps the result as `ListResult[AccountBalance]`. One-line route registration in `router.go`.

**Tech Stack:** Go stdlib (`context`, `time`, `sort`), `gopkg.in/yaml.v3` (no), `chi/v5` (existing router), `testify` (existing tests).

**Spec:** [`docs/superpowers/specs/2026-06-05-web-api-bulk-balances-design.md`](../specs/2026-06-05-web-api-bulk-balances-design.md)

---

## File Map

- Modify: `internal/model/account.go` — append `AccountBalance` struct next to `AccountNode`.
- Modify: `internal/service/account_service.go` — append `GetAccountBalancesBulk` after `GetAccountBalanceFormatted` (around line 167).
- Modify: `internal/service/account_service_test.go` — append `TestGetAccountBalancesBulk`.
- Modify: `internal/api/accounts.go` — append `handleListBalances` after `handleAccountBalance` (around line 73).
- Modify: `internal/api/accounts_test.go` — append the new `TestHandleListBalances_*` test family.
- Modify: `internal/api/router.go` — add one route line after `/accounts/{id}/balance` (line 32).

No new files. Imports: `sort` and `time` are new for `account_service.go` (verify before adding); `time` may already exist in `accounts.go`.

---

### Task 1: Add `AccountBalance` model

**Files:**
- Modify: `internal/model/account.go` — append after the `AccountNode` struct.

- [ ] **Step 1: Append the new type to `internal/model/account.go`**

Read the file first to confirm current shape. The current file (per spec) ends with:

```go
type AccountNode struct {
	Account  *Account       `json:"account"`
	Children []*AccountNode `json:"children"`
}
```

Append (preserve existing blank line conventions):

```go

// AccountBalance is one row in a bulk-balance snapshot response.
// Joins per-account metadata (from Account) with a point-in-time balance.
type AccountBalance struct {
	AccountID int64       `json:"account_id"`
	Name      string      `json:"name"`
	Type      AccountType `json:"type"`
	ParentID  *int64      `json:"parent_id,omitempty"`
	Currency  string      `json:"currency"`
	Amount    int64       `json:"amount"`
	IsHidden  bool        `json:"is_hidden"`
}
```

`Amount` is signed int64 cents — same convention as `model.Account` balances elsewhere in the codebase. `Currency` is the resolved currency (account's own if set, else config default — populated by the service, not the model).

- [ ] **Step 2: Verify the file builds**

Run: `go build ./internal/model/`

Expected: no output, exit 0. If `AccountType` is missing from imports or unresolved, the build will fail — but `AccountType` is defined in the same package (`internal/model/account.go`), so this should just compile.

- [ ] **Step 3: Verify the JSON shape with a quick scratch decode (manual sanity)**

Optional but useful: `go vet ./internal/model/` — vet should be silent.

---

### Task 2: Write the failing service-layer test

**Files:**
- Modify: `internal/service/account_service_test.go` — append `TestGetAccountBalancesBulk` at the end of the file.

Context: existing tests use `mockAccountRepo` from `testhelper_test.go` and the `newTestAccountService(...)` factory. The mock implements `GetAllAccounts(ctx) ([]*model.Account, error)` and `GetAllAccountBalances(ctx, asOf int64) (map[int64]int64, error)` (already at [internal/service/testhelper_test.go:150](../../internal/service/testhelper_test.go)). The service's `*config.Config` provides `cfg.Defaults.Currency`.

- [ ] **Step 1: Read `internal/service/testhelper_test.go` to confirm mock surface**

Specifically confirm:
- `newMockAccountRepo()` exists and returns `*mockAccountRepo`.
- The mock has hooks for seeding accounts (look for a `getAllAccountsResult` or similar field, or a method like `setAccounts(...)`).
- `GetAllAccountBalances` accepts an `asOf` parameter (signature must match `(_ context.Context, _ int64) (map[int64]int64, error)` per the grep already done).
- `newTestAccountService(accRepo *mockAccountRepo)` or similar factory exists, and how `cfg.Defaults.Currency` is set in `defaultConfig()`.

If the mock surface doesn't expose seedable accounts/balances, you may need a one-line addition to the mock (e.g., a `accountsResult []*model.Account` field). Read the mock first; report back with NEEDS_CONTEXT if extending the mock is non-trivial.

- [ ] **Step 2: Append the test function**

Replace the placeholder paths below with whatever the actual mock seeding API is. The conceptual shape:

```go
func TestGetAccountBalancesBulk(t *testing.T) {
	t.Run("hidden excluded by default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.accountsResult = []*model.Account{
			{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: false},
			{ID: 2, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: false},
			{ID: 3, Name: "Assets:Old",  Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true},
			{ID: 4, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD", IsHidden: false},
		}
		accRepo.balancesResult = map[int64]int64{1: 125000, 2: 3500, 3: 9999, 4: -2000}
		svc := newTestAccountService(accRepo)

		rows, err := svc.GetAccountBalancesBulk(context.Background(), 1700000000, false)
		require.NoError(t, err)
		require.Len(t, rows, 3, "hidden account should be excluded")
		assert.Equal(t, int64(1), rows[0].AccountID, "rows should be sorted by AccountID")
		assert.Equal(t, int64(2), rows[1].AccountID)
		assert.Equal(t, int64(4), rows[2].AccountID)
		assert.Equal(t, int64(125000), rows[0].Amount)
		assert.Equal(t, "USD", rows[0].Currency)
		assert.False(t, rows[0].IsHidden)
	})

	t.Run("hidden included when toggled", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.accountsResult = []*model.Account{
			{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: false},
			{ID: 3, Name: "Assets:Old",  Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true},
		}
		accRepo.balancesResult = map[int64]int64{1: 100, 3: 200}
		svc := newTestAccountService(accRepo)

		rows, err := svc.GetAccountBalancesBulk(context.Background(), 0, true)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.True(t, rows[1].IsHidden)
	})

	t.Run("empty currency normalized to config default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.accountsResult = []*model.Account{
			{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "", IsHidden: false},
		}
		accRepo.balancesResult = map[int64]int64{1: 100}
		svc := newTestAccountService(accRepo)

		rows, err := svc.GetAccountBalancesBulk(context.Background(), 0, false)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, defaultConfig().Defaults.Currency, rows[0].Currency,
			"empty Currency should be normalized to config default")
	})

	t.Run("account not in balances map gets zero", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.accountsResult = []*model.Account{
			{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
			{ID: 2, Name: "Assets:NewlyCreated", Type: model.AccountTypeAsset, Currency: "USD"},
		}
		accRepo.balancesResult = map[int64]int64{1: 500} // 2 omitted
		svc := newTestAccountService(accRepo)

		rows, err := svc.GetAccountBalancesBulk(context.Background(), 0, false)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Equal(t, int64(0), rows[1].Amount, "account missing from balances map should get zero")
	})

	t.Run("empty registry returns non-nil empty slice", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.accountsResult = []*model.Account{}
		accRepo.balancesResult = map[int64]int64{}
		svc := newTestAccountService(accRepo)

		rows, err := svc.GetAccountBalancesBulk(context.Background(), 0, false)
		require.NoError(t, err)
		require.NotNil(t, rows, "must be non-nil so JSON marshals to []")
		assert.Len(t, rows, 0)
	})

	t.Run("GetAllAccounts error propagates", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.getAllAccountsErr = errors.New("db boom")
		svc := newTestAccountService(accRepo)

		_, err := svc.GetAccountBalancesBulk(context.Background(), 0, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load accounts")
	})

	t.Run("GetAllAccountBalances error propagates", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.accountsResult = []*model.Account{
			{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
		}
		accRepo.getAllBalancesErr = errors.New("db boom")
		svc := newTestAccountService(accRepo)

		_, err := svc.GetAccountBalancesBulk(context.Background(), 0, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load balances")
	})
}
```

The exact field names (`accountsResult`, `balancesResult`, `getAllAccountsErr`, `getAllBalancesErr`) are placeholders — use whatever the mock actually exposes after you read it in Step 1. If the mock doesn't have error-injection hooks for these methods yet, add them as a small mock extension. Keep the mock additions minimal and consistent with existing mock fields (e.g., `getByIDErr map[int64]error`).

Imports needed in `account_service_test.go` (add only those not already present): `errors`. The file should already have `context`, `testing`, `testify/assert`, `testify/require`, `model`.

- [ ] **Step 3: Run the new test and confirm it fails**

Run: `go test ./internal/service/ -run TestGetAccountBalancesBulk -v`

Expected: **FAIL** — `svc.GetAccountBalancesBulk` is undefined (compile error). That's the TDD red. If it compiles, the test mock fields are off — re-check Step 1 before continuing.

---

### Task 3: Implement `GetAccountBalancesBulk` on `AccountService`

**Files:**
- Modify: `internal/service/account_service.go` — append after `GetAccountBalanceFormatted` (around line 167).

- [ ] **Step 1: Add `"sort"` to the import block**

Read the import block at the top of `account_service.go`. If `"sort"` is not present, add it alphabetically. (`"context"`, `"errors"`, `"fmt"`, `"strings"` are likely present; check before editing.)

- [ ] **Step 2: Append the method**

After the `GetAccountBalanceFormatted` function (which ends at line ~167), add:

```go

// GetAccountBalancesBulk returns one AccountBalance row per account known to
// the registry, with the balance computed as of asOf (Unix seconds). When
// includeHidden is false, accounts with IsHidden=true are omitted. Rows are
// sorted by Account.ID for deterministic output. An account that has no
// entry in the underlying balances map is reported with Amount=0 — this
// covers newly-created accounts that have no transactions yet.
func (as *AccountService) GetAccountBalancesBulk(
	ctx context.Context, asOf int64, includeHidden bool,
) ([]model.AccountBalance, error) {
	accounts, err := as.repo.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load accounts: %w", err)
	}
	balances, err := as.repo.GetAllAccountBalances(ctx, asOf)
	if err != nil {
		return nil, fmt.Errorf("load balances: %w", err)
	}

	rows := make([]model.AccountBalance, 0, len(accounts))
	for _, acc := range accounts {
		if !includeHidden && acc.IsHidden {
			continue
		}
		currency := acc.Currency
		if currency == "" {
			currency = as.config.Defaults.Currency
		}
		rows = append(rows, model.AccountBalance{
			AccountID: acc.ID,
			Name:      acc.Name,
			Type:      acc.Type,
			ParentID:  acc.ParentID,
			Currency:  currency,
			Amount:    balances[acc.ID],
			IsHidden:  acc.IsHidden,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].AccountID < rows[j].AccountID
	})
	return rows, nil
}
```

Notes:
- `as.config` exists on `AccountService` (verify quickly with a `grep` of `as.config` in `account_service.go` — it should already be used).
- The `make([]model.AccountBalance, 0, len(accounts))` form is important: a nil slice would marshal to `null`, but a zero-length slice marshals to `[]`. The spec requires `"items": []` for empty registries.
- `balances[acc.ID]` returns 0 for missing keys (Go map default). This is the documented zero-balance behavior; no explicit check needed.

- [ ] **Step 3: Run the service test**

Run: `go test ./internal/service/ -run TestGetAccountBalancesBulk -v`

Expected: all 7 subtests PASS.

- [ ] **Step 4: Run the full service-layer suite to catch regressions**

Run: `go test ./internal/service/`

Expected: all tests pass.

---

### Task 4: Write the failing API-layer tests

**Files:**
- Modify: `internal/api/accounts_test.go` — append the new `TestHandleListBalances_*` test family at the end.

Context: existing API tests use `newServerWithStore(t)` and `seedAccount(t, svc, name, accType, balance)` (both in [internal/api/testhelper_test.go](../../internal/api/testhelper_test.go)). The harness returns `*httptest.Server` and `*service.Service`. Tests use `http.Get(ts.URL + "/api/...")` and decode into local structs.

- [ ] **Step 1: Append the test family**

```go
func TestHandleListBalances_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 125000)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 3500)

	resp, err := http.Get(ts.URL + "/api/balances")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.ListResult[model.AccountBalance]
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalCount < 2 {
		t.Fatalf("total_count: got %d, want >= 2 (system account may bump it)", got.TotalCount)
	}
	if got.Limit != 0 || got.Offset != 0 {
		t.Errorf("limit/offset: got %d/%d, want 0/0", got.Limit, got.Offset)
	}
	var foundBank, foundCash bool
	for _, row := range got.Items {
		switch row.Name {
		case "Assets:Bank":
			foundBank = true
			if row.Amount != 125000 {
				t.Errorf("Bank amount: got %d, want 125000", row.Amount)
			}
		case "Assets:Cash":
			foundCash = true
			if row.Amount != 3500 {
				t.Errorf("Cash amount: got %d, want 3500", row.Amount)
			}
		}
	}
	if !foundBank || !foundCash {
		t.Errorf("missing seeded rows: bank=%v cash=%v", foundBank, foundCash)
	}
	// Sorted by AccountID ascending.
	for i := 1; i < len(got.Items); i++ {
		if got.Items[i-1].AccountID > got.Items[i].AccountID {
			t.Errorf("rows not sorted by AccountID at index %d", i)
		}
	}
}

func TestHandleListBalances_AsOfDefault(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	// We do not assert the as_of value the service was called with — the spec
	// accepts this gap deliberately (no nowFunc test seam).
}

func TestHandleListBalances_HistoricalAsOf(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances?as_of=1700000000")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestHandleListBalances_InvalidAsOf(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances?as_of=banana")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "validation_failed" {
		t.Errorf("error code: got %q, want %q", body.Error, "validation_failed")
	}
	if body.Field != "as_of" {
		t.Errorf("field: got %q, want %q", body.Field, "as_of")
	}
}

func TestHandleListBalances_InvalidIncludeHidden(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances?include_hidden=maybe")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "validation_failed" {
		t.Errorf("error code: got %q, want %q", body.Error, "validation_failed")
	}
	if body.Field != "include_hidden" {
		t.Errorf("field: got %q, want %q", body.Field, "include_hidden")
	}
}

func TestHandleListBalances_IncludeHidden(t *testing.T) {
	ts, svc := newServerWithStore(t)
	visible := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	hidden := seedAccount(t, svc, "Assets:Old", model.AccountTypeAsset, 0)

	// Mark "Assets:Old" hidden via the service.
	if _, err := svc.Account().UpdateAccountMetadata(
		t.Context(), hidden.ID, hidden.Description, true,
	); err != nil {
		t.Fatalf("hide account: %v", err)
	}
	_ = visible // referenced via the response

	// Default: hidden excluded.
	respDefault, err := http.Get(ts.URL + "/api/balances")
	if err != nil {
		t.Fatalf("GET default: %v", err)
	}
	defer respDefault.Body.Close()
	var gotDefault model.ListResult[model.AccountBalance]
	if err := json.NewDecoder(respDefault.Body).Decode(&gotDefault); err != nil {
		t.Fatalf("decode default: %v", err)
	}
	for _, row := range gotDefault.Items {
		if row.Name == "Assets:Old" {
			t.Errorf("hidden account should be excluded by default, got row %+v", row)
		}
	}

	// Toggled: hidden included.
	respAll, err := http.Get(ts.URL + "/api/balances?include_hidden=true")
	if err != nil {
		t.Fatalf("GET include_hidden: %v", err)
	}
	defer respAll.Body.Close()
	var gotAll model.ListResult[model.AccountBalance]
	if err := json.NewDecoder(respAll.Body).Decode(&gotAll); err != nil {
		t.Fatalf("decode include_hidden: %v", err)
	}
	var sawHidden bool
	for _, row := range gotAll.Items {
		if row.Name == "Assets:Old" {
			sawHidden = true
			if !row.IsHidden {
				t.Errorf("Assets:Old should have IsHidden=true; got %+v", row)
			}
		}
	}
	if !sawHidden {
		t.Error("Assets:Old should appear when include_hidden=true")
	}
}
```

Notes:
- `t.Context()` requires Go ≥ 1.24. If the project pins an older Go version (check `go.mod`), use `context.Background()` instead. Verify with `head -3 go.mod`.
- `UpdateAccountMetadata(ctx, id, description, isHidden)` signature: verify by grepping `internal/service/account_service.go`. If the signature differs, adapt the call site accordingly.
- The `_ = visible` is just to silence the unused-variable error if a reader simplifies the test later; remove if it bothers you.

Imports needed in `accounts_test.go` (add only what's missing): `context` (for `context.Background()` if you avoid `t.Context()`), `model` (already there), `encoding/json` (already there), `net/http` (already there). No `strings` needed by the new tests as written.

- [ ] **Step 2: Run the new tests and confirm they fail**

Run: `go test ./internal/api/ -run TestHandleListBalances -v`

Expected: **FAIL** — most likely a 404 from the router (no `/api/balances` route registered yet), so the tests see `404` instead of `200`/`400` and the assertions trip. That's the right kind of failure. Compile-error failures (e.g., `model.AccountBalance` undefined) indicate Task 1 isn't applied — go back and confirm.

---

### Task 5: Implement `handleListBalances`

**Files:**
- Modify: `internal/api/accounts.go` — append after `handleAccountBalance` (around line 73, before `handleListAccounts`).

- [ ] **Step 1: Confirm `"time"` is in the import block**

Read the top of `accounts.go`. If `"time"` isn't there, add it. (`net/http`, `model`, `service` are present per the existing file.)

- [ ] **Step 2: Append the handler**

After `handleAccountBalance`:

```go

func (s *Server) handleListBalances(w http.ResponseWriter, r *http.Request) error {
	asOfPtr, err := parseInt64Query(r, "as_of")
	if err != nil {
		return err
	}
	asOf := time.Now().Unix()
	if asOfPtr != nil {
		asOf = *asOfPtr
	}
	includeHidden, err := parseBoolQuery(r, "include_hidden")
	if err != nil {
		return err
	}
	rows, err := s.svc.Account().GetAccountBalancesBulk(r.Context(), asOf, includeHidden)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &model.ListResult[model.AccountBalance]{
		Items:      rows,
		TotalCount: len(rows),
		Limit:      0,
		Offset:     0,
	})
}
```

- [ ] **Step 3: Build to catch any compile errors before route registration**

Run: `go build ./internal/api/`

Expected: no output, exit 0. If `time` import is missing, the build fails — fix and re-run.

---

### Task 6: Register the route

**Files:**
- Modify: `internal/api/router.go` — add one line after the existing `/accounts/{id}/balance` route (line 32).

- [ ] **Step 1: Add the route**

Find this line in `internal/api/router.go`:

```go
		r.Method(http.MethodGet, "/accounts/{id}/balance", apiHandler(s.handleAccountBalance))
```

Add immediately below it (separate logical group from `/accounts/...` routes; the convention is OK since `/balances` is a new top-level read endpoint):

```go
		r.Method(http.MethodGet, "/balances", apiHandler(s.handleListBalances))
```

(Both `/balances` and `/accounts/{id}/balance` coexist fine in chi — different paths.)

- [ ] **Step 2: Run the API tests**

Run: `go test ./internal/api/ -run TestHandleListBalances -v`

Expected: all `TestHandleListBalances_*` tests PASS.

If `TestHandleListBalances_IncludeHidden` fails because `UpdateAccountMetadata` signature mismatch or because `seedAccount`'s default doesn't expose `IsHidden`, adapt the test to whatever the actual service surface offers. The contract being verified is "hidden flag flips presence" — any path that produces a hidden account will do.

- [ ] **Step 3: Run the full API suite to catch regressions**

Run: `go test ./internal/api/`

Expected: all tests pass.

---

### Task 7: Full-suite verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`

Expected: every package passes.

- [ ] **Step 2: Run a clean build**

Run: `go build ./...`

Expected: no output, exit 0.

- [ ] **Step 3: Stage exactly the touched files**

Run `git status`. Expected modified files:
- `internal/model/account.go`
- `internal/service/account_service.go`
- `internal/service/account_service_test.go`
- `internal/api/accounts.go`
- `internal/api/accounts_test.go`
- `internal/api/router.go`

If a mock-extension edit landed in `internal/service/testhelper_test.go` (per Task 2 Step 1's guidance), include it too.

The untracked `docs/web-layer/` directory mentioned in earlier sessions stays untracked — do not stage it.

Stage:

```
git add internal/model/account.go \
        internal/service/account_service.go \
        internal/service/account_service_test.go \
        internal/api/accounts.go \
        internal/api/accounts_test.go \
        internal/api/router.go
```

Add `internal/service/testhelper_test.go` if you extended it.

- [ ] **Step 4: Commit**

```
git commit -m "$(cat <<'EOF'
feat(api): GET /api/balances for bulk account-balance snapshots

Adds AccountService.GetAccountBalancesBulk(ctx, asOf, includeHidden) and a
GET /api/balances handler that returns a ListResult[AccountBalance] of all
accounts with their balances at a point in time. Defaults: as_of=now,
include_hidden=false. No pagination (account counts are bounded). Per-currency
totals are deferred to the SPA, which has every row and can aggregate as it
likes for any given view.
EOF
)"
```

This project does not use a `Co-Authored-By` footer (verify with `git log -3 --format=%B HEAD~..HEAD~3`). Do not add one.

Expected: commit succeeds, working tree clean for tracked files.

---

## Self-Review

**Spec coverage:**

- Spec §"Model" (`AccountBalance` struct): Task 1 ✓
- Spec §"Service method" (`GetAccountBalancesBulk` signature + behavior + sort + currency normalization + zero-balance default): Task 3 ✓
- Spec §"Handler" (`handleListBalances` with as_of default-to-now and include_hidden parsing): Task 5 ✓
- Spec §"Route registration": Task 6 ✓
- Spec §"Endpoint contract" (query param semantics, response envelope, empty-registry shape): covered by Task 5 implementation + Task 4 tests ✓
- Spec §"Error handling" (400 for invalid query, 500 for repo errors via default mapError): Task 4 tests ✓
- Spec §"Tests" service-layer: Task 2 (7 subtests covering each enumerated scenario) ✓
- Spec §"Tests" API-layer: Task 4 (6 test functions: OK, AsOfDefault, HistoricalAsOf, InvalidAsOf, InvalidIncludeHidden, IncludeHidden) ✓
- Spec §"Verification" (`go test ./...`, `go build ./...`, manual smoke): Task 7 ✓ (manual smoke is optional, called out in spec)
- Spec §"Commit shape" (single commit, `feat(api):` scope, body verbatim, no co-author): Task 7 Step 4 ✓
- Spec §"Out of scope": no tasks attempt totals, pagination, tree-shape, batch POST, repo changes, or `nonzero_only`. ✓

**Placeholder scan:**

- No "TBD", "implement later", "fill in details".
- Task 2 Step 1 explicitly tells the implementer to verify mock surface and adapt field names — that's investigation, not a placeholder. The fallback is "ask back via NEEDS_CONTEXT if extending the mock is non-trivial," which is the right escalation path.
- The `_ = visible` workaround in Task 4 is real code, documented inline.
- The `t.Context()` vs `context.Background()` note in Task 4 is a real version-dependent fork with explicit fallback.

**Type/name consistency:**

- `AccountBalance` struct field names match between Task 1 (definition) and Tasks 2/4 (usage).
- `GetAccountBalancesBulk(ctx, asOf, includeHidden)` signature matches between Task 2 (test calls), Task 3 (implementation), and Task 5 (handler call).
- JSON tags (`account_id`, `parent_id`, `is_hidden`) consistent between Task 1 (struct tags) and Task 4 (anonymous response struct).
- Field names in tests (`Error`, `Field`) match the API error envelope from `internal/api/errors.go`.
- Route path `/balances` matches across Task 4 (test URLs) and Task 6 (registration).
