# Model JSON Tags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `json:"snake_case"` tags to all core model structs so they serialize with consistent, API-friendly field names.

**Architecture:** Pure additive change — add struct tags to existing model types in `internal/model/`. No behavioral changes, no new types. The codebase already has JSON DTO types in `ui/views/json_types.go` that handle domain-specific transformations (cents→float, unix→date string). The model-level JSON tags provide a sensible default serialization without replacing those DTOs.

**Tech Stack:** Go struct tags, `encoding/json`, `testify`

**Design decisions:**
- **No custom time type.** The issue suggests "consider" a custom marshaler for timestamps. We defer this: the `Timestamp` field is used in 22+ locations across store, service, cmd, and UI layers — arithmetic, SQL scanning, comparisons, `time.Unix()` formatting. Changing it to a wrapper type would be invasive. The existing `ui/views/json_types.go` DTO layer already handles timestamp→RFC3339 conversion for JSON output. When the web layer is built, it should use response DTOs (as the codebase already does) rather than coupling serialization format to the domain model.
- **No `json:"-"` fields.** All fields on the listed structs are API-relevant. No fields need to be hidden.
- **`TransactionRule` excluded.** The issue does not list it and it is TUI-internal (prompt text for the interactive form).
- **`omitempty` on pointer/optional fields only.** `ParentID *int64` and `ExternalID *string` get `omitempty` so null pointers serialize as absent rather than `null`. All other fields are always present.

---

### Task 1: Add JSON tags to Account struct

**Files:**
- Modify: `internal/model/account.go:6-14`
- Create: `internal/model/json_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/model/json_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model_test

import (
	"encoding/json"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccount_JSONKeys(t *testing.T) {
	parentID := int64(5)
	acc := model.Account{
		ID:          1,
		Name:        "Assets:Bank",
		Type:        model.AccountTypeAsset,
		ParentID:    &parentID,
		Currency:    "USD",
		Description: "Main bank",
		IsHidden:    false,
	}

	data, err := json.Marshal(acc)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "name")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "parent_id")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "is_hidden")

	assert.NotContains(t, m, "ID")
	assert.NotContains(t, m, "ParentID")
	assert.NotContains(t, m, "IsHidden")
}

func TestAccount_JSON_OmitsNullParentID(t *testing.T) {
	acc := model.Account{
		ID:       1,
		Name:     "Assets:Cash",
		Type:     model.AccountTypeAsset,
		Currency: "USD",
	}

	data, err := json.Marshal(acc)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, exists := m["parent_id"]
	assert.False(t, exists, "parent_id should be omitted when nil")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestAccount_JSON -v`
Expected: FAIL — keys will be `ID`, `ParentID`, `IsHidden` (Go defaults)

- [ ] **Step 3: Add JSON tags to Account struct**

In `internal/model/account.go`, replace the Account struct:

```go
type Account struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	Type        AccountType `json:"type"`
	ParentID    *int64      `json:"parent_id,omitempty"`
	Currency    string      `json:"currency"`
	Description string      `json:"description"`
	IsHidden    bool        `json:"is_hidden"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run TestAccount_JSON -v`
Expected: PASS

- [ ] **Step 5: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/model/account.go internal/model/json_test.go
git commit -m "feat: add JSON tags to Account struct (issue #74)"
```

---

### Task 2: Add JSON tags to Transaction and TransactionDetail structs

**Files:**
- Modify: `internal/model/transaction.go:8-24`
- Modify: `internal/model/json_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/model/json_test.go`:

```go
func TestTransaction_JSONKeys(t *testing.T) {
	extID := "ext-123"
	tx := model.Transaction{
		ID:          10,
		Timestamp:   1700000000,
		Description: "Groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		ExternalID:  &extID,
	}

	data, err := json.Marshal(tx)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "external_id")

	assert.NotContains(t, m, "ID")
	assert.NotContains(t, m, "ExternalID")
}

func TestTransaction_JSON_OmitsNullExternalID(t *testing.T) {
	tx := model.Transaction{
		ID:          10,
		Timestamp:   1700000000,
		Description: "Groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
	}

	data, err := json.Marshal(tx)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, exists := m["external_id"]
	assert.False(t, exists, "external_id should be omitted when nil")
}

func TestTransactionDetail_JSONKeys(t *testing.T) {
	td := model.TransactionDetail{
		ID:          10,
		Timestamp:   1700000000,
		Description: "Groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		Splits: []model.SplitDetail{
			{ID: 1, AccountID: 2, AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 1000, Currency: "USD"},
		},
	}

	data, err := json.Marshal(td)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "splits")

	assert.NotContains(t, m, "Splits")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/model/ -run "TestTransaction_JSON|TestTransactionDetail_JSON" -v`
