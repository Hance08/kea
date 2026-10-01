# Investment Transaction Type Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new "Investment" transaction type that classifies buy/sell stock transactions by the `Assets:Investments:*` account-name prefix and displays the cash-side bank movement as the list amount.

**Architecture:** Add `TxTypeInvestment` constant plus an `IsInvestmentAccount(name)` helper. Extend the existing `DetermineType` classifier with an Investment branch that wins over Income/Expense/Transfer when an `Assets:Investments:*` split is present alongside another A/L split. Extend the display helpers (`GetDisplayAmount`, `GetDisplayAccount`, `GetDisplayOffsetAccount`), the rule registry, and the picker (`GetAllowedAccounts`). Add a `0009` backfill migration. Wire the new type through `cmd/add`, `cmd/transaction/edit`, the API, and the SPA (where the TypeScript classifier mirrors the Go one).

**Tech Stack:** Go 1.x, SQLite (`mattn/go-sqlite3`), golang-migrate, charmbracelet/huh, React + TypeScript (SPA).

**Spec:** [docs/superpowers/specs/2026-06-16-investment-transaction-type-design.md](../specs/2026-06-16-investment-transaction-type-design.md)

---

## File Map

**Create:**
- `migrations/0009_backfill_investment_type.up.sql`
- `migrations/0009_backfill_investment_type.down.sql`

**Modify (Go):**
- `internal/model/types.go` — new constant, `IsValid`, `ParseTransactionType`, `ParseTransactionTypeLabel`, `IsInvestmentAccount` helper
- `internal/model/types_test.go` — extend existing tables
- `internal/service/transaction_classifier.go` — `DetermineType`, `ValidateSplitsMatchType`, `GetDisplayAccount`, `GetDisplayAmount` (signature change), `GetDisplayOffsetAccount`, `GetAllowedAccounts`, `BuildTransactionListItems`
- `internal/service/transaction_classifier_test.go` — add Investment cases + update existing `GetDisplayAmount` callers
- `internal/service/transaction_service.go` — `GetTransactionRule` Investment entry
- `cmd/add_actions.go` — `modeUIConfigs` Investment entry; `selectAccount` name-prefix filter for Investment
- `cmd/add.go` — `--type` flag help text
- `ui/prompts/transaction.go` — `PromptTransactionType` adds "Record Investment"

**Modify (SPA):**
- `spa/src/lib/types.ts:37` — `TransactionType` union
- `spa/src/lib/determineType.ts` — Investment branch
- `spa/src/lib/transactionDisplay.ts` — `displayAccount`, `displayOffsetAccount`, `displayAmount` Investment branches
- `spa/src/components/transactions/TypeBadge.tsx:4` — `TYPE_CLASSES` Investment entry
- `spa/src/components/transactions/FilterBar.tsx` — filter dropdown
- `spa/src/components/transactions/SimpleFields.tsx` — form options
- `spa/src/components/transactions/TransactionForm.tsx` — form options

---

## Task 1: Add `TxTypeInvestment` constant and `IsInvestmentAccount` helper

**Files:**
- Modify: `internal/model/types.go`
- Test: `internal/model/types_test.go`

- [ ] **Step 1.1: Write the failing tests**

Add to `internal/model/types_test.go`:

```go
func TestIsInvestmentAccount(t *testing.T) {
    tests := []struct {
        name string
        in   string
        want bool
    }{
        {"matches investment account", "Assets:Investments:00878", true},
        {"matches with multi-segment ticker", "Assets:Investments:NYSE:AAPL", true},
        {"rejects similar prefix without final colon", "Assets:Investments", false},
        {"rejects sibling singular name", "Assets:Investment:foo", false},
        {"rejects unrelated asset", "Assets:Bank:DAWHO", false},
        {"rejects empty", "", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := IsInvestmentAccount(tt.in); got != tt.want {
                t.Errorf("IsInvestmentAccount(%q) = %v, want %v", tt.in, got, tt.want)
            }
        })
    }
}
```

Extend the existing tables in `TestParseTransactionType`, `TestParseTransactionTypeLabel`, `TestTransactionType_IsValid`, `TestTransactionType_MarshalJSON`, `TestTransactionType_UnmarshalJSON` with `TxTypeInvestment`. Concretely, add these cases to each table (look at the existing tests for the table shape):

- `TestParseTransactionType`: `{input: "investment", want: TxTypeInvestment}`, `{input: "INVESTMENT", want: TxTypeInvestment}`.
- `TestParseTransactionTypeLabel`: `{input: "Record Investment", want: TxTypeInvestment}`.
- `TestTransactionType_IsValid`: add `TxTypeInvestment` to the `valid` slice.
- `TestTransactionType_MarshalJSON` / `TestTransactionType_UnmarshalJSON`: add `TxTypeInvestment` to the `all` slice.

- [ ] **Step 1.2: Run tests to verify they fail**

Run: `go test ./internal/model/ -run "TestIsInvestmentAccount|TestParseTransactionType|TestParseTransactionTypeLabel|TestTransactionType_IsValid|TestTransactionType_MarshalJSON|TestTransactionType_UnmarshalJSON" -v`
Expected: build error — `undefined: IsInvestmentAccount` and `undefined: TxTypeInvestment`.

- [ ] **Step 1.3: Add the constant and helper to `internal/model/types.go`**

In the `const (...)` block at line 144 add `TxTypeInvestment`:

```go
const (
    TxTypeExpense    TransactionType = "Expense"
    TxTypeIncome     TransactionType = "Income"
    TxTypeTransfer   TransactionType = "Transfer"
    TxTypeOpening    TransactionType = "Opening"
    TxTypeDeposit    TransactionType = "Deposit"
    TxTypeWithdrawal TransactionType = "Withdrawal"
    TxTypeOther      TransactionType = "Other"
    TxTypeInvestment TransactionType = "Investment"
)
```

Update `IsValid`:

```go
func (t TransactionType) IsValid() bool {
    switch t {
    case TxTypeExpense, TxTypeIncome, TxTypeTransfer,
        TxTypeOpening, TxTypeDeposit, TxTypeWithdrawal, TxTypeOther,
        TxTypeInvestment:
        return true
    }
    return false
}
```

Add to `ParseTransactionType` (inside the switch, before the `default`):

