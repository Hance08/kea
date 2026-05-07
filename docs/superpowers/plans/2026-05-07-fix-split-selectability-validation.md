# Fix Split Selectability Validation (Issue #42) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `CreateTransaction` from posting splits to hidden or parent accounts, closing the bypass that exists in the `--split` CLI path.

**Architecture:** Add selectability validation (hidden check + leaf-account check) inside `TransactionService.CreateTransaction`, in the existing per-split loop that resolves account names to IDs. This protects all callers uniformly. The opening-balance path is unaffected because it calls `repo.CreateTransactionWithSplits` directly, bypassing `CreateTransaction`.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add failing tests for hidden and parent account rejection

**Files:**
- Modify: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Write the failing test for hidden account rejection**

Add this test inside `TestCreateTransaction`:

```go
t.Run("split referencing hidden account rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    setupStandardAccounts(accRepo)
    accRepo.addAccount(&model.Account{ID: 10, Name: "Assets:Old", Type: model.AccountTypeAsset, IsHidden: true})
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    input := model.TransactionDetail{
        Description: "hidden test",
        Type:        model.TxTypeExpense,
        Splits: []model.SplitDetail{
            {AccountName: "Assets:Old", Amount: 500},
            {AccountName: "Expenses:Food", Amount: -500},
        },
    }
    _, err := svc.CreateTransaction(context.Background(), input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "split #1")
    assert.Contains(t, err.Error(), "hidden")
})
```

- [ ] **Step 2: Write the failing test for parent account rejection**

Add this test inside `TestCreateTransaction`:

```go
t.Run("split referencing parent account rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    setupStandardAccounts(accRepo)
    accRepo.addAccount(&model.Account{ID: 11, Name: "Assets", Type: model.AccountTypeAsset})
    accRepo.childMap[11] = true
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    input := model.TransactionDetail{
        Description: "parent test",
        Type:        model.TxTypeExpense,
        Splits: []model.SplitDetail{
            {AccountName: "Assets", Amount: 500},
            {AccountName: "Expenses:Food", Amount: -500},
        },
    }
    _, err := svc.CreateTransaction(context.Background(), input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "split #1")
    assert.Contains(t, err.Error(), "parent account")
})
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateTransaction/split_referencing_hidden|TestCreateTransaction/split_referencing_parent" -v`

Expected: Both FAIL — no hidden/parent check exists yet.

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_ops_test.go
git commit -m "test: add failing tests for hidden and parent account split rejection

Covers issue #42 — CreateTransaction does not yet reject splits
that reference hidden or parent accounts.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Add selectability validation in CreateTransaction

**Files:**
- Modify: `internal/service/transaction_ops.go:45-50` (inside the `for i, splitInput` loop)

- [ ] **Step 1: Add hidden and parent-account checks after account lookup**

In `CreateTransaction`, immediately after the `GetAccountByName` call and its error check (line 49), add:

```go
if account.IsHidden {
    return 0, fmt.Errorf("split #%d: account %q is hidden", i+1, account.Name)
}

hasChildren, err := ts.accRepo.HasChildAccounts(ctx, account.ID)
if err != nil {
    return 0, fmt.Errorf("split #%d: %w", i+1, err)
}
if hasChildren {
    return 0, fmt.Errorf("split #%d: account %q is a parent account; select a leaf account instead", i+1, account.Name)
}
```

The full loop body (lines 45–65) should now read:

```go
for i, splitInput := range input.Splits {
    account, err := ts.accRepo.GetAccountByName(ctx, splitInput.AccountName)
    if err != nil {
        return 0, fmt.Errorf("split #%d: %w", i+1, err)
    }

    if account.IsHidden {
        return 0, fmt.Errorf("split #%d: account %q is hidden", i+1, account.Name)
    }

    hasChildren, err := ts.accRepo.HasChildAccounts(ctx, account.ID)
    if err != nil {
        return 0, fmt.Errorf("split #%d: %w", i+1, err)
    }
    if hasChildren {
        return 0, fmt.Errorf("split #%d: account %q is a parent account; select a leaf account instead", i+1, account.Name)
    }

    splitCurrency := currency
    if account.Currency != "" {
        splitCurrency = account.Currency
    }

    splits = append(splits, model.Split{
        AccountID: account.ID,
        Amount:    splitInput.Amount,
        Currency:  splitCurrency,
        Memo:      splitInput.Memo,
    })
}
```

- [ ] **Step 2: Run the two new tests to verify they pass**

Run: `go test ./internal/service/ -run "TestCreateTransaction/split_referencing_hidden|TestCreateTransaction/split_referencing_parent" -v`

Expected: PASS

- [ ] **Step 3: Run the full test suite to check for regressions**

Run: `go test ./...`

Expected: All tests PASS. The existing `setupStandardAccounts` accounts (`Assets:Bank`, `Expenses:Food`, etc.) have `IsHidden: false` (zero value) and `childMap` defaults to `false`, so existing tests are unaffected.

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_ops.go
git commit -m "fix: reject splits referencing hidden or parent accounts in CreateTransaction

Adds selectability validation inside the per-split loop so every
caller (--split, --from/--to, CreateTransactionFromSplits) gets the
same protection. Error messages include the split index for
scriptability.

Fixes #42

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Remove the now-redundant comment in cmd/add_actions.go

**Files:**
- Modify: `cmd/add_actions.go:267-268`

- [ ] **Step 1: Remove the stale comment**

The comment on lines 267–268 says validation is deferred to `CreateTransaction` for name validity only. Now that `CreateTransaction` also handles selectability, the comment is misleading. Delete these two lines:

```go
// Account name validity and selectability are validated by CreateTransaction
// (which calls GetAccountByName per split). Errors propagate with split index context.
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`

Expected: All PASS (no behavior change).

- [ ] **Step 3: Commit**

```bash
git add cmd/add_actions.go
git commit -m "chore: remove stale validation comment in split builder

The comment described the old behavior; CreateTransaction now enforces
selectability directly.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```
