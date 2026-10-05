# Fix Balance Sheet Zero-Balance Account Omission — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Include zero-balance accounts in the balance sheet report instead of silently skipping them.

**Architecture:** Remove the `balance == 0` early-continue in `GenerateBalanceSheet`, so all accounts appear in the report regardless of balance. Update the existing test that asserts zero-balance exclusion to instead assert inclusion.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Update the test to expect zero-balance accounts included

**Files:**
- Modify: `internal/service/report_service_test.go:685-697`

- [ ] **Step 1: Rewrite the existing test case**

The test at line 685 currently asserts zero-balance accounts are *excluded*. Change it to assert they are *included* with amount 0:

```go
t.Run("accounts with zero balance are included in the report", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Empty", Type: model.AccountTypeAsset})
    accRepo.balances[1] = 5000
    accRepo.balances[2] = 0
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
    require.NoError(t, err)
    assert.Len(t, result.Assets, 2)
    assert.Equal(t, "Assets:Bank", result.Assets[0].AccountName)
    assert.Equal(t, int64(5000), result.Assets[0].Amount)
    assert.Equal(t, "Assets:Empty", result.Assets[1].AccountName)
    assert.Equal(t, int64(0), result.Assets[1].Amount)
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/service/ -run TestGenerateBalanceSheet/accounts_with_zero_balance -v`

Expected: FAIL — currently only 1 asset row is returned because `balance == 0` skips the account.

---

### Task 2: Remove the zero-balance skip

**Files:**
- Modify: `internal/service/report_service.go:308-310`

- [ ] **Step 3: Delete the zero-balance guard**

Remove these three lines (308–310):

```go
if balance == 0 {
    continue
}
```

The loop body starting at line 312 (`currency := acc.Currency`) now runs for every account unconditionally.

- [ ] **Step 4: Run all balance sheet tests to verify they pass**

Run: `go test ./internal/service/ -run TestGenerateBalanceSheet -v`

Expected: All subtests PASS, including the updated zero-balance test.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`

Expected: All packages PASS with no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "fix: include zero-balance accounts in balance sheet report (closes #23)"
```