```go
    case "investment":
        return TxTypeInvestment, nil
```

Add to `ParseTransactionTypeLabel` (before the final `return TxTypeTransfer`):

```go
    if strings.Contains(lower, "investment") {
        return TxTypeInvestment
    }
```

Update the error message in `ParseTransactionType`'s `default`:

```go
        return "", fmt.Errorf("invalid transaction type %q: must be expense, income, transfer, opening, deposit, withdrawal, other, or investment", s)
```

Add the helper alongside `IsOpeningBalancesAccount` (after line 196):

```go
const InvestmentAccountPrefix = "Assets:Investments:"

func IsInvestmentAccount(name string) bool {
    return strings.HasPrefix(name, InvestmentAccountPrefix)
}
```

- [ ] **Step 1.4: Run tests to verify they pass**

Run: `go test ./internal/model/ -v`
Expected: PASS.

- [ ] **Step 1.5: Commit**

```bash
git add internal/model/types.go internal/model/types_test.go
git commit -m "feat(model): add TxTypeInvestment and IsInvestmentAccount helper"
```

---

## Task 2: Extend `DetermineType` with Investment branch

**Files:**
- Modify: `internal/service/transaction_classifier.go:15-104`
- Test: `internal/service/transaction_classifier_test.go`

- [ ] **Step 2.1: Add failing tests**

Add these cases to the `TestDetermineType` table in `internal/service/transaction_classifier_test.go` (the table starts around line 72). The `classifierAccRepo` fixture already includes `Assets:Investments:00878`, `Assets:Bank:DAWHO`, `Expenses:Fees:Stocks`:

```go
{
    name: "investment: sell stock with realized gain → Investment",
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, -5287),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, 8118),
        split("Revenue:Salary", model.AccountTypeRevenue, -2831),
    },
    want: model.TxTypeInvestment,
},
{
    name: "investment: buy stock with fee → Investment",
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, 20490),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, -20519),
        split("Expenses:Fees:Stocks", model.AccountTypeExpense, 29),
    },
    want: model.TxTypeInvestment,
},
{
    name: "investment: clean transfer between brokerages → Investment",
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, 1000),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, -1000),
    },
    want: model.TxTypeInvestment,
},
{
    name: "investment account alone (no cash side) falls through to Transfer",
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, 100),
        split("Assets:Investments:00878", model.AccountTypeAsset, -100),
    },
    want: model.TxTypeTransfer,
},
```

- [ ] **Step 2.2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestDetermineType -v`
Expected: the four new cases fail; the first three return `Income` or `Transfer`; the fourth passes (already covered by existing logic).

- [ ] **Step 2.3: Add the Investment branch to `DetermineType`**

In `internal/service/transaction_classifier.go`, modify `DetermineType` (line 15). Add two new tracking variables and a branch placed **after** the `isOpening` check but **before** the existing `hasExpense && hasRevenue` check.

Inside the loop where each split's `accType` is resolved, after the existing `case` handling, track investment splits. Replace the existing `case model.AccountTypeAsset:` and `case model.AccountTypeLiability:` branches with:

```go
        case model.AccountTypeAsset:
            assetOrLiabCnt++
            if model.IsInvestmentAccount(split.AccountName) {
                hasInvestmentAccount = true
            } else {
                nonInvestmentAssetOrLiabCnt++
            }
            if split.Amount > 0 {
                isAssetIncrease = true
                totalPositiveAssetLiabAmount += split.Amount
            }
        case model.AccountTypeLiability:
            assetOrLiabCnt++
            nonInvestmentAssetOrLiabCnt++
            if split.Amount > 0 {
                totalPositiveAssetLiabAmount += split.Amount
            }
```

Declare the new variables alongside the existing ones near the top of the function:

```go
    var (
        hasExpense                  bool
        hasRevenue                  bool
        hasEquity                   bool
        assetOrLiabCnt              int
        isOpening                   bool
        isAssetIncrease             bool
        hasInvestmentAccount        bool
        nonInvestmentAssetOrLiabCnt int
    )
```

Insert the new branch immediately after the `if isOpening { return ... }` block:

```go
    if hasInvestmentAccount && nonInvestmentAssetOrLiabCnt >= 1 {
        return model.TxTypeInvestment, nil
    }
```

- [ ] **Step 2.4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestDetermineType -v`
Expected: all cases PASS, including pre-existing ones (no regression).

- [ ] **Step 2.5: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_classifier_test.go
git commit -m "feat(classifier): detect Investment transactions by account-name prefix"
```

---

## Task 3: Extend `ValidateSplitsMatchType` with Investment case

**Files:**
- Modify: `internal/service/transaction_classifier.go:311-374`
- Test: `internal/service/transaction_classifier_test.go`

- [ ] **Step 3.1: Add failing tests**

Find `TestValidateSplitsMatchType` in `internal/service/transaction_classifier_test.go` (if absent, search for `ValidateSplitsMatchType`). Add these cases (the test uses `classifierAccRepo()`):

```go
{
    name:    "investment: valid sell with realized gain",
    txType:  model.TxTypeInvestment,
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, -5287),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, 8118),
        split("Revenue:Salary", model.AccountTypeRevenue, -2831),
    },
    wantErr: false,
},
{
    name:   "investment: valid buy with fee",
    txType: model.TxTypeInvestment,
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, 20490),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, -20519),
        split("Expenses:Fees:Stocks", model.AccountTypeExpense, 29),
    },
    wantErr: false,
},
{
    name:   "investment: missing Investments split → error",
    txType: model.TxTypeInvestment,
    splits: []model.SplitDetail{
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, 100),
        split("Assets:Cash", model.AccountTypeAsset, -100),
    },
    wantErr: true,
},
{
    name:   "investment: missing cash side → error",
    txType: model.TxTypeInvestment,
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, 100),
        split("Assets:Investments:00878", model.AccountTypeAsset, -100),
    },
    wantErr: true,
},
```

- [ ] **Step 3.2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestValidateSplitsMatchType -v`
Expected: the four new cases fail (current `default` returns `unknown transaction type`).

- [ ] **Step 3.3: Add the Investment case**

In `internal/service/transaction_classifier.go`, in `ValidateSplitsMatchType` (line 311), add a new case before `default`:

```go
    case model.TxTypeInvestment:
        var hasInvestment, hasOtherAssetOrLiab bool
        for _, s := range splits {
            accType, err := ts.resolveAccountType(ctx, s)
            if err != nil {
                return err
            }
            if accType != model.AccountTypeAsset && accType != model.AccountTypeLiability {
                continue
            }
            name := s.AccountName
            if name == "" {
                acc, err := ts.accRepo.GetAccountByID(ctx, s.AccountID)
                if err != nil {
                    return err
                }
                name = acc.Name
            }
            if model.IsInvestmentAccount(name) {
                hasInvestment = true
            } else {
                hasOtherAssetOrLiab = true
            }
        }
        if !hasInvestment {
            return validationErrorf("type",
                "investment transaction requires at least one Assets:Investments:* account")
        }
        if !hasOtherAssetOrLiab {
            return validationErrorf("type",
                "investment transaction requires at least one other Asset or Liability account (the cash side)")
        }
```

- [ ] **Step 3.4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestValidateSplitsMatchType -v`
Expected: PASS.

- [ ] **Step 3.5: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_classifier_test.go
git commit -m "feat(classifier): validate Investment transaction split shape"
```

---

## Task 4: Type-aware `GetDisplayAmount` (signature change)

**Files:**
- Modify: `internal/service/transaction_classifier.go:167-186`
- Modify: `internal/service/transaction_classifier.go:254` (BuildTransactionListItems caller)
- Test: `internal/service/transaction_classifier_test.go:244-281` (existing tests need signature update)

- [ ] **Step 4.1: Update existing tests for the new signature and add Investment cases**

In `internal/service/transaction_classifier_test.go`, update every call site of `svc.GetDisplayAmount(...)` to pass a transaction type. The existing tests at lines 247-280 use generic splits — pass `""` (empty string) which the new function will treat as the legacy fallback path. Then add the Investment subtests:

```go
t.Run("Investment: returns cash-side magnitude for buy", func(t *testing.T) {
    svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
    splits := []model.SplitDetail{
        {AccountName: "Assets:Investments:00878", AccountType: model.AccountTypeAsset, Amount: 20490, Currency: "TWD"},
        {AccountName: "Assets:Bank:DAWHO", AccountType: model.AccountTypeAsset, Amount: -20519, Currency: "TWD"},
        {AccountName: "Expenses:Fees:Stocks", AccountType: model.AccountTypeExpense, Amount: 29, Currency: "TWD"},
    }
    amount, currency := svc.GetDisplayAmount(splits, string(model.TxTypeInvestment))
    assert.Equal(t, int64(20519), amount)
    assert.Equal(t, "TWD", currency)
})

t.Run("Investment: returns cash-side magnitude for sell", func(t *testing.T) {
    svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
    splits := []model.SplitDetail{
        {AccountName: "Assets:Investments:00878", AccountType: model.AccountTypeAsset, Amount: -5287, Currency: "TWD"},
        {AccountName: "Assets:Bank:DAWHO", AccountType: model.AccountTypeAsset, Amount: 8118, Currency: "TWD"},
        {AccountName: "Revenue:Salary", AccountType: model.AccountTypeRevenue, Amount: -2831, Currency: "TWD"},
    },
    amount, currency := svc.GetDisplayAmount(splits, string(model.TxTypeInvestment))
    assert.Equal(t, int64(8118), amount)
    assert.Equal(t, "TWD", currency)
})
```

Also fix the legacy tests at lines 247-281 — every call gets a second argument `""`. Example update for the first subtest:

```go
t.Run("empty splits returns 0 and empty currency", func(t *testing.T) {
    amount, currency := svc.GetDisplayAmount(nil, "")
    assert.Equal(t, int64(0), amount)
    assert.Equal(t, "", currency)
})
```

Apply the same `, ""` change to all four existing subtests.

- [ ] **Step 4.2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetDisplayAmount -v`
Expected: build error — call signature mismatch.

- [ ] **Step 4.3: Update `GetDisplayAmount` signature and add Investment branch**

In `internal/service/transaction_classifier.go`, replace lines 167-186:

```go
func (ts *TransactionService) GetDisplayAmount(splits []model.SplitDetail, txType string) (int64, string) {
    if len(splits) == 0 {
        return 0, ""
    }

    if txType == string(model.TxTypeInvestment) {
        var bestAmount int64
        currency := splits[0].Currency
        for _, s := range splits {
            if s.AccountType != model.AccountTypeAsset && s.AccountType != model.AccountTypeLiability {
                continue
            }
            if model.IsInvestmentAccount(s.AccountName) {
                continue
            }
            abs := s.Amount
            if abs < 0 {
                abs = -abs
            }
            if abs > bestAmount {
                bestAmount = abs
                currency = s.Currency
            }
        }
        return bestAmount, currency
    }

    var maxAmount int64
    currency := splits[0].Currency
    for _, split := range splits {
        if split.Amount > maxAmount {
            maxAmount = split.Amount
            currency = split.Currency
        }
    }
    return maxAmount, currency
}
```

In `BuildTransactionListItems` (line 234), change the call at line 254:

```go
        amountCents, currency := ts.GetDisplayAmount(detail.Splits, txType)
```

- [ ] **Step 4.4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v`
Expected: PASS (all classifier tests).

Then run: `go build ./...`
Expected: no compile errors. (If any other callers of `GetDisplayAmount` exist outside `internal/service`, the build will surface them — fix them by passing the relevant type, e.g. the transaction's `tx.Type`.)

- [ ] **Step 4.5: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_classifier_test.go
git commit -m "feat(classifier): type-aware GetDisplayAmount with Investment cash-side rule"
```

---

## Task 5: Investment branches in display Account/Offset and picker/rule

**Files:**
- Modify: `internal/service/transaction_classifier.go` (`GetDisplayAccount`, `GetDisplayOffsetAccount`, `GetAllowedAccounts`)
- Modify: `internal/service/transaction_service.go:27-50` (`GetTransactionRule`)
- Test: `internal/service/transaction_classifier_test.go`

- [ ] **Step 5.1: Add failing tests**

In `internal/service/transaction_classifier_test.go`:

```go
func TestGetDisplayAccount_Investment(t *testing.T) {
    svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
    splits := []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, -5287),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, 8118),
        split("Revenue:Salary", model.AccountTypeRevenue, -2831),
    }
    got, err := svc.GetDisplayAccount(context.Background(), splits, string(model.TxTypeInvestment))
    require.NoError(t, err)
    assert.Equal(t, "Assets:Investments:00878", got)
}