Expected: FAIL

- [ ] **Step 3: Add JSON tags to Transaction and TransactionDetail structs**

In `internal/model/transaction.go`, replace both structs:

```go
type Transaction struct {
	ID          int64             `json:"id"`
	Timestamp   int64             `json:"timestamp"`
	Description string            `json:"description"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	ExternalID  *string           `json:"external_id,omitempty"`
}

type TransactionDetail struct {
	ID          int64             `json:"id"`
	Timestamp   int64             `json:"timestamp"`
	Description string            `json:"description"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	Splits      []SplitDetail     `json:"splits"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/model/ -run "TestTransaction_JSON|TestTransactionDetail_JSON" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/model/transaction.go internal/model/json_test.go
git commit -m "feat: add JSON tags to Transaction and TransactionDetail structs (issue #74)"
```

---

### Task 3: Add JSON tags to Split and SplitDetail structs

**Files:**
- Modify: `internal/model/transaction.go:63-80`
- Modify: `internal/model/json_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/model/json_test.go`:

```go
func TestSplit_JSONKeys(t *testing.T) {
	s := model.Split{
		ID:            1,
		TransactionID: 10,
		AccountID:     2,
		Amount:        1000,
		Currency:      "USD",
		Memo:          "test",
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "transaction_id")
	assert.Contains(t, m, "account_id")
	assert.Contains(t, m, "amount")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "memo")

	assert.NotContains(t, m, "TransactionID")
	assert.NotContains(t, m, "AccountID")
}

