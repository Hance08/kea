# Web API Write Endpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 7 HTTP write endpoints on top of the existing `internal/api/` foundation: `POST /api/accounts`, `PATCH /api/accounts/{id}`, `DELETE /api/accounts/{id}`, `POST /api/transactions`, `PATCH /api/transactions/{id}`, `PATCH /api/transactions/{id}/status`, `DELETE /api/transactions/{id}`. Bring the four write-only error sentinels (`ErrAlreadyExists`, `ErrReconciled`, `ErrCircularParent`, `ErrNotEditable`) online with integration tests.

**Architecture:** Two new handler files (`accounts_write.go`, `transactions_write.go`) hung off `*Server`. JSON tags are added in place to three model input structs (`CreateAccountInput`, `CreateTransactionFromSplitsInput`, `UpdateTransactionInput`) so handlers can decode straight into them — zero translation layer. One small API-local DTO (`updateAccountRequest`) handles field-presence dispatch for `PATCH /api/accounts/{id}` using pointer fields. Tests reuse the read-spec's `newServerWithStore` substrate; a sibling helper `newServerForWrite` exposes the underlying `*store.Store` so write tests can mark transactions reconciled and inject a parent cycle via the repo.

**Tech Stack:** Go 1.25, `github.com/go-chi/chi/v5` (already in `go.mod`), stdlib `net/http` + `net/http/httptest` + `encoding/json`. No new dependencies.

**Spec:** [`docs/superpowers/specs/2026-06-03-web-api-write-endpoints-design.md`](../specs/2026-06-03-web-api-write-endpoints-design.md)

---

## File Layout

| File | Responsibility |
|------|----------------|
| `internal/model/input.go` *(modify)* | Add JSON tags to `CreateAccountInput`, `CreateTransactionFromSplitsInput`, `UpdateTransactionInput`. |
| `internal/model/json_test.go` *(modify)* | Round-trip tests for the three now-tagged structs. |
| `internal/api/testhelper_test.go` *(modify)* | Add `newServerForWrite` sibling helper that exposes `*store.Store`; add `seedReconciledTransaction` and `injectParentSelfCycle` helpers. |
| `internal/api/accounts_write.go` *(create)* | `handleCreateAccount`, `handleUpdateAccount`, `handleDeleteAccount`, `updateAccountRequest` DTO. |
| `internal/api/accounts_write_test.go` *(create)* | End-to-end HTTP tests for the three account write routes. |
| `internal/api/transactions_write.go` *(create)* | `handleCreateTransaction`, `handleUpdateTransaction`, `handleUpdateTransactionStatus`, `handleDeleteTransaction`, `updateStatusRequest` DTO. |
| `internal/api/transactions_write_test.go` *(create)* | End-to-end HTTP tests for the four transaction write routes. |
| `internal/api/router.go` *(modify)* | Register the seven new routes. |

Build order: model JSON tags → testhelper extension → account POST → account DELETE → account PATCH → transaction POST → transaction DELETE → transaction status PATCH → transaction full PATCH → final verification. Each task ends in a green build with new endpoint(s) registered and tested.

---

## Task 1: JSON tags on model input structs

**Files:**
- Modify: `internal/model/input.go`
- Modify: `internal/model/json_test.go`

- [ ] **Step 1: Write failing tests for the three input struct JSON shapes**

Append to `internal/model/json_test.go`:

```go
func TestCreateAccountInput_JSONKeys(t *testing.T) {
	parentID := int64(7)
	in := model.CreateAccountInput{
		Name:        "Assets:Bank:Checking",
		Type:        model.AccountTypeAsset,
		Currency:    "USD",
		Description: "primary checking",
		ParentID:    &parentID,
		Balance:     12345,
	}
	data, err := json.Marshal(in)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "name")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "parent_id")
	assert.Contains(t, m, "balance")

	assert.NotContains(t, m, "Name")
	assert.NotContains(t, m, "ParentID")
}

func TestCreateAccountInput_JSON_OmitsNullParentID(t *testing.T) {
	in := model.CreateAccountInput{
		Name:     "Assets:Cash",
		Type:     model.AccountTypeAsset,
		Currency: "USD",
	}
	data, err := json.Marshal(in)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, exists := m["parent_id"]
	assert.False(t, exists, "parent_id should be omitted when nil")
}

func TestCreateAccountInput_JSON_RoundTrip(t *testing.T) {
	src := `{"name":"Assets:Bank","type":"A","currency":"USD","description":"d","parent_id":42,"balance":10000}`
	var in model.CreateAccountInput
	require.NoError(t, json.Unmarshal([]byte(src), &in))
	assert.Equal(t, "Assets:Bank", in.Name)
	assert.Equal(t, model.AccountTypeAsset, in.Type)
	assert.Equal(t, "USD", in.Currency)
	assert.Equal(t, "d", in.Description)
	require.NotNil(t, in.ParentID)
	assert.Equal(t, int64(42), *in.ParentID)
	assert.Equal(t, int64(10000), in.Balance)
}

func TestCreateTransactionFromSplitsInput_JSONKeys(t *testing.T) {
	in := model.CreateTransactionFromSplitsInput{
		Splits:      []model.SplitDetail{{AccountName: "Assets:Bank", Amount: -500}},
		Description: "Coffee",
		Timestamp:   1700000000,
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
	}
	data, err := json.Marshal(in)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "splits")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "type")

	assert.NotContains(t, m, "Splits")
}

func TestCreateTransactionFromSplitsInput_JSON_RoundTrip(t *testing.T) {
	src := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"Coffee",
		"timestamp":1700000000,
		"status":"Cleared",
		"type":"Expense"
	}`
	var in model.CreateTransactionFromSplitsInput
	require.NoError(t, json.Unmarshal([]byte(src), &in))
	assert.Equal(t, "Coffee", in.Description)
	assert.Equal(t, int64(1700000000), in.Timestamp)
	assert.Equal(t, model.StatusCleared, in.Status)
	assert.Equal(t, model.TxTypeExpense, in.Type)
	require.Len(t, in.Splits, 2)
	assert.Equal(t, "Assets:Bank", in.Splits[0].AccountName)
	assert.Equal(t, int64(-500), in.Splits[0].Amount)
}

