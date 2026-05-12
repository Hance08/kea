# Fix Liability Report Sign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `GetNetWorthAt` and `GenerateBalanceSheet` so liability balances (stored as negative values) are normalized to positive obligations before computing net worth and display totals.

**Architecture:** The domain stores liability balances as negative values (a $2000 credit card debt is stored as -2000 cents on the liability account). Reports must normalize these to positive amounts before subtraction (`net worth = assets - |liabilities|`) and before rendering in the balance sheet. The fix uses `utils.AbsInt64` at the report boundary — storage is unchanged.

**Tech Stack:** Go, testify

---

## Background

**How liability values flow through the system:**

1. Opening balance for a $20 liability creates: liability split = -2000, equity split = +2000 (per CLAUDE.md rules)
2. `GetAllAccountBalances` sums splits → liability account balance = -2000
3. `GetSplitsWithAccountsByDateRange` returns splits with their raw amounts

**Current bug in `GetNetWorthAt` (report_service.go:21-33):**
- `liabilities["USD"] += split.Amount` → liabilities = -2000
- `nw = assets - liabilities` → `assets - (-2000)` = assets + 2000 (WRONG)
- Should be: assets - 2000

**Current bug in `GenerateBalanceSheet` (report_service.go:212-225):**
- `TotalLiabilities[ccy] += balance` where balance is -2000 → TotalLiabilities = -2000
- `NetWorth = assets - (-2000)` = assets + 2000 (WRONG)
- Liability rows also display with negative amounts, inconsistent with the account list which strips the sign

**Existing tests use positive liability values (e.g., balance = 3000), which masks the bug.** The tests must be updated to use realistic negative values.

---

### Task 1: Fix `GetNetWorthAt` — update tests to use realistic negative liability values, then fix

**Files:**
- Modify: `internal/service/report_service.go:21-33`
- Modify: `internal/service/report_service_test.go:97-169`

- [ ] **Step 1: Update existing `GetNetWorthAt` tests to use negative liability splits**

In `report_service_test.go`, update the "assets minus liabilities" test (line 97). Change the liability split amount from `5000` to `-5000` (realistic: liability splits are negative). Update expected values:

```go
t.Run("assets minus liabilities", func(t *testing.T) {
    txRepo := newMockTransactionRepo()
    addTxSplits(txRepo.splitsWithAccts, 1,
        model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 15000, Currency: "USD"},
        model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -15000, Currency: "USD"},
    )
    addTxSplits(txRepo.splitsWithAccts, 2,
        model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -5000, Currency: "USD"},
        model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: 5000, Currency: "USD"},
    )
    svc := newTestTransactionService(newMockAccountRepo(), txRepo)

    // totalAssets = 15000, totalLiabilities = abs(-5000) = 5000
    // netWorth = 15000 - 5000 = 10000
    nw, err := svc.GetNetWorthAt(context.Background(), 0)
    require.NoError(t, err)
    assert.Equal(t, int64(10000), nw["USD"])
})
```

Also update the "multi-currency" test (line 146). Change the liability split from `Amount: 2000` to `Amount: -2000`, and update the offset asset split from `Amount: -2000` to `Amount: 2000` (equity funds the liability opening balance, not assets). Update the expected values:

```go
t.Run("multi-currency keeps totals separate", func(t *testing.T) {
    txRepo := newMockTransactionRepo()
    addTxSplits(txRepo.splitsWithAccts, 1,
        model.SplitDetail{AccountName: "Assets:USD_Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
        model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
    )
    addTxSplits(txRepo.splitsWithAccts, 2,
        model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 50000, Currency: "TWD"},
        model.SplitDetail{AccountName: "Equity:Opening_TWD", AccountType: model.AccountTypeEquity, Amount: -50000, Currency: "TWD"},
    )
    addTxSplits(txRepo.splitsWithAccts, 3,
        model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -2000, Currency: "USD"},
        model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: 2000, Currency: "USD"},
    )
    svc := newTestTransactionService(newMockAccountRepo(), txRepo)

    nw, err := svc.GetNetWorthAt(context.Background(), 0)
    require.NoError(t, err)
    // USD: assets 10000, liabilities abs(-2000)=2000 → net 8000
    assert.Equal(t, int64(8000), nw["USD"])
    // TWD: assets 50000, liabilities 0 → net 50000
    assert.Equal(t, int64(50000), nw["TWD"])
    assert.Len(t, nw, 2)
})
```

- [ ] **Step 2: Add a test for liability-only currency (no matching assets)**

Add this test case inside `TestGetNetWorthAt`:

