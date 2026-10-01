# Fix `handleAccountBalance` Empty-Currency Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize empty `acc.Currency` to `config.Defaults.Currency` in `handleAccountBalance`'s response, mirroring the pattern in `GET /api/balances` and `GenerateBalanceSheet`.

**Architecture:** Single-handler change in `internal/api/accounts.go` (4 added lines). One new TDD-style test in `internal/api/accounts_test.go` that creates an empty-`Currency` account via `svc.Account().CreateAccount` directly (since `seedAccount` hardcodes `"USD"`), GETs `/api/accounts/{id}/balance`, asserts the response currency equals `svc.Config().Defaults.Currency`.

**Tech Stack:** Go stdlib (`net/http`, `encoding/json`), `testify`/`httptest` (existing test harness).

**Spec:** [`docs/superpowers/specs/2026-06-05-fix-account-balance-currency-normalization-design.md`](../specs/2026-06-05-fix-account-balance-currency-normalization-design.md)

---

## File Map

- Modify: `internal/api/accounts.go` — `handleAccountBalance` body (lines ~55–73). Replace the response build with a normalized version.
- Modify: `internal/api/accounts_test.go` — append `TestHandleAccountBalance_EmptyCurrencyNormalized`.

No new files, no new imports. `svc.Config()` is the existing service-facade accessor.

---

### Task 1: Write the failing test

**Files:**
- Modify: `internal/api/accounts_test.go` — append a new top-level test function at the end of the file.

Context: existing API tests use `newServerWithStore(t)` (returns `*httptest.Server` and `*service.Service`) and `seedAccount(t, svc, name, accType, balance)` (helpers in [internal/api/testhelper_test.go](../../internal/api/testhelper_test.go)). `seedAccount` hardcodes `Currency: "USD"` (line ~33), so it cannot produce an empty-currency account. The test must call `svc.Account().CreateAccount(...)` directly with `Currency: ""`.

`AccountService.ValidateCurrency` accepts `""` (returns `nil` with comment "empty is allowed, will use default"), so the create call will succeed.

`itoa` is the existing helper for `strconv.Itoa(int64)` used in other tests in this file (e.g. `TestHandleAccountByID_OK`).

- [ ] **Step 1: Append the new test function**

Add at the end of `internal/api/accounts_test.go`:

```go
// TestHandleAccountBalance_EmptyCurrencyNormalized verifies the handler
// resolves an empty account Currency to the config default, mirroring the
// behavior of GET /api/balances and GenerateBalanceSheet. ValidateCurrency
// explicitly allows empty input, so this case is reachable through normal
// account creation.
func TestHandleAccountBalance_EmptyCurrencyNormalized(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc, err := svc.Account().CreateAccount(t.Context(), model.CreateAccountInput{
		Name:     "Assets:NoCurrency",
		Type:     model.AccountTypeAsset,
		Currency: "",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	resp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got struct {
		AccountID int64  `json:"account_id"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := svc.Config().Defaults.Currency
	if got.Currency != want {
		t.Errorf("currency: got %q, want %q (config default)", got.Currency, want)
	}
	if got.AccountID != acc.ID {
		t.Errorf("account_id: got %d, want %d", got.AccountID, acc.ID)
	}
}
```

Imports needed in `accounts_test.go` (verify which are already present):
- `encoding/json` — already present (other tests decode JSON).
- `net/http` — already present.
- `testing` — already present.
- `github.com/hance08/kea/internal/model` — already present.

No new imports needed. `t.Context()` requires Go ≥ 1.24; the project uses Go 1.25.4 (verified in prior follow-ups in this session), so it works.

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/api/ -run TestHandleAccountBalance_EmptyCurrencyNormalized -v`

Expected: **FAIL** — the assertion `got.Currency != want` trips because the current handler returns `acc.Currency` (which is `""`) while `want` resolves to `svc.Config().Defaults.Currency` (likely `"USD"`). The expected failure message looks like:

```
currency: got "", want "USD" (config default)
```

