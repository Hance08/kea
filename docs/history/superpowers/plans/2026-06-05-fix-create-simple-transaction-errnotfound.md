# Fix `CreateSimpleTransaction` `ErrNotFound` Translation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Translate `repository.ErrNotFound` from the two `GetAccountByName` calls in `CreateSimpleTransaction`'s type-inference branch into per-side `*ValidationError`s (`from_account` / `to_account`), mirroring PR #180's pattern.

**Architecture:** Two parallel condition translations in `internal/service/transaction_ops.go::CreateSimpleTransaction` (~10 added lines total). Two new subtests inside the existing `TestCreateSimpleTransaction` t.Run block, both setting `Type: ""` to enter the type-inference branch. Non-`ErrNotFound` branches keep their existing `fmt.Errorf` wraps so real repo failures continue to surface.

**Tech Stack:** Go stdlib (`errors`, `fmt`), `testify/{assert,require}` (existing test imports).

**Spec:** [`docs/superpowers/specs/2026-06-05-fix-create-simple-transaction-errnotfound-design.md`](../specs/2026-06-05-fix-create-simple-transaction-errnotfound-design.md)

---

## File Map

- Modify: `internal/service/transaction_ops.go` — `CreateSimpleTransaction` type-inference branch (lines ~171–179). Two parallel translations.
- Modify: `internal/service/transaction_ops_test.go` — append two new `t.Run` subtests inside `TestCreateSimpleTransaction` (around line 381).

No new files, no new imports (`errors`, `fmt`, `repository`, `model` already imported in `transaction_ops.go`; `errors`, `assert`, `require`, `model` already in the test file).

---

### Task 1: Add the failing service-layer tests

**Files:**
- Modify: `internal/service/transaction_ops_test.go` — append two new subtests inside the existing `TestCreateSimpleTransaction` function (after the existing "empty type infers from account types" subtest near line 451; before the closing `}` of `TestCreateSimpleTransaction`).

Context: existing tests use `newMockAccountRepo()`, `newMockTransactionRepo()`, `setupStandardAccounts(accRepo)`, and `newTestTransactionService(accRepo, txRepo)`. The mock at `internal/service/testhelper_test.go:112` returns `fmt.Errorf("account %q not found: %w", name, repository.ErrNotFound)` for unknown names — exactly the production wrap shape.

The contract being pinned: when an unknown account name reaches the type-inference branch (`Type: ""`), `CreateSimpleTransaction` returns a `*ValidationError` with `Field` set to `from_account` or `to_account` (matching which input field was wrong) and a `Message` that includes the offending account name.

- [ ] **Step 1: Locate the closing brace of `TestCreateSimpleTransaction`**

Run: `grep -n "func TestCreateSimpleTransaction\|^}" internal/service/transaction_ops_test.go | head -10`

Find the `func TestCreateSimpleTransaction(t *testing.T) {` line (~381) and the matching closing `}` of that function. The closing brace sits right before the next top-level test (`TestDeleteTransaction`, around line 463). Confirm visually with `sed -n '450,465p' internal/service/transaction_ops_test.go`.

- [ ] **Step 2: Append the two new subtests inside the function**

Insert the following two `t.Run` blocks immediately before the closing `}` of `TestCreateSimpleTransaction`. Keep the existing trailing newline / formatting of the file.

```go
	t.Run("unknown from account returns validation error on from_account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		_, err := svc.CreateSimpleTransaction(
			context.Background(),
			model.CreateSimpleTransactionInput{
				FromAccount: "Assets:NotReal",
				ToAccount:   "Expenses:Food",
				Amount:      100,
				Description: "x",
				Timestamp:   0,
				Status:      model.StatusPending,
				Type:        "", // empty -> enters the type-inference branch
			},
		)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T: %v", err, err)
		assert.Equal(t, "from_account", ve.Field)
		assert.Contains(t, ve.Message, "Assets:NotReal")
	})

	t.Run("unknown to account returns validation error on to_account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		_, err := svc.CreateSimpleTransaction(
			context.Background(),
			model.CreateSimpleTransactionInput{
				FromAccount: "Assets:Bank",
				ToAccount:   "Expenses:NotReal",
				Amount:      100,
				Description: "x",
				Timestamp:   0,
				Status:      model.StatusPending,
				Type:        "", // empty -> enters the type-inference branch
			},
		)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T: %v", err, err)
		assert.Equal(t, "to_account", ve.Field)
		assert.Contains(t, ve.Message, "Expenses:NotReal")
	})
```

Both subtests deliberately set `Type: ""` to enter the type-inference branch. If `Type` is non-empty, `CreateSimpleTransaction` skips the branch and delegates to `CreateTransaction`, which has its own (already-fixed) translation — different code path, different test.

`setupStandardAccounts` seeds `Assets:Bank` and `Expenses:Food` (plus others); the first test uses a real `Expenses:Food` for `ToAccount` so the failure is unambiguously on `FromAccount`, and vice versa.

`errors`, `assert`, `require`, `model` are already imported in the test file (used by neighbor tests). No import changes.

- [ ] **Step 3: Run the new tests and confirm they fail**

Run: `go test ./internal/service/ -run 'TestCreateSimpleTransaction/(unknown_from_account|unknown_to_account)' -v`

