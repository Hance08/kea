# Fix `CreateTransaction` Split-Account `ErrNotFound` Translation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Translate `repository.ErrNotFound` to a service-layer `ValidationError` inside `CreateTransaction`'s split lookup, so the API returns 400 with field `"splits"` instead of 500 when a request body references a nonexistent account name.

**Architecture:** One-condition fix at the service boundary in `internal/service/transaction_ops.go`. Mirrors the existing `UpdateTransaction` handling of the same logical error (`internal/service/transaction_ops.go:350`). No changes to the API error mapper, the repository, or mocks.

**Tech Stack:** Go 1.x, stdlib `errors`, `testify/assert`/`require`, `net/http/httptest`.

**Spec:** [`docs/superpowers/specs/2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`](../specs/2026-06-05-fix-create-transaction-split-account-errnotfound-design.md)

---

## File Map

- Modify: `internal/service/transaction_ops.go` — `CreateTransaction`, the split loop (≈ lines 60–66). Add typed translation of `repository.ErrNotFound` into `validationErrorf("splits", ...)`.
- Modify: `internal/service/transaction_ops_test.go` — add `TestCreateTransaction_UnknownSplitAccount_ReturnsValidationError` as a top-level test function.
- Modify: `internal/api/transactions_write_test.go` — rename `TestHandleCreateTransaction_NonexistentAccount_Currently500` to `TestHandleCreateTransaction_NonexistentAccount`, drop the stale "Known rough edge" comment block above it (lines ≈ 208–210), and assert 400 + `error == "validation_failed"` + `field == "splits"`.
- Modify (local-only, gitignored): `docs/superpowers/specs/2026-06-03-web-api-write-endpoints-design.md` — replace three stale "Known rough edge" references with a resolution pointer to the new spec.

No new files. No new imports. `repository` is already imported in `transaction_ops.go`.

---

### Task 1: Add a failing service-layer test for unknown split account

**Files:**
- Modify: `internal/service/transaction_ops_test.go` — append a new top-level test function (after the existing `TestCreateTransaction` block, before the next top-level test).
- Test: `internal/service/transaction_ops_test.go` (same file — service-layer tests are colocated)

Context: existing tests use the `newTestTransactionService(accRepo, txRepo)` factory and the `setupStandardAccounts(accRepo)` helper from `testhelper_test.go`. `setupStandardAccounts` seeds `Assets:Bank` and `Expenses:Food` among others. The mock `GetAccountByName` returns `fmt.Errorf("account %q not found: %w", name, repository.ErrNotFound)` for unknown names (`testhelper_test.go:112`), which exactly reproduces the production bug shape.

The API contract we are establishing: `CreateTransaction` returns an error that satisfies `errors.As(&ve)` for `*ValidationError`, with `ve.Field == "splits"` and a message that includes the offending name.

- [ ] **Step 1: Add the new test function**

Append to `internal/service/transaction_ops_test.go`:

```go
// TestCreateTransaction_UnknownSplitAccount_ReturnsValidationError pins the
// service contract: when a request references a nonexistent split account
// name, CreateTransaction must return a *ValidationError tagged with the
// "splits" field, not a wrapped repository.ErrNotFound. The latter would
// fall through mapError to HTTP 500; the former maps to 400.
func TestCreateTransaction_UnknownSplitAccount_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	setupStandardAccounts(accRepo)
	svc := newTestTransactionService(accRepo, newMockTransactionRepo())

	input := model.TransactionDetail{
		Type:        model.TxTypeExpense,
		Description: "lunch",
		Status:      model.StatusCleared,
		Timestamp:   1700000000,
		Splits: []model.SplitDetail{
			{AccountName: "Assets:Bank", Amount: -500},
			{AccountName: "Expenses:DoesNotExist", Amount: 500},
		},
	}

	_, err := svc.CreateTransaction(context.Background(), input)
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T: %v", err, err)
	assert.Equal(t, "splits", ve.Field)
	assert.Contains(t, ve.Message, "Expenses:DoesNotExist")
}
```