func TestGetDisplayOffsetAccount_Investment(t *testing.T) {
    svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
    splits := []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, -5287),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, 8118),
        split("Revenue:Salary", model.AccountTypeRevenue, -2831),
    }
    got, err := svc.GetDisplayOffsetAccount(
        context.Background(), splits, string(model.TxTypeInvestment), "Assets:Investments:00878",
    )
    require.NoError(t, err)
    assert.Equal(t, "Assets:Bank:DAWHO", got)
}

func TestGetAllowedAccounts_Investment(t *testing.T) {
    svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
    accs := []*model.Account{
        {Name: "Assets:Investments:0050", Type: model.AccountTypeAsset},
        {Name: "Liabilities:Card", Type: model.AccountTypeLiability},
        {Name: "Revenue:Realized", Type: model.AccountTypeRevenue},
        {Name: "Expenses:Fees", Type: model.AccountTypeExpense},
        {Name: "Equity:Retained", Type: model.AccountTypeEquity},
    }
    got := svc.GetAllowedAccounts(model.TxTypeInvestment, model.AccountTypeAsset, accs)
    var names []string
    for _, a := range got {
        names = append(names, a.Name)
    }
    assert.ElementsMatch(t,
        []string{"Assets:Investments:0050", "Liabilities:Card", "Revenue:Realized", "Expenses:Fees"},
        names)
}

func TestGetTransactionRule_Investment(t *testing.T) {
    svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
    rule, err := svc.GetTransactionRule(model.TxTypeInvestment)
    require.NoError(t, err)
    assert.Equal(t, model.TxTypeInvestment, rule.TxType)
    assert.ElementsMatch(t, []string{"A", "L"}, rule.SourceTypes)
    assert.ElementsMatch(t, []string{"A", "L"}, rule.DestTypes)
}
```

- [ ] **Step 5.2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestGetDisplayAccount_Investment|TestGetDisplayOffsetAccount_Investment|TestGetAllowedAccounts_Investment|TestGetTransactionRule_Investment" -v`
Expected: FAIL.

- [ ] **Step 5.3: Add Investment branches**

In `internal/service/transaction_classifier.go`, in `GetDisplayAccount` (line 106), add this case before `case "Other":`:

```go
    case "Investment":
        for _, split := range splits {
            if model.IsInvestmentAccount(split.AccountName) {
                return split.AccountName, nil
            }
        }
```

In `GetDisplayOffsetAccount` (line 188), add this case before `default:`:

```go
    case string(model.TxTypeInvestment):
        for _, split := range splits {
            if split.AccountType != model.AccountTypeAsset && split.AccountType != model.AccountTypeLiability {
                continue
            }
            if model.IsInvestmentAccount(split.AccountName) {
                continue
            }
            seen[split.AccountName] = struct{}{}
        }
```

In `GetAllowedAccounts` (line 271), add this case before `default:`:

```go
    case model.TxTypeInvestment:
        return ts.filterAccountsByTypes(allAccounts, []model.AccountType{
            model.AccountTypeAsset,
            model.AccountTypeLiability,
            model.AccountTypeRevenue,
            model.AccountTypeExpense,
        })
```

In `internal/service/transaction_service.go`, in `GetTransactionRule` (line 27), add this case before `default:`:

```go
    case model.TxTypeInvestment:
        return model.TransactionRule{
            TxType:      model.TxTypeInvestment,
            SourceTypes: []string{"A", "L"},
            DestTypes:   []string{"A", "L"},
        }, nil
```

- [ ] **Step 5.4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v`
Expected: PASS.

- [ ] **Step 5.5: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_service.go internal/service/transaction_classifier_test.go
git commit -m "feat(service): Investment branches in display, picker, and rule registry"
```

---

## Task 6: Migration `0009` — backfill existing Investment-shaped rows

**Files:**
- Create: `migrations/0009_backfill_investment_type.up.sql`
- Create: `migrations/0009_backfill_investment_type.down.sql`
- Test: `internal/store/sqlite_transaction_test.go` (or a new file `migration_0009_test.go`)

- [ ] **Step 6.1: Write the failing migration test**

Open `internal/store/sqlite_transaction_test.go` and look at an existing migration-aware test (the file initializes a SQLite store with all migrations applied). Create `internal/store/migration_0009_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
    "context"
    "testing"

    "github.com/hance08/kea/internal/model"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestMigration0009_BackfillInvestmentType inserts pre-existing rows of each
// shape and verifies the migration reclassifies only Investment-shaped ones.
func TestMigration0009_BackfillInvestmentType(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()

    // Insert accounts.
    invID, _ := s.CreateAccount(ctx, "Assets:Investments:00878", model.AccountTypeAsset, "TWD", "", nil)
    bankID, _ := s.CreateAccount(ctx, "Assets:Bank:DAWHO", model.AccountTypeAsset, "TWD", "", nil)
    revID, _ := s.CreateAccount(ctx, "Revenue:Stuff", model.AccountTypeRevenue, "TWD", "", nil)
    feeID, _ := s.CreateAccount(ctx, "Expenses:Fees:Stocks", model.AccountTypeExpense, "TWD", "", nil)
    foodID, _ := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "TWD", "", nil)

    // After all migrations have run, insert rows simulating the
    // pre-migration classifier output for each shape.
    // Sell stock (currently classified as Income).
    sellID := insertTx(t, s, "sell", model.TxTypeIncome, []splitFixture{
        {invID, -5287}, {bankID, 8118}, {revID, -2831},
    })
    // Buy stock (currently classified as Transfer).
    buyID := insertTx(t, s, "buy", model.TxTypeTransfer, []splitFixture{
        {invID, 20490}, {bankID, -20519}, {feeID, 29},
    })
    // Plain expense — must NOT be reclassified.
    expID := insertTx(t, s, "lunch", model.TxTypeExpense, []splitFixture{
        {foodID, 500}, {bankID, -500},
    })
    // Plain income (no Investments split) — must NOT be reclassified.
    incomeID := insertTx(t, s, "tip", model.TxTypeIncome, []splitFixture{
        {revID, -100}, {bankID, 100},
    })

    // Reset the type column on the Investment-shaped rows to simulate
    // their state BEFORE this migration ran.
    _, err := s.db.ExecContext(ctx,
        `UPDATE transactions SET type = 'Income' WHERE id = ?`, sellID)
    require.NoError(t, err)
    _, err = s.db.ExecContext(ctx,
        `UPDATE transactions SET type = 'Transfer' WHERE id = ?`, buyID)
    require.NoError(t, err)

    // Re-run the 0009 up migration. (The test store applies all migrations on
    // initialization, so 0009 has already run once and was a no-op against an
    // empty DB. Running it again is idempotent against the data we just inserted.)
    _, err = s.db.ExecContext(ctx, mustReadMigration(t, "0009_backfill_investment_type.up.sql"))
    require.NoError(t, err)

    // Assert.
    assert.Equal(t, model.TxTypeInvestment, fetchType(t, s, sellID), "sell should be Investment")
    assert.Equal(t, model.TxTypeInvestment, fetchType(t, s, buyID), "buy should be Investment")
    assert.Equal(t, model.TxTypeExpense, fetchType(t, s, expID), "expense untouched")
    assert.Equal(t, model.TxTypeIncome, fetchType(t, s, incomeID), "income untouched")
}
```