func TestUpdateTransactionInput_JSON_IDInvisibleOnMarshal(t *testing.T) {
	in := model.UpdateTransactionInput{
		ID:          99,
		Description: "x",
		Timestamp:   1700000000,
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		Splits:      []model.SplitDetail{},
	}
	data, err := json.Marshal(in)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, hasID := m["id"]
	assert.False(t, hasID, "id should not be emitted in JSON")

	assert.Contains(t, m, "description")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "splits")
}

func TestUpdateTransactionInput_JSON_RoundTrip_IgnoresIDInInput(t *testing.T) {
	src := `{"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense","splits":[]}`
	var in model.UpdateTransactionInput
	require.NoError(t, json.Unmarshal([]byte(src), &in))
	assert.Equal(t, int64(0), in.ID, "id field must remain zero when absent from JSON")
	assert.Equal(t, "x", in.Description)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/model/ -run 'CreateAccountInput|CreateTransactionFromSplitsInput|UpdateTransactionInput' -v
```

Expected: FAIL — without JSON tags, the marshalled keys will be `Name`, `Type`, `ParentID`, etc., and assertions on `name`, `type`, `parent_id` will not match. The unmarshal round-trips fail because Go's case-insensitive matching is partial (it handles unambiguous case mismatches but the `id` field can't round-trip through `json:"-"`).

- [ ] **Step 3: Add JSON tags to `internal/model/input.go`**

Replace the file contents (preserve SPDX header) with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type CreateAccountInput struct {
	Name        string      `json:"name"`
	Type        AccountType `json:"type"`
	Currency    string      `json:"currency"`
	Description string      `json:"description"`
	ParentID    *int64      `json:"parent_id,omitempty"`
	Balance     int64       `json:"balance"`
}

type CreateSimpleTransactionInput struct {
	FromAccount string
	ToAccount   string
	Amount      int64
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
}

type CreateTransactionFromSplitsInput struct {
	Splits      []SplitDetail     `json:"splits"`
	Description string            `json:"description"`
	Timestamp   int64             `json:"timestamp"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
}

