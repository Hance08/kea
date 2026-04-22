# Multi-Split Transaction Display Fix

**Date:** 2026-04-22
**Branch:** fix/transaction-list-display-multi-split

## Problem

When a transaction has 2+ Asset/Liability splits and a minor Expense split, `DetermineType` incorrectly classifies it as Expense because the `hasExpense && assetOrLiabCnt >= 1` condition fires before the `assetOrLiabCnt >= 2` Transfer path is reached. This causes a mismatched display — the Account shown belongs to the Expense split while the Amount comes from the dominant Asset split.

**Example:**
- Debit: `Assets:Investments:00878` = 5160
- Debit: `Expenses:Fees:Stocks` = 7
- Credit: `Assets:Bank:DAWHO` = -5167

Current (wrong): Type=Expense, Account=`Expenses:Fees:Stocks`, Amount=5160
Expected: Type=Transfer, Account=`Assets:Investments:00878`, Amount=5160

## Classification Rule

When a transaction has both Expense splits and 2+ Asset/Liability splits, use dominant amount to decide:

| Condition | Type |
|---|---|
| total positive A/L amount > total E amount | Transfer |
| total E amount ≥ total positive A/L amount | Expense (ties go to Expense) |

When there is only 1 Asset/Liability split alongside an Expense split, it remains Expense (unchanged behaviour).

## Design

### Change 1 — `DetermineType` in `internal/service/transaction_classifier.go`

**Add accumulator in the loop:**
```go
case "A", "L":
    assetOrLiabCnt++
    if split.Amount > 0 {
        isAssetIncrease = true
        totalPositiveAssetLiabAmount += split.Amount  // new
    }
```

**Replace the single hasExpense condition with two paths:**
```go
// Before:
if hasExpense && assetOrLiabCnt >= 1 {
    return model.TxTypeExpense, nil
}

// After:
if hasExpense && assetOrLiabCnt >= 2 {
    if totalPositiveAssetLiabAmount > totalExpenseAmount {
        return model.TxTypeTransfer, nil
    }
    return model.TxTypeExpense, nil
}
if hasExpense && assetOrLiabCnt == 1 {
    return model.TxTypeExpense, nil
}
```

### Change 2 — `GetDisplayAccount` and `GetDisplayAmount`

No changes needed. Once type is correctly Transfer, the existing Transfer branch in `GetDisplayAccount` returns the first positive A/L split (correct), and `GetDisplayAmount` returns the max positive amount which belongs to that same split (correct).

### Change 3 — Tests in `internal/service/transaction_classifier_test.go`

Three new cases added to `TestDetermineType`:

1. `"transfer with fee: A/L dominates E → Transfer"` — stock purchase with small fee
2. `"expense split bill: E ties A/L → Expense"` — 60/60 split, ties go to Expense
3. `"expense dominant: E > A/L → Expense"` — E is larger than A/L positive flow

## End-to-End Verification

| Transaction | Type | Account | Amount | Offset |
|---|---|---|---|---|
| Stock purchase (A/L=5160, E=7) | Transfer | `Assets:Investments:00878` | 5160 | `(multiple)` |
| Split bill (A/L=60, E=60, tie) | Expense | `Expenses:Food:Drink` | 60 | `(multiple)` |
| Dominant expense (A/L=40, E=100) | Expense | `Expenses:Food` | 100 | `(multiple)` |

## Scope

- One variable added, one condition split into two paths in `DetermineType`
- No changes to `GetDisplayAccount`, `GetDisplayAmount`, or `GetDisplayOffsetAccount`
- No schema or model changes