func TestSplitDetail_JSONKeys(t *testing.T) {
	sd := model.SplitDetail{
		ID:          1,
		AccountID:   2,
		AccountName: "Assets:Bank",
		AccountType: model.AccountTypeAsset,
		Amount:      1000,
		Currency:    "USD",
		Memo:        "test",
	}

	data, err := json.Marshal(sd)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "account_id")
	assert.Contains(t, m, "account_name")
	assert.Contains(t, m, "account_type")
	assert.Contains(t, m, "amount")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "memo")

	assert.NotContains(t, m, "AccountID")
	assert.NotContains(t, m, "AccountName")
	assert.NotContains(t, m, "AccountType")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/model/ -run "TestSplit_JSON|TestSplitDetail_JSON" -v`
Expected: FAIL

- [ ] **Step 3: Add JSON tags to Split and SplitDetail structs**

In `internal/model/transaction.go`, replace both structs:

```go
type Split struct {
	ID            int64  `json:"id"`
	TransactionID int64  `json:"transaction_id"`
	AccountID     int64  `json:"account_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Memo          string `json:"memo"`
}

type SplitDetail struct {
	ID          int64       `json:"id"`
	AccountID   int64       `json:"account_id"`
	AccountName string      `json:"account_name"`
	AccountType AccountType `json:"account_type"`
	Amount      int64       `json:"amount"`
	Currency    string      `json:"currency"`
	Memo        string      `json:"memo"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/model/ -run "TestSplit_JSON|TestSplitDetail_JSON" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/model/transaction.go internal/model/json_test.go
git commit -m "feat: add JSON tags to Split and SplitDetail structs (issue #74)"
```

---

### Task 4: Add JSON tags to ReconcileEntry struct

**Files:**
- Modify: `internal/model/transaction.go:86-93`
- Modify: `internal/model/json_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/model/json_test.go`:

```go
func TestReconcileEntry_JSONKeys(t *testing.T) {
	re := model.ReconcileEntry{
		ID:            1,
		Timestamp:     1700000000,
		Description:   "Payment",
		Status:        model.StatusCleared,
		Amount:        5000,
		OffsetAccount: "Expenses:Rent",
	}

	data, err := json.Marshal(re)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "amount")
	assert.Contains(t, m, "offset_account")

	assert.NotContains(t, m, "OffsetAccount")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestReconcileEntry_JSON -v`
Expected: FAIL

- [ ] **Step 3: Add JSON tags to ReconcileEntry struct**

In `internal/model/transaction.go`, replace the struct:

```go
type ReconcileEntry struct {
	ID            int64             `json:"id"`
	Timestamp     int64             `json:"timestamp"`
	Description   string            `json:"description"`
	Status        TransactionStatus `json:"status"`
	Amount        int64             `json:"amount"`
	OffsetAccount string            `json:"offset_account"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run TestReconcileEntry_JSON -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/model/transaction.go internal/model/json_test.go
git commit -m "feat: add JSON tags to ReconcileEntry struct (issue #74)"
```

---

### Task 5: Add JSON tags to TransactionStatus and TransactionType for string serialization

**Files:**
- Modify: `internal/model/types.go:68-69`
- Modify: `internal/model/json_test.go`

The `TransactionStatus` (int) and `TransactionType` (string) types are used as struct fields. `TransactionType` already serializes as its string value since it's a `string` typedef. However, `TransactionStatus` is an `int` typedef and serializes as a raw number (0, 1, 2) — not human-readable. Add a `MarshalJSON`/`UnmarshalJSON` pair so it serializes as `"Pending"`, `"Cleared"`, `"Reconciled"`.

- [ ] **Step 1: Write the failing test**

Append to `internal/model/json_test.go`:

```go
func TestTransactionStatus_JSONMarshal(t *testing.T) {
	tests := []struct {
		status model.TransactionStatus
		want   string
	}{
		{model.StatusPending, `"Pending"`},
		{model.StatusCleared, `"Cleared"`},
		{model.StatusReconciled, `"Reconciled"`},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			data, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(data))
		})
	}
}

func TestTransactionStatus_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		input string
		want  model.TransactionStatus
	}{
		{`"Pending"`, model.StatusPending},
		{`"Cleared"`, model.StatusCleared},
		{`"Reconciled"`, model.StatusReconciled},
		{`0`, model.StatusPending},
		{`1`, model.StatusCleared},
		{`2`, model.StatusReconciled},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var s model.TransactionStatus
			err := json.Unmarshal([]byte(tt.input), &s)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestTransactionStatus_JSONRoundTrip(t *testing.T) {
	tx := model.Transaction{
		ID:     1,
		Status: model.StatusReconciled,
		Type:   model.TxTypeExpense,
	}

	data, err := json.Marshal(tx)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Reconciled", m["status"])

	var tx2 model.Transaction
	require.NoError(t, json.Unmarshal(data, &tx2))
	assert.Equal(t, model.StatusReconciled, tx2.Status)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/model/ -run "TestTransactionStatus_JSON" -v`
Expected: FAIL — status marshals as `0`/`1`/`2` instead of string names

- [ ] **Step 3: Add MarshalJSON and UnmarshalJSON to TransactionStatus**

In `internal/model/types.go`, add after the existing `String()` method (after line 88):

```go
func (s TransactionStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *TransactionStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		var num int
		if err2 := json.Unmarshal(data, &num); err2 != nil {
			return err
		}
		*s = TransactionStatus(num)
		return nil
	}
	switch str {
	case "Pending":
		*s = StatusPending
	case "Cleared":
		*s = StatusCleared
	case "Reconciled":
		*s = StatusReconciled
	default:
		return fmt.Errorf("unknown transaction status %q", str)
	}
	return nil
}
```

Also add `"encoding/json"` to the import block in `types.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/model/ -run "TestTransactionStatus_JSON" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/model/types.go internal/model/json_test.go
git commit -m "feat: add JSON marshal/unmarshal for TransactionStatus (issue #74)"
```

---

### Task 6: Final verification and closing commit

**Files:** None new

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: all tests pass

- [ ] **Step 2: Verify JSON output is correct end-to-end**

Run: `go test ./internal/model/ -run "JSON" -v`
Expected: all JSON tests pass

- [ ] **Step 3: Build the binary**

Run: `make build`
Expected: builds successfully

- [ ] **Step 4: Close the issue**

```bash
gh issue close 74 --comment "Resolved: added json tags to Account, Transaction, TransactionDetail, Split, SplitDetail, ReconcileEntry structs. Added JSON string marshaling for TransactionStatus. Custom time type deferred — existing DTO layer in ui/views/json_types.go handles timestamp formatting for API output."
```