If `insertTx`, `splitFixture`, `mustReadMigration`, `fetchType`, or `newTestStore` don't already exist, define them inline at the top of `migration_0009_test.go`:

```go
type splitFixture struct {
    accountID int64
    amount    int64
}

func insertTx(t *testing.T, s *Store, desc string, txType model.TransactionType, splits []splitFixture) int64 {
    t.Helper()
    ctx := context.Background()
    var splitInputs []model.SplitDetail
    for _, sp := range splits {
        splitInputs = append(splitInputs, model.SplitDetail{
            AccountID: sp.accountID,
            Amount:    sp.amount,
            Currency:  "TWD",
        })
    }
    tx := &model.Transaction{
        Timestamp:   1700000000,
        Description: desc,
        Status:      model.StatusCleared,
        Type:        txType,
    }
    id, err := s.CreateTransactionWithSplits(ctx, tx, splitInputs)
    require.NoError(t, err)
    return id
}

func fetchType(t *testing.T, s *Store, txID int64) model.TransactionType {
    t.Helper()
    var ty string
    err := s.db.QueryRowContext(context.Background(),
        `SELECT type FROM transactions WHERE id = ?`, txID).Scan(&ty)
    require.NoError(t, err)
    return model.TransactionType(ty)
}

func mustReadMigration(t *testing.T, name string) string {
    t.Helper()
    data, err := migrations.FS.ReadFile(name)
    require.NoError(t, err)
    return string(data)
}
```

Check `internal/store/sqlite_store_test.go` for the existing `newTestStore` helper; if its signature differs, adapt the call. If `migrations.FS` is in a different package path, inspect `migrations/embed.go` and import accordingly.

- [ ] **Step 6.2: Run the test to verify it fails**

Run: `go test ./internal/store/ -run TestMigration0009_BackfillInvestmentType -v`
Expected: FAIL — the migration file doesn't exist yet, or `mustReadMigration` errors.

- [ ] **Step 6.3: Create the migration files**

Create `migrations/0009_backfill_investment_type.up.sql`:

```sql
UPDATE transactions SET type = 'Investment'
WHERE type IN ('Income', 'Transfer', 'Other')
  AND id IN (
    SELECT s_inv.transaction_id
    FROM splits s_inv
    JOIN accounts a_inv ON s_inv.account_id = a_inv.id
    WHERE a_inv.name LIKE 'Assets:Investments:%'
    INTERSECT
    SELECT s_cash.transaction_id
    FROM splits s_cash
    JOIN accounts a_cash ON s_cash.account_id = a_cash.id
    WHERE a_cash.type IN ('A', 'L')
      AND a_cash.name NOT LIKE 'Assets:Investments:%'
  );
```

Create `migrations/0009_backfill_investment_type.down.sql` — conservative reverse classifier:

```sql
-- Reverse: reclassify Investment rows back using the legacy classifier output.
-- Investment rows containing a Revenue split → Income; otherwise → Transfer.
-- (Lossy by design; perfect round-trip is not required.)
UPDATE transactions SET type = 'Income'
WHERE type = 'Investment'
  AND id IN (
    SELECT s.transaction_id
    FROM splits s
    JOIN accounts a ON s.account_id = a.id
    WHERE a.type = 'R'
  );

UPDATE transactions SET type = 'Transfer'
WHERE type = 'Investment';
```

- [ ] **Step 6.4: Run the test to verify it passes**

Run: `go test ./internal/store/ -run TestMigration0009_BackfillInvestmentType -v`
Expected: PASS.

Also run the full store test suite to confirm no regression on migration ordering:

Run: `go test ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 6.5: Commit**

```bash
git add migrations/0009_backfill_investment_type.up.sql \
        migrations/0009_backfill_investment_type.down.sql \
        internal/store/migration_0009_test.go
git commit -m "feat(migrations): backfill Investment type for existing rows (0009)"
```

---

## Task 7: `kea add` — prompt label, mode UI config, flag help

**Files:**
- Modify: `ui/prompts/transaction.go:15-28`
- Modify: `cmd/add_actions.go:198-202`
- Modify: `cmd/add.go` (flag help text for `--type`)

- [ ] **Step 7.1: Update `PromptTransactionType`**

In `ui/prompts/transaction.go`, replace the `options` slice:

```go
    options := []string{
        "Record Expense",
        "Record Income",
        "Transfer",
        "Record Investment",
    }