```go
t.Run("liability-only currency yields negative net worth", func(t *testing.T) {
    txRepo := newMockTransactionRepo()
    addTxSplits(txRepo.splitsWithAccts, 1,
        model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -3000, Currency: "USD"},
        model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: 3000, Currency: "USD"},
    )
    svc := newTestTransactionService(newMockAccountRepo(), txRepo)

    nw, err := svc.GetNetWorthAt(context.Background(), 0)
    require.NoError(t, err)
    assert.Equal(t, int64(-3000), nw["USD"])
})
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetNetWorthAt -v`
Expected: FAIL — the current code adds raw negative amounts and then subtracts, producing wrong results.

- [ ] **Step 4: Fix `GetNetWorthAt` in `report_service.go`**

Change line 29 to normalize liability amounts:

```go
case model.AccountTypeLiability:
    liabilities[split.Currency] += utils.AbsInt64(split.Amount)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/service/ -run TestGetNetWorthAt -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "fix: normalize liability sign in GetNetWorthAt

Use AbsInt64 on liability split amounts so net worth is computed as
assets - |liabilities| instead of assets - (negative value)."
```

---

### Task 2: Fix `GenerateBalanceSheet` — update tests to use realistic negative balances, then fix

**Files:**
- Modify: `internal/service/report_service.go:214-252`
- Modify: `internal/service/report_service_test.go:428-537`

- [ ] **Step 1: Update existing `GenerateBalanceSheet` tests to use negative liability balances**

In `report_service_test.go`, update the "basic asset and liability calculation" test (line 429). Change `accRepo.balances[2] = 3000` to `accRepo.balances[2] = -3000`. Update expected values — `TotalLiabilities` should now be the positive display value 3000, and net worth should still be 7000:

```go
t.Run("basic asset and liability calculation", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Liabilities:Card", Type: model.AccountTypeLiability})
    accRepo.balances[1] = 10000
    accRepo.balances[2] = -3000
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
    require.NoError(t, err)
    assert.Equal(t, int64(10000), result.TotalAssets["USD"])
    assert.Equal(t, int64(3000), result.TotalLiabilities["USD"])
    assert.Equal(t, int64(7000), result.NetWorth["USD"]) // 10000 - 3000
    require.Len(t, result.Assets, 1)
    require.Len(t, result.Liabilities, 1)
    assert.Equal(t, int64(3000), result.Liabilities[0].Amount)
})
```

Also update the "multi-currency" test (line 520). Change `accRepo.balances[3] = 3000` to `accRepo.balances[3] = -3000`. Expected values stay the same since TotalLiabilities and NetWorth should still be 3000 and 7000 respectively after the fix:

```go
t.Run("multi-currency totals are grouped by currency", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:USD_Bank", Type: model.AccountTypeAsset, Currency: "USD"})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:TWD_Bank", Type: model.AccountTypeAsset, Currency: "TWD"})
    accRepo.addAccount(&model.Account{ID: 3, Name: "Liabilities:Card", Type: model.AccountTypeLiability, Currency: "USD"})
    accRepo.balances[1] = 10000
    accRepo.balances[2] = 50000
    accRepo.balances[3] = -3000
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
    require.NoError(t, err)
    assert.Equal(t, int64(10000), result.TotalAssets["USD"])
    assert.Equal(t, int64(50000), result.TotalAssets["TWD"])
    assert.Equal(t, int64(3000), result.TotalLiabilities["USD"])
    assert.Equal(t, int64(7000), result.NetWorth["USD"])  // 10000 - 3000
    assert.Equal(t, int64(50000), result.NetWorth["TWD"]) // 50000 - 0
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/service/ -run TestGenerateBalanceSheet -v`
Expected: FAIL — negative balances flow through as-is, producing wrong totals.

- [ ] **Step 3: Fix `GenerateBalanceSheet` in `report_service.go`**

In the liability case of the account-type switch (around line 237), normalize the balance:

```go
case model.AccountTypeLiability:
    displayBalance := utils.AbsInt64(balance)
    row := model.ReportRow{
        AccountName: acc.Name,
        Amount:      displayBalance,
        Currency:    currency,
    }
    result.Liabilities = append(result.Liabilities, row)
    result.TotalLiabilities[currency] += displayBalance
```

Note: the `row` variable and the asset/equity cases are already defined above. Only the liability case needs the `displayBalance` normalization. The asset and equity cases continue using `balance` directly.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/service/ -run TestGenerateBalanceSheet -v`
Expected: PASS

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "fix: normalize liability sign in GenerateBalanceSheet

Use AbsInt64 on liability balances so TotalLiabilities and liability
rows display as positive amounts, and NetWorth = assets - liabilities
is computed correctly."
```
