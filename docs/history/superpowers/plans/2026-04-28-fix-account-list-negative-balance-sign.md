# Fix: Account List Strips Negative Sign from Asset/Expense Balances

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the negative sign in `AccountListView.Render` for Asset (`A`) and Expense (`E`) accounts so an overdrawn or contra balance is displayed accurately.

**Architecture:** The fix lives entirely in `ui/views/account_list.go`. Instead of unconditionally trimming the leading `-` from every formatted balance, only trim it for account types whose balances are stored as negative-signed by convention (`L`, `R`, `C`). `A` and `E` accounts pass the sign through unchanged. A unit test is added to `ui/views/account_list_test.go`.

**Tech Stack:** Go, `github.com/olekukonko/tablewriter`, `github.com/pterm/pterm`, `github.com/stretchr/testify`

---

## Files

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `ui/views/account_list.go` | Scope the `-` trim to `L`/`R`/`C` only |
| Create | `ui/views/account_list_test.go` | Unit tests for `AccountListView.Render` balance sign handling |

---

### Task 1: Write the failing test

**Files:**
- Create: `ui/views/account_list_test.go`

- [ ] **Step 1: Write the failing test**

`AccountListView.Render` writes directly to `os.Stdout` via tablewriter, so the test captures stdout with a pipe (same pattern used in `json_test.go`).

```go
package views

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRender redirects stdout, calls Render, and returns the output string.
func captureRender(t *testing.T, accounts []*model.Account, getter func(int64) (string, error)) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	v := NewAccountListView()
	err = v.Render(accounts, getter)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

func TestAccountListView_NegativeAssetBalancePreservesSign(t *testing.T) {
	acc := &model.Account{
		ID:       1,
		Name:     "Assets:Checking",
		Type:     model.AccountTypeAsset,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "-50", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.True(t, strings.Contains(output, "-50"),
		"negative asset balance should show minus sign; got:\n%s", output)
}

func TestAccountListView_NegativeExpenseBalancePreservesSign(t *testing.T) {
	acc := &model.Account{
		ID:       2,
		Name:     "Expenses:Refund",
		Type:     model.AccountTypeExpense,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "-20", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.True(t, strings.Contains(output, "-20"),
		"negative expense balance should show minus sign; got:\n%s", output)
}

func TestAccountListView_NegativeLiabilityBalanceStripsSign(t *testing.T) {
	acc := &model.Account{
		ID:       3,
		Name:     "Liabilities:CreditCard",
		Type:     model.AccountTypeLiability,
		Currency: "TWD",
	}
	// Liabilities are stored negative-signed; the view must strip the sign.
	getter := func(int64) (string, error) { return "-100", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.True(t, strings.Contains(output, "100"),
		"liability balance should strip minus sign; got:\n%s", output)
	assert.False(t, strings.Contains(output, "-100"),
		"liability balance must not show minus sign; got:\n%s", output)
}

func TestAccountListView_PositiveAssetBalanceUnchanged(t *testing.T) {
	acc := &model.Account{
		ID:       4,
		Name:     "Assets:Bank",
		Type:     model.AccountTypeAsset,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "200", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.True(t, strings.Contains(output, "200"),
		"positive asset balance should be unchanged; got:\n%s", output)
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./ui/views/ -run TestAccountListView -v
```

Expected: `FAIL` — `TestAccountListView_NegativeAssetBalancePreservesSign` and `TestAccountListView_NegativeExpenseBalancePreservesSign` fail because the current code strips `-` unconditionally.

---

### Task 2: Fix the view

**Files:**
- Modify: `ui/views/account_list.go:38-40`

- [ ] **Step 1: Replace the unconditional trim with a type-aware strip**

Replace lines 38-40 in `ui/views/account_list.go`:

```go
// Before (line 40):
balanceStr := strings.TrimPrefix(balance, "-")
```

```go
// After — only strip for types stored as negative-signed by convention:
var balanceStr string
switch acc.Type {
case model.AccountTypeLiability, model.AccountTypeRevenue, model.AccountTypeEquity:
    balanceStr = strings.TrimPrefix(balance, "-")
default: // Asset, Expense: preserve sign
    balanceStr = balance
}
```

The `import "github.com/hance08/kea/internal/model"` is already present (used by `model.Account` in the function signature), so no import change is needed.

You can also remove the `"strings"` import if it is no longer used elsewhere in the file — check first with:
```bash
grep -n 'strings\.' ui/views/account_list.go
```
If `strings.TrimPrefix` was the only usage, remove the `"strings"` import. Otherwise leave it.

- [ ] **Step 2: Run the tests to confirm they pass**

```bash
go test ./ui/views/ -run TestAccountListView -v
```

Expected output:
```
--- PASS: TestAccountListView_NegativeAssetBalancePreservesSign
--- PASS: TestAccountListView_NegativeExpenseBalancePreservesSign
--- PASS: TestAccountListView_NegativeLiabilityBalanceStripsSign
--- PASS: TestAccountListView_PositiveAssetBalanceUnchanged
PASS
```

- [ ] **Step 3: Run the full test suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 4: Commit**

```bash
git add ui/views/account_list.go ui/views/account_list_test.go
git commit -m "fix: preserve negative sign for Asset/Expense balances in account list view

Closes #35"
```