Expected: both new subtests **FAIL** because the current code returns a plain `fmt.Errorf("failed to resolve ... account: %w", err)` whose underlying error is `repository.ErrNotFound`, not a `*ValidationError`. The `errors.As(err, &ve)` call returns `false`, tripping the `require.True`. Sample expected failure:

```
expected *ValidationError, got *fmt.wrapError: failed to resolve from account: account "Assets:NotReal" not found: record not found
```

If either test compiles but passes, the test setup is wrong — STOP and re-check.

---

### Task 2: Apply the two parallel translations

**Files:**
- Modify: `internal/service/transaction_ops.go` — `CreateSimpleTransaction` type-inference branch (lines ~171–179).

The current code:

```go
	// If no type provided, infer from account types.
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(ctx, input.FromAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(ctx, input.ToAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
```

- [ ] **Step 1: Replace the two error blocks**

Replace the block above with:

```go
	// If no type provided, infer from account types.
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(ctx, input.FromAccount)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return model.TransactionDetail{}, validationErrorf("from_account",
					"from account %q not found", input.FromAccount)
			}
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(ctx, input.ToAccount)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return model.TransactionDetail{}, validationErrorf("to_account",
					"to account %q not found", input.ToAccount)
			}
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
```

No surrounding lines change. No imports change — `errors`, `fmt`, and `repository` are already used elsewhere in this file (confirmed by PR #180's translation at the split loop earlier in the function).

- [ ] **Step 2: Run the new tests and confirm they pass**

Run: `go test ./internal/service/ -run 'TestCreateSimpleTransaction/(unknown_from_account|unknown_to_account)' -v`

Expected: both new subtests **PASS**.

- [ ] **Step 3: Run the full `TestCreateSimpleTransaction` suite to catch regressions**

Run: `go test ./internal/service/ -run TestCreateSimpleTransaction -v`

Expected: all 8 subtests PASS (6 pre-existing + 2 new). The "empty type infers from account types" subtest in particular must still pass — it exercises the happy path through the type-inference branch with valid account names.

---

### Task 3: Full-suite verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`

Expected: every package PASS. Pay particular attention to the CLI's `cmd/add` tests — they exercise the CLI's wrapper around `CreateSimpleTransaction`. The CLI doesn't inspect the error type, just prints it, so the translation should be transparent to them.

- [ ] **Step 2: Clean build**

Run: `go build ./...`

Expected: no output, exit 0.

- [ ] **Step 3: Stage the two files**

Run `git status`. Expected modified files:
- `internal/service/transaction_ops.go`
- `internal/service/transaction_ops_test.go`

The untracked `docs/web-layer/` directory stays untracked. Stage:

```
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
```

Verify with `git status` that only those two are staged.

- [ ] **Step 4: Commit**

```
git commit -m "$(cat <<'EOF'
fix(service): translate repository.ErrNotFound in CreateSimpleTransaction type inference

CreateSimpleTransaction's type-inference branch wrapped GetAccountByName
errors unconditionally, leaking repository.ErrNotFound past the service
boundary. The web API never reaches this path (it requires Type to be set),
but the CLI's `add` command does — and any future API endpoint mirroring
the CLI's ergonomics would have inherited the leak.

Mirror the pattern just applied to CreateTransaction's split loop
(commit a8a08e2): translate repository.ErrNotFound to validationErrorf
with from_account / to_account field tags. The non-NotFound branch keeps
its existing wrap so genuine repo failures continue to surface.

Closes the audit identified in the 2026-06-05 ErrNotFound translation work.
EOF
)"
```

This project does not use a `Co-Authored-By` footer (verify with `git log -3 --format=%B HEAD~..HEAD~3`). Do not add one.

Expected: commit succeeds, working tree clean for tracked files.

---

## Self-Review

**Spec coverage:**

- Spec §"Change" (`internal/service/transaction_ops.go` — two parallel translations with `from_account` / `to_account` fields): Task 2 ✓
- Spec §"Tests" (two new subtests with `Type: ""`, `errors.As` for `*ValidationError`, field + message-contains assertions): Task 1 ✓
- Spec §"Verification" (`go test ./...`, `go build ./...`): Task 3 Steps 1–2 ✓
- Spec §"Commit shape" (single commit, `fix(service):` scope, body verbatim, no co-author): Task 3 Step 4 ✓
- Spec §"Out of scope" — no tasks attempt CLI changes, broader audit, or DetermineType modifications ✓

**Placeholder scan:**

- No TBDs, no "implement later", no "similar to Task N".
- Every code step shows the exact before/after.
- Expected failure output in Task 1 Step 3 is concrete and falsifiable.

**Type/name consistency:**

- `from_account` / `to_account` field names match between Task 1 (test assertions) and Task 2 (implementation).
- `validationErrorf(field, format, args...)` signature matches the existing helper in `internal/service/errors.go` (used throughout the file by the prior PR #180 fix).
- Subtest names quoted in Task 1 (`"unknown from account returns validation error on from_account"` / `"unknown to account returns validation error on to_account"`) match the test-run filter pattern in Task 1 Step 3 (`unknown_from_account|unknown_to_account` — Go test name normalization replaces spaces with underscores).
- `CreateSimpleTransactionInput` field names (`FromAccount`, `ToAccount`, `Amount`, `Description`, `Timestamp`, `Status`, `Type`) match the struct definition in `internal/model/input.go:15`.