If `errors` is not already imported in the file's import block, add it. (Most tests in this file already use `errors.Is`, so it usually is.)

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/service/ -run TestCreateTransaction_UnknownSplitAccount_ReturnsValidationError -v`

Expected: **FAIL.** The current code path is `fmt.Errorf("split #%d: %w", i+1, err)`, which produces a plain wrapped error — `errors.As(err, &ve)` returns `false`, tripping the `require.True` and stopping the test.

Do not proceed to Task 2 until this test fails for the right reason (wrong error type, not "function not defined" or compile error). If it compiles but passes, the test is wrong — re-check.

---

### Task 2: Implement the service-layer translation

**Files:**
- Modify: `internal/service/transaction_ops.go` — `CreateTransaction`, the `for i, splitInput := range input.Splits` loop near line 60. Currently:

```go
account, err := ts.accRepo.GetAccountByName(ctx, splitInput.AccountName)
if err != nil {
    return 0, fmt.Errorf("split #%d: %w", i+1, err)
}
```

- [ ] **Step 1: Apply the edit**

Replace the four-line block above with:

```go
account, err := ts.accRepo.GetAccountByName(ctx, splitInput.AccountName)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return 0, validationErrorf("splits",
            "split #%d: account %q not found", i+1, splitInput.AccountName)
    }
    return 0, fmt.Errorf("split #%d: %w", i+1, err)
}
```

Rationale: typed translation at the service boundary. The non-`ErrNotFound` branch preserves the existing wrap so genuine repository failures (DB errors, context cancellation) still flow to `mapError`'s default 500 branch.

No imports change. `errors`, `fmt`, and `repository` are already imported (confirmed; the `repository.ErrNotFound` package alias is the same one used elsewhere in this file at lines 231, 254, 284, 350).

- [ ] **Step 2: Run the service-layer test and confirm it passes**

Run: `go test ./internal/service/ -run TestCreateTransaction_UnknownSplitAccount_ReturnsValidationError -v`

Expected: **PASS.**

- [ ] **Step 3: Run the rest of the CreateTransaction suite to catch regressions**

Run: `go test ./internal/service/ -run TestCreateTransaction -v`

Expected: all `TestCreateTransaction*` cases pass (the OK cases, fewer-than-2-splits, missing type, description validation, etc.). The translation only changes behavior on `repository.ErrNotFound` from `GetAccountByName`; nothing else should move.

---

### Task 3: Flip the API test from 500 to 400

**Files:**
- Modify: `internal/api/transactions_write_test.go` — the test at lines ≈ 208–229.

Current state (verbatim from the file):

```go
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
	resp := postJSONStr(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500 (known rough edge — fix is out of scope)", resp.StatusCode)
	}
}
```

The new contract: 400, `error == "validation_failed"`, `field == "splits"`. The API's error envelope is `errorBody{Error, Message, Field}` (`internal/api/errors.go:15`).

- [ ] **Step 1: Replace the test verbatim with the new version**

Replace the entire block above (comment lines and function body together) with:

```go
func TestHandleCreateTransaction_NonexistentAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:DoesNotExist","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSONStr(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var got struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Field   string `json:"field"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error != "validation_failed" {
		t.Errorf("error code: got %q, want %q", got.Error, "validation_failed")
	}
	if got.Field != "splits" {
		t.Errorf("field: got %q, want %q", got.Field, "splits")
	}
	if !strings.Contains(got.Message, "Expenses:DoesNotExist") {
		t.Errorf("message should mention the unknown account; got %q", got.Message)
	}
}
```

The inline anonymous struct mirrors the envelope from `internal/api/errors.go`. `json`, `strings`, `net/http`, and `model` are already imported (confirmed in lines 1–13 of the file).

- [ ] **Step 2: Run the flipped test and confirm it passes**

Run: `go test ./internal/api/ -run TestHandleCreateTransaction_NonexistentAccount -v`

Expected: **PASS.** Service now returns a `*ValidationError`; `mapError` returns 400 with `error="validation_failed"` and `field="splits"`; the assertions match.

- [ ] **Step 3: Run the full `TestHandleCreateTransaction*` suite to catch regressions**

Run: `go test ./internal/api/ -run TestHandleCreateTransaction -v`

Expected: all `TestHandleCreateTransaction*` cases pass. The flip only affects the previously-500 case; other cases (OK, Unbalanced, OneSplit, TypeMismatch, ReconciledOnCreate, EmptyDescription, MemoTooLong, HiddenAccount, ParentAccountInSplit) keep their existing assertions.

---

### Task 4: Full-suite verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`

Expected: all packages pass. Pay particular attention to `internal/service` and `internal/api`.

- [ ] **Step 2: Run a clean build**

Run: `go build ./...`

Expected: no output, exit 0.

- [ ] **Step 3: Stage the three modified files**

Run:

```
git add internal/service/transaction_ops.go \
        internal/service/transaction_ops_test.go \
        internal/api/transactions_write_test.go
```

Verify with `git status` that exactly those three files are staged and nothing else (the `docs/web-layer/` untracked directory mentioned in the session start is unrelated).

- [ ] **Step 4: Commit**

Use HEREDOC to preserve the blank line between subject and body:

```
git commit -m "$(cat <<'EOF'
fix(service): translate repository.ErrNotFound to validation error in CreateTransaction split lookup

A request body referencing a nonexistent split account name previously
surfaced as HTTP 500 because the service layer wrapped repository.ErrNotFound
without translating it. CreateTransaction now mirrors UpdateTransaction's
existing handling (transaction_ops.go:350) and returns a
validationErrorf("splits", ...), yielding 400 with a field-tagged envelope.
EOF
)"
```

Conventional commit, scope `(service)`. The API test flip is a contract change paired with the service fix, so a single commit is correct.

Expected: commit succeeds, working tree clean for tracked files.

---

### Task 5: Tidy stale notes in the predecessor spec (local-only)

**Files:**
- Modify (gitignored, local-only): `docs/superpowers/specs/2026-06-03-web-api-write-endpoints-design.md`

The predecessor spec carries three references to this rough edge that are now stale. Because `docs/superpowers/` is gitignored, this is local hygiene — no commit.

- [ ] **Step 1: Replace the "Known rough edge" section (line ≈ 282)**

Locate the heading `### Known rough edge — \`repository.ErrNotFound\` in split account lookup` and the paragraph immediately below it. Replace the whole section with:

```markdown
### Known rough edge — `repository.ErrNotFound` in split account lookup *(Resolved)*

Resolved in [`2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`](2026-06-05-fix-create-transaction-split-account-errnotfound-design.md). The service layer now translates `repository.ErrNotFound` from `GetAccountByName` into a `validationErrorf("splits", ...)`, yielding 400 with `error="validation_failed"` and `field="splits"` — matching `UpdateTransaction`'s existing behavior.
```

- [ ] **Step 2: Update the test-inventory line (line ≈ 329)**

Find the bullet `- Split referencing a nonexistent account → currently returns 500 (see "Known rough edge" above); test asserts the current behavior and notes the fix is out of scope.`

Replace with:

```markdown
- Split referencing a nonexistent account → returns 400 with `field="splits"` (resolved by `2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`).
```

- [ ] **Step 3: Update the deferred-work line (line ≈ 394)**

Find the bullet `- Fix for \`repository.ErrNotFound\` vs \`service.ErrNotFound\` mismatch in split account resolution — flagged above, change lives outside this spec.`

Replace with:

```markdown
- ~~Fix for `repository.ErrNotFound` vs `service.ErrNotFound` mismatch in split account resolution~~ — resolved in `2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`.
```

- [ ] **Step 4: No commit**

`docs/superpowers/` is gitignored (`.gitignore:57`). Skip `git add`/`git commit` for this step. Confirm with `git status` that the file is not listed as modified (it should not be, because git doesn't track it).

---

## Self-Review

Cross-check against the spec:

- Spec §"Change" item 1 (service-layer translation): Task 2 ✓
- Spec §"Change" item 2 (new service-layer test): Task 1 ✓
- Spec §"Change" item 3 (flip API test, drop stale comment, assert 400 / `validation_failed` / `field == "splits"`): Task 3 ✓
- Spec §"Change" item 4 (tidy stale notes in predecessor spec): Task 5 ✓
- Spec §"Verification": Task 4 ✓
- Spec §"Commit shape" (single commit, fix(service): ... message): Task 4 step 4 ✓

Type/name consistency:
- `validationErrorf` signature: `(field, format string, args ...any) *ValidationError` — matches `internal/service/errors.go:30`.
- API envelope JSON keys: `error` / `message` / `field` — matches `internal/api/errors.go:15`.
- Validation error code: `"validation_failed"` — matches `internal/api/errors.go:47` and `errors_test.go:23`.

Placeholder scan: no TBDs, no "implement appropriately", no "similar to Task N"; all code blocks are verbatim or precise edits.