If the test passes without the fix, the test is wrong — either `svc.Config().Defaults.Currency` is also empty (in which case the default config isn't what we think), or the handler is already doing something different. Stop and investigate.

If the test fails for a compile reason (e.g. `svc.Config` undefined, `CreateAccountInput` field name wrong), fix the test before proceeding.

---

### Task 2: Apply the handler fix

**Files:**
- Modify: `internal/api/accounts.go` — `handleAccountBalance` (around lines 55–73).

The current code:

```go
func (s *Server) handleAccountBalance(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.svc.Account().GetAccountByID(r.Context(), id)
	if err != nil {
		return err
	}
	amount, err := s.svc.Account().GetAccountBalance(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: id,
		Amount:    amount,
		Currency:  acc.Currency,
	})
}
```

- [ ] **Step 1: Replace the final return block**

Replace:

```go
	return writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: id,
		Amount:    amount,
		Currency:  acc.Currency,
	})
```

with:

```go
	currency := acc.Currency
	if currency == "" {
		currency = s.svc.Config().Defaults.Currency
	}
	return writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: id,
		Amount:    amount,
		Currency:  currency,
	})
```

No import changes — `s.svc.Config()` is already accessible through the existing `Server` struct's service facade.

- [ ] **Step 2: Run the new test and confirm it passes**

Run: `go test ./internal/api/ -run TestHandleAccountBalance_EmptyCurrencyNormalized -v`

Expected: **PASS**.

- [ ] **Step 3: Run the rest of the `TestHandleAccountBalance*` suite to confirm no regressions**

Run: `go test ./internal/api/ -run TestHandleAccountBalance -v`

Expected: all `TestHandleAccountBalance_*` tests PASS (the new one plus the existing happy-path cases that assert non-empty currencies). The normalization is a no-op for accounts whose `Currency` is already set.

---

### Task 3: Full-suite verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`

Expected: all packages PASS.

- [ ] **Step 2: Run a clean build**

Run: `go build ./...`

Expected: no output, exit 0.

- [ ] **Step 3: Stage exactly two files**

Run `git status`. Expected modified files:
- `internal/api/accounts.go`
- `internal/api/accounts_test.go`

The untracked `docs/web-layer/` directory stays untracked.

Stage:

```
git add internal/api/accounts.go internal/api/accounts_test.go
```

- [ ] **Step 4: Commit**

```
git commit -m "$(cat <<'EOF'
fix(api): normalize empty currency in /api/accounts/{id}/balance response

handleAccountBalance returned acc.Currency directly, so an account created
without a Currency (legal per ValidateCurrency) surfaced as "currency": "".
The bulk /api/balances endpoint and GenerateBalanceSheet already normalize
empty to config.Defaults.Currency; this commit applies the same pattern at
the handler layer, mirroring how handleListBalances composes the as_of
default inline.
EOF
)"
```

This project does not use a `Co-Authored-By` footer (verify with `git log -3 --format=%B` if uncertain). Do not add one.

Expected: commit succeeds, working tree clean for tracked files.

---

## Self-Review

**Spec coverage:**

- Spec §"Change" / `internal/api/accounts.go::handleAccountBalance` (4-line normalization): Task 2 ✓
- Spec §"Test" / new `TestHandleAccountBalance_EmptyCurrencyNormalized` (uses direct `CreateAccount`, asserts against `svc.Config().Defaults.Currency`): Task 1 ✓
- Spec §"Verification" (`go test ./internal/api/...`, `go test ./...`, `go build ./...`): Tasks 2–3 ✓
- Spec §"Commit shape" (single commit, `fix(api):` scope, verbatim body, no co-author): Task 3 Step 4 ✓
- Spec §"Out of scope": no tasks attempt `seedAccount` extension, service-layer signature change, or broader audit.

**Placeholder scan:**

- No TBDs, no "implement later", no "similar to Task N".
- Each code step shows verbatim before/after.
- Expected failure output for Task 1 Step 2 is concrete and falsifiable.

**Type/name consistency:**

- `TestHandleAccountBalance_EmptyCurrencyNormalized` referenced consistently in Task 1 (add) and Task 2 (re-run).
- `svc.Config().Defaults.Currency` accessor used consistently between the test (expected) and the handler (actual).
- `balanceResponse` struct fields (`AccountID`, `Amount`, `Currency`) match what `handleAccountBalance` already declares in the file.
- `CreateAccountInput` field names (`Name`, `Type`, `Currency`) verified against `internal/model/account.go` / `internal/service/account_ops.go::CreateAccount`.