type UpdateTransactionInput struct {
	ID          int64             `json:"-"`
	Description string            `json:"description"`
	Timestamp   int64             `json:"timestamp"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	Splits      []SplitDetail     `json:"splits"`
}
```

`CreateSimpleTransactionInput` is intentionally left untagged — it's CLI-only and not exposed through the API in this spec.

`json:"-"` on `UpdateTransactionInput.ID` makes the field invisible to the JSON encoder/decoder; combined with `decodeJSON`'s `DisallowUnknownFields`, any `"id"` key in the request body is rejected as an unknown field.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/model/ -run 'CreateAccountInput|CreateTransactionFromSplitsInput|UpdateTransactionInput' -v
```

Expected: PASS for all seven new tests.

- [ ] **Step 5: Run the full model tests to confirm no regressions**

```bash
go test ./internal/model/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/model/input.go internal/model/json_test.go
git commit -m "feat(model): add JSON tags to write-endpoint input structs"
```

---

## Task 2: Extend testhelper for write-endpoint tests

**Files:**
- Modify: `internal/api/testhelper_test.go`

This task introduces three helpers the upcoming write-endpoint tests need: a server constructor that exposes the underlying store, a reconciled-tx seed helper, and a parent-cycle injector. It is purely test infrastructure but is exercised by inline sanity tests.

- [ ] **Step 1: Write failing sanity tests for the new helpers**

Append to `internal/api/testhelper_test.go`:

```go
func TestNewServerForWrite_ExposesStore(t *testing.T) {
	_, _, st := newServerForWrite(t)
	if st == nil {
		t.Fatal("store is nil")
	}
}

func TestSeedReconciledTransaction(t *testing.T) {
	_, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	seedReconciledTransaction(t, st, src.ID, d.ID)

	got, err := svc.Transaction().GetTransactionByID(t.Context(), d.ID)
	if err != nil {
		t.Fatalf("get tx: %v", err)
	}
	if got.Status != model.StatusReconciled {
		t.Fatalf("status: got %v, want Reconciled", got.Status)
	}
}

func TestInjectParentSelfCycle(t *testing.T) {
	_, svc, st := newServerForWrite(t)
	acc := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	injectParentSelfCycle(t, st, acc.ID)

	got, err := svc.Account().GetAccountByID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("get acc: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != acc.ID {
		t.Fatalf("parent_id: got %v, want self-pointer (%d)", got.ParentID, acc.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestNewServerForWrite|TestSeedReconciledTransaction|TestInjectParentSelfCycle' -v
```

Expected: FAIL — `newServerForWrite`, `seedReconciledTransaction`, `injectParentSelfCycle` are undefined.

- [ ] **Step 3: Add the helpers to `internal/api/testhelper_test.go`**

Update the import block of `testhelper_test.go` to add `"github.com/hance08/kea/internal/store"`. Then append:

```go
// newServerForWrite is a variant of newServerWithStore that also returns the
// underlying *store.Store, so write tests can manipulate reconcile state and
// inject a parent cycle directly via the repo (bypassing the service layer
// where convenient).
func newServerForWrite(t *testing.T) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"

	svc := service.NewService(st, st, st, cfg)
	srv := NewServer(cfg, svc, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc, st
}

// seedReconciledTransaction flips a Cleared transaction to Reconciled by
// directly invoking the reconcile-aware repo methods. Mirrors what
// service.ReconcileTransactions does internally, minus the balance bookkeeping
// (which write-endpoint tests do not need).
func seedReconciledTransaction(t *testing.T, st *store.Store, accountID int64, txID int64) {
	t.Helper()
	if _, err := st.MarkSplitsReconciledByAccount(t.Context(), accountID, []int64{txID}); err != nil {
		t.Fatalf("MarkSplitsReconciledByAccount: %v", err)
	}
	if err := st.BulkUpdateTransactionStatus(t.Context(), []int64{txID}, model.StatusReconciled); err != nil {
		t.Fatalf("BulkUpdateTransactionStatus: %v", err)
	}
}

// injectParentSelfCycle directly sets accounts.parent_id = id on the given
// account, going around the service-layer cycle validation. Used to exercise
// the ErrCircularParent code path from API tests.
func injectParentSelfCycle(t *testing.T, st *store.Store, accountID int64) {
	t.Helper()
	if _, err := st.DB().ExecContext(t.Context(),
		"UPDATE accounts SET parent_id = ? WHERE id = ?", accountID, accountID); err != nil {
		t.Fatalf("inject cycle: %v", err)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestNewServerForWrite|TestSeedReconciledTransaction|TestInjectParentSelfCycle' -v
```

Expected: PASS for all three.

- [ ] **Step 5: Run the full api package tests to confirm no regressions**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/testhelper_test.go
git commit -m "test(api): add write-endpoint test helpers"
```

---

## Task 3: `POST /api/accounts`

**Files:**
- Create: `internal/api/accounts_write.go`
- Modify: `internal/api/router.go`
- Create: `internal/api/accounts_write_test.go`

- [ ] **Step 1: Write failing tests for `POST /api/accounts`**

Create `internal/api/accounts_write_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestHandleCreateAccount_OK(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Cash","type":"A","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:Cash" || got.Type != model.AccountTypeAsset || got.Currency != "USD" {
		t.Errorf("got %+v", got)
	}
	if got.ID == 0 {
		t.Errorf("expected non-zero id")
	}
}

func TestHandleCreateAccount_WithBalance(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Bank","type":"A","currency":"USD","description":"","balance":100000}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
	var acc model.Account
	if err := json.NewDecoder(resp.Body).Decode(&acc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	balResp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
	if err != nil {
		t.Fatalf("GET balance: %v", err)
	}
	defer balResp.Body.Close()
	var bal balanceResponse
	if err := json.NewDecoder(balResp.Body).Decode(&bal); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	if bal.Amount != 100000 {
		t.Errorf("balance: got %d, want 100000", bal.Amount)
	}

	sysResp, err := http.Get(ts.URL + "/api/accounts/by-name?name=Equity:OpeningBalances_USD")
	if err != nil {
		t.Fatalf("GET sys account: %v", err)
	}
	defer sysResp.Body.Close()
	if sysResp.StatusCode != http.StatusOK {
		t.Errorf("system equity account not found: %d", sysResp.StatusCode)
	}
}

func TestHandleCreateAccount_LiabilityBalanceSignReversed(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Liabilities:CreditCard","type":"L","currency":"USD","description":"","balance":50000}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	var acc model.Account
	_ = json.NewDecoder(resp.Body).Decode(&acc)

	balResp, _ := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
	defer balResp.Body.Close()
	var bal balanceResponse
	_ = json.NewDecoder(balResp.Body).Decode(&bal)
	// Liability opening: liability split = -amount, so the stored balance is -50000.
	if bal.Amount != -50000 {
		t.Errorf("liability balance: got %d, want -50000", bal.Amount)
	}
}

func TestHandleCreateAccount_DuplicateName(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	body := `{"name":"Assets:Cash","type":"A","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "already_exists" {
		t.Errorf("error code: got %q, want already_exists", errBody["error"])
	}
}

func TestHandleCreateAccount_CircularParent(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	// Create a valid asset parent, then poke its parent_id to point at itself.
	parent := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	injectParentSelfCycle(t, st, parent.ID)

	// Now attempting to create a child of "Assets:Bank" walks the cycle.
	body, _ := json.Marshal(map[string]any{
		"name":      "Assets:Bank:Checking",
		"type":      "A",
		"currency":  "USD",
		"parent_id": parent.ID,
		"balance":   0,
	})
	resp := postJSON(t, ts.URL+"/api/accounts", string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "circular_parent" {
		t.Errorf("error: got %q, want circular_parent", errBody["error"])
	}
}

func TestHandleCreateAccount_InvalidType(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Cash","type":"Z","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "type" {
		t.Errorf("field: got %q, want type", errBody["field"])
	}
}

func TestHandleCreateAccount_EmptyName(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"","type":"A","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "name" {
		t.Errorf("field: got %q, want name", errBody["field"])
	}
}

func TestHandleCreateAccount_DescriptionTooLong(t *testing.T) {
	ts, _ := newServerWithStore(t)

	longDesc := strings.Repeat("a", model.DescriptionMaxLength+1)
	body, err := json.Marshal(map[string]any{
		"name":        "Assets:Cash",
		"type":        "A",
		"currency":    "USD",
		"description": longDesc,
		"balance":     0,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := postJSON(t, ts.URL+"/api/accounts", string(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "description" {
		t.Errorf("field: got %q, want description", errBody["field"])
	}
}

func TestHandleCreateAccount_UnknownField(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Cash","type":"A","currency":"USD","balance":0,"unknown_field":1}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleCreateAccount' -v
```

Expected: FAIL — every test reports 404 because the route is not registered yet.

- [ ] **Step 3: Create the handler in `internal/api/accounts_write.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	var input model.CreateAccountInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	acc, err := s.svc.Account().CreateAccountWithBalance(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, acc)
}
```

`CreateAccountWithBalance` already short-circuits to a no-opening-balance path when `input.Balance == 0`, so the handler does not need to branch.

- [ ] **Step 4: Register the route in `internal/api/router.go`**

Add the line below the existing `/reports/net-worth` line, inside the `r.Route("/api", ...)` block:

```go
r.Method(http.MethodPost, "/accounts", apiHandler(s.handleCreateAccount))
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleCreateAccount' -v
```

Expected: PASS for all nine tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/accounts_write.go internal/api/accounts_write_test.go internal/api/router.go
git commit -m "feat(api): POST /api/accounts"
```

---

## Task 4: `DELETE /api/accounts/{id}`

**Files:**
- Modify: `internal/api/accounts_write.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/accounts_write_test.go`

- [ ] **Step 1: Write failing tests for `DELETE /api/accounts/{id}`**

Append to `internal/api/accounts_write_test.go`:

```go
func deleteReq(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

func TestHandleDeleteAccount_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(acc.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["deleted"] != true {
		t.Errorf("deleted: got %v, want true", body["deleted"])
	}
	if int64(body["id"].(float64)) != acc.ID {
		t.Errorf("id: got %v, want %d", body["id"], acc.ID)
	}

	getResp, _ := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID))
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete, GET status: got %d, want 404", getResp.StatusCode)
	}
}

func TestHandleDeleteAccount_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/accounts/9999")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleDeleteAccount_BadPath(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/accounts/abc")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleDeleteAccount_HasChildren(t *testing.T) {
	ts, svc := newServerWithStore(t)
	parent := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	_, err := svc.Account().CreateAccount(t.Context(), model.CreateAccountInput{
		Name:     "Assets:Bank:Checking",
		Type:     model.AccountTypeAsset,
		Currency: "USD",
		ParentID: &parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(parent.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "validation_failed" {
		t.Errorf("error: got %q, want validation_failed", errBody["error"])
	}
}

func TestHandleDeleteAccount_HasTransactions(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	_ = seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(src.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleDeleteAccount_SystemAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000)

	sys, err := svc.Account().GetAccountByName(t.Context(), "Equity:OpeningBalances_USD")
	if err != nil {
		t.Fatalf("lookup sys account: %v", err)
	}

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(sys.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "not_editable" {
		t.Errorf("error: got %q, want not_editable", errBody["error"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleDeleteAccount' -v
```

Expected: FAIL — route not registered, 404 across the board.

- [ ] **Step 3: Add the delete handler to `internal/api/accounts_write.go`**

Append to the file:

```go
func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	acc, err := s.svc.Account().GetAccountByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.svc.Account().DeleteAccountByName(ctx, acc.Name); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}
```

The `GetAccountByID` step gives a clean `service.ErrNotFound` → 404 mapping for missing IDs and supplies the name needed by `DeleteAccountByName`.

- [ ] **Step 4: Register the route in `internal/api/router.go`**

Append to the `/api` block:

```go
r.Method(http.MethodDelete, "/accounts/{id}", apiHandler(s.handleDeleteAccount))
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleDeleteAccount' -v
```

Expected: PASS for all six tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/accounts_write.go internal/api/accounts_write_test.go internal/api/router.go
git commit -m "feat(api): DELETE /api/accounts/{id}"
```

---

## Task 5: `PATCH /api/accounts/{id}`

**Files:**
- Modify: `internal/api/accounts_write.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/accounts_write_test.go`

- [ ] **Step 1: Write failing tests for `PATCH /api/accounts/{id}`**

Append to `internal/api/accounts_write_test.go`:

```go
func patchJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
	}
	return resp
}

func TestHandleUpdateAccount_NoFields(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "validation_failed" {
		t.Errorf("error: got %q", errBody["error"])
	}
}

func TestHandleUpdateAccount_RenameOnly(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"name":"Assets:CashRenamed"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:CashRenamed" {
		t.Errorf("name: got %q", got.Name)
	}

	oldResp, _ := http.Get(ts.URL + "/api/accounts/by-name?name=Assets:Cash")
	defer oldResp.Body.Close()
	if oldResp.StatusCode != http.StatusNotFound {
		t.Errorf("old name still resolvable: %d", oldResp.StatusCode)
	}
}

func TestHandleUpdateAccount_MetadataOnly(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"description":"new desc","is_hidden":true}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Description != "new desc" || !got.IsHidden {
		t.Errorf("got %+v", got)
	}
}

func TestHandleUpdateAccount_RenameAndMetadata(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	body := `{"name":"Assets:Cash2","description":"d2","is_hidden":true}`
	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:Cash2" || got.Description != "d2" || !got.IsHidden {
		t.Errorf("got %+v", got)
	}
}

func TestHandleUpdateAccount_NameEqualToCurrent_NoOp(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	body := `{"name":"Assets:Cash","description":"updated"}`
	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Name != "Assets:Cash" || got.Description != "updated" {
		t.Errorf("got %+v", got)
	}
}

func TestHandleUpdateAccount_RenameAcrossParentPath(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"name":"Liabilities:CC"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "name" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleUpdateAccount_RenameSystemAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000) // creates Equity:OpeningBalances_USD
	sys, err := svc.Account().GetAccountByName(t.Context(), "Equity:OpeningBalances_USD")
	if err != nil {
		t.Fatalf("lookup sys: %v", err)
	}

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(sys.ID), `{"name":"Equity:OpeningBalances_USD2"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "not_editable" {
		t.Errorf("error: got %q", errBody["error"])
	}
}

func TestHandleUpdateAccount_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := patchJSON(t, ts.URL+"/api/accounts/9999", `{"description":"x"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleUpdateAccount_UnknownField(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"description":"x","unknown":1}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleUpdateAccount' -v
```

Expected: FAIL — 404 across the board.

- [ ] **Step 3: Add the DTO and handler to `internal/api/accounts_write.go`**

Update the import block at the top of the file to add `"github.com/hance08/kea/internal/service"`. The final imports:

```go
import (
	"net/http"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)
```

Then append:

```go
type updateAccountRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsHidden    *bool   `json:"is_hidden,omitempty"`
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	var req updateAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.Name == nil && req.Description == nil && req.IsHidden == nil {
		return &service.ValidationError{Message: "no updatable fields provided"}
	}

	ctx := r.Context()
	current, err := s.svc.Account().GetAccountByID(ctx, id)
	if err != nil {
		return err
	}

	updated := current
	if req.Name != nil && *req.Name != current.Name {
		renamed, err := s.svc.Account().RenameAccount(ctx, current.Name, *req.Name)
		if err != nil {
			return err
		}
		updated = renamed
	}
	if req.Description != nil || req.IsHidden != nil {
		desc, hidden := updated.Description, updated.IsHidden
		if req.Description != nil {
			desc = *req.Description
		}
		if req.IsHidden != nil {
			hidden = *req.IsHidden
		}
		meta, err := s.svc.Account().UpdateAccountMetadata(ctx, updated.ID, desc, hidden)
		if err != nil {
			return err
		}
		updated = meta
	}
	return writeJSON(w, http.StatusOK, updated)
}
```

- [ ] **Step 4: Register the route in `internal/api/router.go`**

Append to the `/api` block:

```go
r.Method(http.MethodPatch, "/accounts/{id}", apiHandler(s.handleUpdateAccount))
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleUpdateAccount' -v
```

Expected: PASS for all nine tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/accounts_write.go internal/api/accounts_write_test.go internal/api/router.go
git commit -m "feat(api): PATCH /api/accounts/{id}"
```

---

## Task 6: `POST /api/transactions`

**Files:**
- Create: `internal/api/transactions_write.go`
- Modify: `internal/api/router.go`
- Create: `internal/api/transactions_write_test.go`

- [ ] **Step 1: Write failing tests for `POST /api/transactions`**

Create `internal/api/transactions_write_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestHandleCreateTransaction_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"Coffee",
		"timestamp":1700000000,
		"status":"Cleared",
		"type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
	var got model.TransactionDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID == 0 || got.Description != "Coffee" || got.Type != model.TxTypeExpense {
		t.Errorf("got %+v", got)
	}
	if len(got.Splits) != 2 {
		t.Fatalf("splits: got %d, want 2", len(got.Splits))
	}
	for _, s := range got.Splits {
		if s.AccountID == 0 || s.AccountName == "" || s.AccountType == "" {
			t.Errorf("split missing resolved fields: %+v", s)
		}
	}
}

func TestHandleCreateTransaction_Unbalanced(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":400}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateTransaction_OneSplit(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)

	body := `{
		"splits":[{"account_name":"Assets:Bank","amount":-500}],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "splits" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleCreateTransaction_TypeMismatch(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	// Splits describe an Expense, but type claims Income.
	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Income"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateTransaction_ReconciledOnCreate(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Reconciled","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "status" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleCreateTransaction_EmptyDescription(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateTransaction_MemoTooLong(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	longMemo := strings.Repeat("a", model.MemoMaxLength+1)
	body, err := json.Marshal(map[string]any{
		"splits": []map[string]any{
			{"account_name": "Assets:Bank", "amount": -500, "memo": longMemo},
			{"account_name": "Expenses:Coffee", "amount": 500},
		},
		"description": "x",
		"timestamp":   1700000000,
		"status":      "Cleared",
		"type":        "Expense",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp := postJSON(t, ts.URL+"/api/transactions", string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "memo" {
		t.Errorf("field: got %q, want memo", errBody["field"])
	}
}

func TestHandleCreateTransaction_HiddenAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	bank := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	if _, err := svc.Account().UpdateAccountMetadata(t.Context(), bank.ID, "", true); err != nil {
		t.Fatalf("hide: %v", err)
	}

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// A split referencing a nonexistent account currently returns 500 because
// CreateTransaction surfaces repository.ErrNotFound (not service.ErrNotFound),
// and mapError only matches the service-level sentinel. The spec records this
// as a known rough edge to be fixed outside this plan.
func TestHandleCreateTransaction_NonexistentAccount_Currently500(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:DoesNotExist","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500 (known rough edge — fix is out of scope)", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleCreateTransaction' -v
```

Expected: FAIL — route not registered.

- [ ] **Step 3: Create the handler in `internal/api/transactions_write.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) error {
	var input model.CreateTransactionFromSplitsInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	ctx := r.Context()
	detail, err := s.svc.Transaction().CreateTransactionFromSplits(ctx, input)
	if err != nil {
		return err
	}
	full, err := s.svc.Transaction().GetTransactionByID(ctx, detail.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, full)
}
```

- [ ] **Step 4: Register the route in `internal/api/router.go`**

Append to the `/api` block:

```go
r.Method(http.MethodPost, "/transactions", apiHandler(s.handleCreateTransaction))
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleCreateTransaction' -v
```

Expected: PASS for all nine tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/transactions_write.go internal/api/transactions_write_test.go internal/api/router.go
git commit -m "feat(api): POST /api/transactions"
```

---

## Task 7: `DELETE /api/transactions/{id}`

**Files:**
- Modify: `internal/api/transactions_write.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/transactions_write_test.go`

- [ ] **Step 1: Write failing tests for `DELETE /api/transactions/{id}`**

Append to `internal/api/transactions_write_test.go`:

```go
func TestHandleDeleteTransaction_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := deleteReq(t, ts.URL+"/api/transactions/"+itoa(d.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["deleted"] != true {
		t.Errorf("deleted: got %v", body["deleted"])
	}

	getResp, _ := http.Get(ts.URL + "/api/transactions/" + itoa(d.ID))
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: %d, want 404", getResp.StatusCode)
	}
}

func TestHandleDeleteTransaction_Pending(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusPending)

	resp := deleteReq(t, ts.URL+"/api/transactions/"+itoa(d.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestHandleDeleteTransaction_Reconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, d.ID)

	resp := deleteReq(t, ts.URL+"/api/transactions/"+itoa(d.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "reconciled" {
		t.Errorf("error: got %q, want reconciled", errBody["error"])
	}
}

func TestHandleDeleteTransaction_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/transactions/9999")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleDeleteTransaction_BadPath(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/transactions/abc")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleDeleteTransaction' -v
```

Expected: FAIL — route not registered.

- [ ] **Step 3: Add the handler to `internal/api/transactions_write.go`**

Append:

```go
func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.Transaction().DeleteTransaction(r.Context(), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}
```

- [ ] **Step 4: Register the route in `internal/api/router.go`**

```go
r.Method(http.MethodDelete, "/transactions/{id}", apiHandler(s.handleDeleteTransaction))
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleDeleteTransaction' -v
```

Expected: PASS for all five tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/transactions_write.go internal/api/transactions_write_test.go internal/api/router.go
git commit -m "feat(api): DELETE /api/transactions/{id}"
```

---

## Task 8: `PATCH /api/transactions/{id}/status`

**Files:**
- Modify: `internal/api/transactions_write.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/transactions_write_test.go`

- [ ] **Step 1: Write failing tests for the status sub-route**

Append to `internal/api/transactions_write_test.go`:

```go
func TestHandleUpdateTransactionStatus_PendingToCleared(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusPending)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Cleared"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.TransactionDetail
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Status != model.StatusCleared {
		t.Errorf("status: got %v, want Cleared", got.Status)
	}
}

func TestHandleUpdateTransactionStatus_ClearedToPending(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Pending"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestHandleUpdateTransactionStatus_RejectsReconciledTarget(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Reconciled"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "status" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleUpdateTransactionStatus_OnReconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, d.ID)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Cleared"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status: got %d, want 409", resp.StatusCode)
	}
}

func TestHandleUpdateTransactionStatus_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := patchJSON(t, ts.URL+"/api/transactions/9999/status", `{"status":"Cleared"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleUpdateTransactionStatus' -v
```

Expected: FAIL — route not registered.

- [ ] **Step 3: Add the DTO and handler to `internal/api/transactions_write.go`**

Append (the `model` import already exists):

```go
type updateStatusRequest struct {
	Status model.TransactionStatus `json:"status"`
}

func (s *Server) handleUpdateTransactionStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	var req updateStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	if err := s.svc.Transaction().UpdateTransactionStatus(ctx, id, req.Status); err != nil {
		return err
	}
	detail, err := s.svc.Transaction().GetTransactionByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, detail)
}
```

`TransactionStatus.UnmarshalJSON` accepts the string `"Reconciled"` and sets the status to `StatusReconciled`; the service-layer `UpdateTransactionStatus` then rejects it with `validationErrorf("status", ...)`. The 400 in `TestHandleUpdateTransactionStatus_RejectsReconciledTarget` exercises that service-level guard.

- [ ] **Step 4: Register the route in `internal/api/router.go`**

```go
r.Method(http.MethodPatch, "/transactions/{id}/status", apiHandler(s.handleUpdateTransactionStatus))
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleUpdateTransactionStatus' -v
```

Expected: PASS for all five tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/transactions_write.go internal/api/transactions_write_test.go internal/api/router.go
git commit -m "feat(api): PATCH /api/transactions/{id}/status"
```

---

## Task 9: `PATCH /api/transactions/{id}` (full replace)

**Files:**
- Modify: `internal/api/transactions_write.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/transactions_write_test.go`

- [ ] **Step 1: Write failing tests for the full-replace PATCH**

Append to `internal/api/transactions_write_test.go`:

```go
func TestHandleUpdateTransaction_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	if len(d.Splits) != 2 {
		t.Fatalf("seed splits: %d", len(d.Splits))
	}

	bodyMap := map[string]any{
		"description": "Updated coffee",
		"timestamp":   1700000001,
		"status":      "Cleared",
		"type":        "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -750, "currency": "USD"},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 750, "currency": "USD"},
		},
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.TransactionDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Description != "Updated coffee" || got.Timestamp != 1700000001 {
		t.Errorf("got %+v", got)
	}
	if len(got.Splits) != 2 {
		t.Fatalf("splits: %d", len(got.Splits))
	}
	if got.Splits[0].Amount != 750 && got.Splits[0].Amount != -750 {
		t.Errorf("amount: %d", got.Splits[0].Amount)
	}
}

func TestHandleUpdateTransaction_Reconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, d.ID)

	body, _ := json.Marshal(map[string]any{
		"description": "x",
		"timestamp":   1700000001,
		"status":      "Cleared",
		"type":        "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -500},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 500},
		},
	})

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "reconciled" {
		t.Errorf("error: got %q", errBody["error"])
	}
}

func TestHandleUpdateTransaction_BodyIDRejected(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	// Body carries "id":99 — DisallowUnknownFields should reject because
	// UpdateTransactionInput.ID is json:"-" (invisible to the decoder).
	body, _ := json.Marshal(map[string]any{
		"id":          99,
		"description": "x",
		"timestamp":   1700000001,
		"status":      "Cleared",
		"type":        "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -500},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 500},
		},
	})

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	body, _ := json.Marshal(map[string]any{
		"description": "x", "timestamp": 1, "status": "Cleared", "type": "Expense",
		"splits": []map[string]any{
			{"account_id": 1, "amount": -1},
			{"account_id": 2, "amount": 1},
		},
	})
	resp := patchJSON(t, ts.URL+"/api/transactions/9999", string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_Unbalanced(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	body, _ := json.Marshal(map[string]any{
		"description": "x", "timestamp": 1700000001, "status": "Cleared", "type": "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -500},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 400},
		},
	})
	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_ForeignSplitID(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d1 := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "T1", model.TxTypeExpense, model.StatusCleared)
	d2 := seedTransaction(t, svc, src.Name, dst.Name, 700, 1700000001, "T2", model.TxTypeExpense, model.StatusCleared)

	// Try to update d1 with a split ID belonging to d2.
	body, _ := json.Marshal(map[string]any{
		"description": "x", "timestamp": 1700000002, "status": "Cleared", "type": "Expense",
		"splits": []map[string]any{
			{"id": d2.Splits[0].ID, "account_id": d2.Splits[0].AccountID, "amount": -500},
			{"id": d1.Splits[1].ID, "account_id": d1.Splits[1].AccountID, "amount": 500},
		},
	})
	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d1.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "splits" {
		t.Errorf("field: got %q", errBody["field"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/ -run 'TestHandleUpdateTransaction_' -v
```

Expected: FAIL — route not registered. The `TestHandleUpdateTransactionStatus_*` tests from Task 8 are excluded by the `_` after `Transaction` in the test name pattern.

- [ ] **Step 3: Add the handler to `internal/api/transactions_write.go`**

Append:

```go
func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	var input model.UpdateTransactionInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	input.ID = id

	ctx := r.Context()
	if err := s.svc.Transaction().UpdateTransactionComplete(ctx, input); err != nil {
		return err
	}
	detail, err := s.svc.Transaction().GetTransactionByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, detail)
}
```

- [ ] **Step 4: Register the route in `internal/api/router.go`**

Append:

```go
r.Method(http.MethodPatch, "/transactions/{id}", apiHandler(s.handleUpdateTransaction))
```

The final state of the `/transactions` route group:

```go
r.Method(http.MethodGet,    "/transactions",             apiHandler(s.handleListTransactions))
r.Method(http.MethodGet,    "/transactions/{id}",        apiHandler(s.handleTransactionByID))
r.Method(http.MethodPost,   "/transactions",             apiHandler(s.handleCreateTransaction))
r.Method(http.MethodPatch,  "/transactions/{id}",        apiHandler(s.handleUpdateTransaction))
r.Method(http.MethodPatch,  "/transactions/{id}/status", apiHandler(s.handleUpdateTransactionStatus))
r.Method(http.MethodDelete, "/transactions/{id}",        apiHandler(s.handleDeleteTransaction))
```

Chi matches `/transactions/{id}/status` (the more specific route) before `/transactions/{id}` regardless of registration order, so route ordering is not load-bearing.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/ -run 'TestHandleUpdateTransaction_' -v
```

Expected: PASS for all six tests.

- [ ] **Step 6: Run the full api package tests**

```bash
go test ./internal/api/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/transactions_write.go internal/api/transactions_write_test.go internal/api/router.go
git commit -m "feat(api): PATCH /api/transactions/{id}"
```

---

## Task 10: Final verification — full test suite

This task confirms nothing elsewhere in the codebase regressed and the seven new routes coexist cleanly with the existing 14 read routes plus health/version.

**Files:** none modified.

- [ ] **Step 1: Build the project**

```bash
make build
```

Expected: success, produces `./kea_test`.

- [ ] **Step 2: Run the full Go test suite**

```bash
go test ./...
```

Expected: PASS across every package.

- [ ] **Step 3: Run `go vet` and `gofmt` checks**

```bash
go vet ./...
gofmt -l internal/api/ internal/model/
```

Expected: no output from either.

- [ ] **Step 4: Optional — smoke-test the running server**

In one shell, start the server:

```bash
./kea_test serve
```

In another shell:

```bash
# POST account
curl -sS -X POST http://localhost:8080/api/accounts \
  -H 'Content-Type: application/json' \
  -d '{"name":"Assets:SmokeTest","type":"A","currency":"USD","description":"","balance":0}'

# Replace ID with the value from the POST response
curl -sS -X DELETE http://localhost:8080/api/accounts/<ID> -w '\n%{http_code}\n'
```

If both succeed, stop the server with `Ctrl-C` and verify the graceful shutdown log line ("server shutting down") appears. This smoke step is optional — the `go test` pass above is sufficient evidence the endpoints work end-to-end.

- [ ] **Step 5: No commit needed**

This task does not modify code.

---

## Notes for the executor

- **Existing helpers are reused as-is.** `seedAccount` and `seedTransaction` from `testhelper_test.go` cover all the happy-path seeding write tests need; only reconciled-state seeding and parent-cycle injection required the new `newServerForWrite` / `seedReconciledTransaction` / `injectParentSelfCycle` helpers introduced in Task 2.
- **The `splits` array in `model.SplitDetail` JSON** carries `account_id` (not `account_name`) when it's the source of truth for an update. `CreateTransactionFromSplits` reads `account_name`; `UpdateTransactionComplete` reads `account_id`. The tests above respect this distinction.
- **Liability balance sign in `TestHandleCreateAccount_LiabilityBalanceSignReversed`:** the assertion uses `bal.Amount == -50000` because `createOpeningBalanceInRepo` sets the liability split to `-amountInCents`. If the service-layer convention ever inverts, the test will need updating; the assertion is intentional and matches the current service behavior described in `account_ops.go:179-187`.
- **The `TestHandleCreateTransaction_NonexistentAccount_Currently500` test asserts current behavior**, not desired behavior. The spec records this as a known rough edge to be fixed outside this plan. When the fix lands (likely a one-line change to translate `repository.ErrNotFound` → `service.ErrNotFound` in `CreateTransaction`'s split-resolution loop), update this test's expectation to 400 and rename it.
- **No changes outside `internal/api/`, `internal/model/`** are required for any task.