```

- [ ] **Step 7.2: Update `modeUIConfigs`**

In `cmd/add_actions.go` (line 198), add the Investment entry:

```go
var modeUIConfigs = map[model.TransactionType]struct{ Src, Dst string }{
    model.TxTypeExpense:    {"Payment Source:", "Expense Type:"},
    model.TxTypeIncome:     {"Revenue Type:", "Deposit To:"},
    model.TxTypeTransfer:   {"From Account:", "To Account:"},
    model.TxTypeInvestment: {"Cash Account:", "Investment Account:"},
}
```

- [ ] **Step 7.3: Update `--type` flag help**

In `cmd/add.go`, find the line registering `--type` and update its help text. Search for `cmd.Flags().StringVar(&flags.Type` — change the description to:

```
"Transaction type: expense, income, transfer, investment (in flag mode, investment creates two splits; use 'kea transaction edit' to add fee or gain splits)"
```

- [ ] **Step 7.4: Build and run smoke test**

Run: `go build ./...`
Expected: no errors.

Run: `go test ./cmd/... ./internal/service/... -v`
Expected: PASS.

- [ ] **Step 7.5: Commit**

```bash
git add ui/prompts/transaction.go cmd/add_actions.go cmd/add.go
git commit -m "feat(cmd/add): support Investment as a transaction type"
```

---

## Task 8: `kea add` interactive — filter accounts by `Assets:Investments:*` prefix

**Files:**
- Modify: `cmd/add_actions.go` (the interactive flow around `selectAccount` for Investment)
- Modify: `ui/prompts/transaction.go` (`filterAccounts` / `PromptAccountSelection`)

**Context:** For Investment in interactive mode, the source account is "any A/L except `Assets:Investments:*`" and the destination is "only `Assets:Investments:*`". The existing `filterAccounts` in `ui/prompts/transaction.go:66` filters by type and currency only. The minimal change: in `runInteractive`, when `mode == TxTypeInvestment`, post-filter the two `selectAccount` results by name prefix before passing them on. We do this in the runner, not the prompt helper, to keep the prompt helper generic.

- [ ] **Step 8.1: Wrap the `selectAccount` calls for Investment**

In `cmd/add_actions.go`, in `runInteractive` (around line 129), replace the two `selectAccount` calls with a small Investment-aware fork:

```go
    var fromAccount, toAccount string
    if mode == model.TxTypeInvestment {
        // Cash side: any A/L NOT under Assets:Investments:*
        cashAccounts := filterNonInvestmentAccounts(accounts)
        fromAccount, err = r.selectAccount(ctx, cashAccounts, rule.SourceTypes, uiConf.Src, true, "")
        if err != nil {
            return addTransactionInput{}, err
        }
        fromAcc, err := r.accSvc.GetAccountByName(ctx, fromAccount)
        if err != nil {
            return addTransactionInput{}, fmt.Errorf("failed to load account %q: %w", fromAccount, err)
        }
        // Investment side: ONLY Assets:Investments:*
        invAccounts := filterInvestmentAccounts(accounts)
        toAccount, err = r.selectAccount(ctx, invAccounts, rule.DestTypes, uiConf.Dst, true, fromAcc.Currency)
        if err != nil {
            return addTransactionInput{}, err
        }
    } else {
        fromAccount, err = r.selectAccount(ctx, accounts, rule.SourceTypes, uiConf.Src, true, "")
        if err != nil {
            return addTransactionInput{}, err
        }
        fromAcc, err := r.accSvc.GetAccountByName(ctx, fromAccount)
        if err != nil {
            return addTransactionInput{}, fmt.Errorf("failed to load account %q: %w", fromAccount, err)
        }
        toAccount, err = r.selectAccount(ctx, accounts, rule.DestTypes, uiConf.Dst, mode != model.TxTypeExpense, fromAcc.Currency)
        if err != nil {
            return addTransactionInput{}, err
        }
    }
```

Add the two helpers at the bottom of `cmd/add_actions.go`:

```go
func filterInvestmentAccounts(accounts []*model.Account) []*model.Account {
    var out []*model.Account
    for _, a := range accounts {
        if model.IsInvestmentAccount(a.Name) {
            out = append(out, a)
        }
    }
    return out
}

func filterNonInvestmentAccounts(accounts []*model.Account) []*model.Account {
    var out []*model.Account
    for _, a := range accounts {
        if !model.IsInvestmentAccount(a.Name) {
            out = append(out, a)
        }
    }
    return out
}
```

- [ ] **Step 8.2: Build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 8.3: Manual smoke test**

Run: `make build && ./kea_test add` (or `go run ./cmd/kea add`).
Expected: choosing "Record Investment" from the prompt offers a cash account list first (no Investments accounts visible), then offers only `Assets:Investments:*` accounts for the second pick.

If you do not have an existing ledger with `Assets:Investments:*` accounts, create one first via `kea account add Assets:Investments:0050 ...`.

- [ ] **Step 8.4: Commit**

```bash
git add cmd/add_actions.go
git commit -m "feat(cmd/add): split investment account picker into cash + investment sides"
```

---

## Task 9: SPA — `TransactionType` union + `determineType`

**Files:**
- Modify: `spa/src/lib/types.ts:37-44`
- Modify: `spa/src/lib/determineType.ts`
- Test: `spa/src/test/` (look for existing `determineType.test.ts`; create one if absent)

- [ ] **Step 9.1: Add `Investment` to the `TransactionType` union**

In `spa/src/lib/types.ts`, line 37:

```ts
export type TransactionType =
  | 'Expense'
  | 'Income'
  | 'Transfer'
  | 'Opening'
  | 'Deposit'
  | 'Withdrawal'
  | 'Other'
  | 'Investment';
```

- [ ] **Step 9.2: Write the failing test**

Check whether `spa/src/test/determineType.test.ts` exists. If yes, append. If no, create it:

```ts
import { describe, expect, it } from 'vitest';
import { determineType } from '@/lib/determineType';
import type { SplitDetail } from '@/lib/types';

const sp = (
  account_name: string,
  account_type: 'A' | 'L' | 'C' | 'R' | 'E',
  amount: number,
): SplitDetail => ({
  id: 0,
  account_id: 0,
  account_name,
  account_type,
  amount,
  currency: 'TWD',
  memo: '',
});

describe('determineType: Investment', () => {
  it('detects sell with realized gain', () => {
    const splits = [
      sp('Assets:Investments:00878', 'A', -5287),
      sp('Assets:Bank:DAWHO', 'A', 8118),
      sp('Revenue:Realized', 'R', -2831),
    ];
    expect(determineType(splits)).toBe('Investment');
  });

  it('detects buy with fee', () => {
    const splits = [
      sp('Assets:Investments:0050', 'A', 20490),
      sp('Assets:Bank:DAWHO', 'A', -20519),
      sp('Expenses:Fees:Stocks', 'E', 29),
    ];
    expect(determineType(splits)).toBe('Investment');
  });

  it('detects clean broker-to-broker transfer', () => {
    const splits = [
      sp('Assets:Investments:0050', 'A', 1000),
      sp('Assets:Bank:DAWHO', 'A', -1000),
    ];
    expect(determineType(splits)).toBe('Investment');
  });

  it('falls through when no non-Investments A/L cash side', () => {
    const splits = [
      sp('Assets:Investments:0050', 'A', 100),
      sp('Assets:Investments:0050', 'A', -100),
    ];
    expect(determineType(splits)).toBe('Transfer');
  });
});
```

- [ ] **Step 9.3: Run the test to verify it fails**

Run: `pnpm --filter spa test -- determineType` (or `npm test` / `yarn test` per the project's setup — inspect `spa/package.json` for the script name).
Expected: FAIL.

- [ ] **Step 9.4: Add the Investment branch to `determineType.ts`**

In `spa/src/lib/determineType.ts`, modify as follows. Declare the new tracking variables:

```ts
  let hasInvestmentAccount = false;
  let nonInvestmentAssetOrLiabCnt = 0;
```

Inside the loop, replace the `case 'A':` and `case 'L':` blocks:

```ts
      case 'A':
        assetOrLiabCnt++;
        if (s.account_name.startsWith('Assets:Investments:')) {
          hasInvestmentAccount = true;
        } else {
          nonInvestmentAssetOrLiabCnt++;
        }
        if (s.amount > 0) {
          isAssetIncrease = true;
          totalPositiveAssetLiabAmount += s.amount;
        }
        break;
      case 'L':
        assetOrLiabCnt++;
        nonInvestmentAssetOrLiabCnt++;
        if (s.amount > 0) totalPositiveAssetLiabAmount += s.amount;
        break;
```

After `if (isOpening) return 'Opening';` add:

```ts
  if (hasInvestmentAccount && nonInvestmentAssetOrLiabCnt >= 1) return 'Investment';
```

- [ ] **Step 9.5: Run the test to verify it passes**

Run: `pnpm --filter spa test -- determineType`
Expected: PASS.

- [ ] **Step 9.6: Commit**

```bash
git add spa/src/lib/types.ts spa/src/lib/determineType.ts spa/src/test/determineType.test.ts
git commit -m "feat(spa): add Investment to TransactionType union and classifier"
```

---

## Task 10: SPA — display helpers and TypeBadge

**Files:**
- Modify: `spa/src/lib/transactionDisplay.ts`
- Modify: `spa/src/components/transactions/TypeBadge.tsx:4-12`
- Test: `spa/src/test/transactionDisplay.test.ts` (create if absent)

- [ ] **Step 10.1: Write failing tests for the display helpers**

Create `spa/src/test/transactionDisplay.test.ts` (or extend the existing one):

```ts
import { describe, expect, it } from 'vitest';
import { displayAccount, displayAmount, displayOffsetAccount } from '@/lib/transactionDisplay';
import type { SplitDetail } from '@/lib/types';

const sp = (
  account_name: string,
  account_type: 'A' | 'L' | 'C' | 'R' | 'E',
  amount: number,
): SplitDetail => ({
  id: 0,
  account_id: 0,
  account_name,
  account_type,
  amount,
  currency: 'TWD',
  memo: '',
});

describe('Investment display helpers', () => {
  const sellSplits = [
    sp('Assets:Investments:00878', 'A', -5287),
    sp('Assets:Bank:DAWHO', 'A', 8118),
    sp('Revenue:Realized', 'R', -2831),
  ];
  const buySplits = [
    sp('Assets:Investments:0050', 'A', 20490),
    sp('Assets:Bank:DAWHO', 'A', -20519),
    sp('Expenses:Fees:Stocks', 'E', 29),
  ];

  it('displayAccount returns the Investments account', () => {
    expect(displayAccount(sellSplits, 'Investment')).toBe('Assets:Investments:00878');
    expect(displayAccount(buySplits, 'Investment')).toBe('Assets:Investments:0050');
  });

  it('displayOffsetAccount returns the cash account', () => {
    expect(displayOffsetAccount(sellSplits, 'Investment', 'Assets:Investments:00878'))
      .toBe('Assets:Bank:DAWHO');
    expect(displayOffsetAccount(buySplits, 'Investment', 'Assets:Investments:0050'))
      .toBe('Assets:Bank:DAWHO');
  });

  it('displayAmount returns the cash-side magnitude', () => {
    expect(displayAmount(sellSplits, 'Investment')).toEqual({ amount: 8118, currency: 'TWD' });
    expect(displayAmount(buySplits, 'Investment')).toEqual({ amount: 20519, currency: 'TWD' });
  });
});
```

- [ ] **Step 10.2: Run tests to verify they fail**

Run: `pnpm --filter spa test -- transactionDisplay`
Expected: FAIL.

- [ ] **Step 10.3: Add Investment branches**

In `spa/src/lib/transactionDisplay.ts`:

In `displayAccount`, add this case before the final `return`:

```ts
    case 'Investment':
      for (const s of splits) {
        if (s.account_name.startsWith('Assets:Investments:')) return s.account_name;
      }
      break;
```

In `displayOffsetAccount`, after the existing logic, special-case Investment. The cleanest insertion is to replace the function body's branching with:

```ts
export function displayOffsetAccount(
  splits: SplitDetail[],
  type: TransactionType | string,
  primaryAccount: string,
): string {
  if (splits.length === 0) return '-';

  const seen = new Set<string>();

  if (type === 'Investment') {
    for (const s of splits) {
      if (s.account_type !== 'A' && s.account_type !== 'L') continue;
      if (s.account_name.startsWith('Assets:Investments:')) continue;
      seen.add(s.account_name);
    }
  } else {
    const primaryType = type === 'Expense' ? 'E' : type === 'Income' ? 'R' : null;
    if (primaryType !== null) {
      for (const s of splits) {
        if (s.account_type !== primaryType) seen.add(s.account_name);
      }
    } else {
      for (const s of splits) {
        if (s.account_name !== primaryAccount) seen.add(s.account_name);
      }
    }
  }

  if (seen.size === 0) return '-';
  if (seen.size === 1) return seen.values().next().value as string;
  return '(multiple)';
}
```

In `displayAmount`, add an Investment case before the fallback:

```ts
    case 'Investment': {
      let best = 0;
      let chosen = currency;
      for (const s of splits) {
        if (s.account_type !== 'A' && s.account_type !== 'L') continue;
        if (s.account_name.startsWith('Assets:Investments:')) continue;
        const abs = Math.abs(s.amount);
        if (abs > best) {
          best = abs;
          chosen = s.currency;
        }
      }
      return { amount: best, currency: chosen };
    }
```

In `spa/src/components/transactions/TypeBadge.tsx:4`, add the Investment palette:

```ts
const TYPE_CLASSES: Record<TransactionType, string> = {
  Expense: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200',
  Income: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200',
  Transfer: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200',
  Opening: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  Deposit: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
  Withdrawal: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200',
  Other: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  Investment: 'bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200',
};
```

- [ ] **Step 10.4: Run tests to verify they pass**

Run: `pnpm --filter spa test`
Expected: PASS.

- [ ] **Step 10.5: Commit**

```bash
git add spa/src/lib/transactionDisplay.ts \
        spa/src/components/transactions/TypeBadge.tsx \
        spa/src/test/transactionDisplay.test.ts
git commit -m "feat(spa): display helpers and badge for Investment"
```

---

## Task 11: SPA — dropdowns in `FilterBar`, `SimpleFields`, `TransactionForm`

**Files:**
- Modify: `spa/src/components/transactions/FilterBar.tsx`
- Modify: `spa/src/components/transactions/SimpleFields.tsx`
- Modify: `spa/src/components/transactions/TransactionForm.tsx`

- [ ] **Step 11.1: Locate the type lists**

Run: `grep -n "'Expense'" spa/src/components/transactions/FilterBar.tsx spa/src/components/transactions/SimpleFields.tsx spa/src/components/transactions/TransactionForm.tsx`

Each match identifies a hard-coded enumeration of transaction types used for a `<select>` / dropdown.

- [ ] **Step 11.2: Add `'Investment'` to every such enumeration**

For each of the three files, locate the array (it will look like `['Expense', 'Income', 'Transfer', ...]` or a `Record<TransactionType, ...>` map) and append `'Investment'` so the option appears in the UI. Match the existing order of the union in `lib/types.ts` (`Investment` last).

- [ ] **Step 11.3: Build the SPA**

Run: `pnpm --filter spa build` (or the project's build command).
Expected: no TypeScript errors. Any missing `Investment` key in a `Record<TransactionType, X>` map elsewhere will surface here — fix each by adding the missing entry. (Common spots: badge maps, color maps, label maps.)

- [ ] **Step 11.4: Run all SPA tests**

Run: `pnpm --filter spa test`
Expected: PASS.

- [ ] **Step 11.5: Commit**

```bash
git add spa/src/components/transactions/FilterBar.tsx \
        spa/src/components/transactions/SimpleFields.tsx \
        spa/src/components/transactions/TransactionForm.tsx
git commit -m "feat(spa): expose Investment in transaction dropdowns"
```

---

## Task 12: End-to-end verification

**Files:** (no edits — verification only)

- [ ] **Step 12.1: Full Go test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 12.2: Full SPA test suite**

Run: `pnpm --filter spa test` (and `pnpm --filter spa build` if not run earlier).
Expected: PASS.

- [ ] **Step 12.3: Manual smoke — create an Investment transaction via CLI**

```bash
make build
./kea_test ledger add test-investment /tmp/test-investment.db
./kea_test ledger switch test-investment
./kea_test account add Assets:Bank:DAWHO --type A --currency TWD
./kea_test account add Assets:Investments:0050 --type A --currency TWD
./kea_test account add Expenses:Fees:Stocks --type E --currency TWD
./kea_test add --type investment --from Assets:Bank:DAWHO --to Assets:Investments:0050 --amount 100 --description "buy test"
./kea_test transaction list
```

Expected output for the last command: the row's Type column shows `Investment`, Account column shows `Assets:Investments:0050`, OffsetAccount shows `Assets:Bank:DAWHO`, Amount shows `100`.

- [ ] **Step 12.4: Manual smoke — start the web server and view the transaction**

```bash
./kea_test serve &
```

In a browser, open the SPA, navigate to Transactions. Expected: the just-created transaction shows with the violet "Investment" badge, the bank as offset, and amount 100. The type-filter dropdown includes "Investment".

- [ ] **Step 12.5: Final commit if any verification fix-ups were needed**

If any of the above surfaced missed call sites, fix them, re-run the relevant suite, then:

```bash
git add -A
git commit -m "fix: address Investment type integration findings from e2e smoke"
```

Otherwise, no commit needed.

- [ ] **Step 12.6: Push the branch**

```bash
git push -u origin feat/investment-transaction-type
```

The feature is ready for PR.

---

## Self-Review Notes

- Spec coverage check:
  - Decision table → Tasks 2, 3, 4, 5 (classifier, validation, display, picker/rule)
  - Architecture → Model (Task 1), Classifier (Tasks 2–5), Migration (Task 6), `cmd/add` (Tasks 7–8), SPA (Tasks 9–11), end-to-end (Task 12)
  - Open Question (flag-mode multi-split): per spec recommendation, `--type investment` in two-account flag mode is unchanged; fee/realized-gain splits are added via `kea transaction edit` after the fact or via the existing `--split` flag-mode multi-split path (which `cmd/add_actions.go:204` already handles). No additional task needed.
- Type-consistency check:
  - `TxTypeInvestment` string value is `"Investment"` everywhere (Go const, SQL backfill literal, SPA union, badge map key, prompt label parsing).
  - `IsInvestmentAccount` / `InvestmentAccountPrefix` defined once in `internal/model/types.go`, mirrored as a literal `'Assets:Investments:'` string check in SPA (which doesn't import Go constants).
  - `GetDisplayAmount` signature change cascades to its only caller (`BuildTransactionListItems`) and to the existing test file in Task 4. Both are updated in the same task.
