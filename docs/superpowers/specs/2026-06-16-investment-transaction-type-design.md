# Investment Transaction Type Design

**Date:** 2026-06-16
**Status:** Draft

## Problem

Today, investment activity (buying and selling stocks) does not classify cleanly under the existing transaction types. With the current classifier in `internal/service/transaction_classifier.go`:

- A **sell** with splits `Assets:Investments:00878 -5287`, `Assets:Bank:DAWHO +8118`, `Revenue:RealizedInvestmentIncome -2831` classifies as **Income**, and the list shows the bank amount (8118) under a misleading "Income" label.
- A **buy** with splits `Assets:Investments:0050 +20490`, `Assets:Bank:DAWHO -20519`, `Expenses:Fees:Stocks +29` classifies as **Transfer** because the positive Asset side exceeds the small Expense side — the small fee defeats the existing expense heuristic.

Neither label is right. The user wants a dedicated **Investment** type so:
1. The Type column on the transaction list reads "Investment".
2. The Account column shows the investment position (e.g., `Assets:Investments:0050`), not the bank account.
3. The Amount column shows the cash-side magnitude — the money that actually moved on the bank side.

## Goals

- Introduce `TxTypeInvestment = "Investment"` as a user-facing transaction type.
- Classify any transaction touching an `Assets:Investments:*` account as Investment, taking precedence over the existing Income / Expense / Transfer branches.
- Validate that an Investment transaction contains at least one `Assets:Investments:*` split and at least one other Asset/Liability split. Revenue and Expense splits are optional and unrestricted by name.
- Display the investment account as the primary Account, the bank account as the Offset, and the cash-side magnitude as the Amount.
- Make `kea add`, `kea transaction edit`, the SPA, and the JSON API accept and emit the new type.
- Backfill existing transactions whose splits match the Investment shape.

## Non-Goals

- Position-level reporting (cost basis, unrealized P&L, lots, FIFO/LIFO). The split shape stays generic — performance reporting is a separate spec.
- A new "Investment loss" Expense account. The user has no `Expenses:RealizedInvestmentLoss` today; validation accepts any Expense (or Revenue) split, so adding such an account later requires no code change.
- A "Buy" vs "Sell" sub-type. Direction is implicit in the sign of the `Assets:Investments:*` split.
- A new Reports page or chart aggregating investment activity.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Detection signal | Account name prefix `Assets:Investments:` | Mirrors the existing `Equity:OpeningBalances_*` prefix convention; no schema migration; flexible |
| Classifier precedence | Before Income / Expense / Transfer branches | Investment is more specific; must win over the generic heuristics |
| Validation rule | ≥1 `Assets:Investments:*` split + ≥1 other A/L split; R and E optional | Covers buy, sell, and zero-fee transfers between brokerages; forward-compatible with future loss/dividend accounts |
| Display Amount | Max `\|amount\|` among Asset/Liability splits whose account is **not** `Assets:Investments:*` | The bank-side cash movement — consistent with how every other type's Amount column reads as "money moved" |
| Display Account | The `Assets:Investments:*` split's account | The subject of the transaction ("I bought 0050"); mirrors how Expense shows the expense account |
| Display Offset | The non-Investments Asset/Liability account (or `(multiple)`) | Same pattern as Transfer's offset |
| Allowed accounts (picker) | Asset, Liability, Revenue, Expense (no Equity) | Permissive; investment activity legitimately touches fees, dividends, taxes |
| `add --type` value | `investment` | Lowercase, parallels `expense`, `income`, `transfer` |

## Architecture

### Model (`internal/model/`)

**`types.go`**

Add the new constant and extend `IsValid`, `ParseTransactionType`:

```go
const (
    // ... existing constants ...
    TxTypeInvestment TransactionType = "Investment"
)

func (t TransactionType) IsValid() bool {
    switch t {
    case TxTypeExpense, TxTypeIncome, TxTypeTransfer,
        TxTypeOpening, TxTypeDeposit, TxTypeWithdrawal, TxTypeOther,
        TxTypeInvestment:
        return true
    }
    return false
}

func ParseTransactionType(s string) (TransactionType, error) {
    switch strings.ToLower(strings.TrimSpace(s)) {
    // ... existing cases ...
    case "investment":
        return TxTypeInvestment, nil
    }
    // ...
}
```

Add a small helper alongside `IsOpeningBalancesAccount`:

```go
const InvestmentAccountPrefix = "Assets:Investments:"

func IsInvestmentAccount(name string) bool {
    return strings.HasPrefix(name, InvestmentAccountPrefix)
}
```

`ParseTransactionTypeLabel` (which maps prompt labels like "Record Expense" → type) gains an "investment" branch.

### Classifier (`internal/service/transaction_classifier.go`)

`DetermineType` is retained per the [Transaction Type Storage Design](2026-04-23-transaction-type-storage-design.md) for backfill and import tooling. Add the Investment branch **before** the existing rev/exp/asset checks:

```go
func (ts *TransactionService) DetermineType(ctx context.Context, splits []model.SplitDetail) (model.TransactionType, error) {
    if len(splits) == 0 {
        return model.TxTypeOther, nil
    }

    var (
        // ... existing counters ...
        hasInvestmentAccount  bool
        otherAssetLiabCount   int
    )

    for _, split := range splits {
        // ... existing per-split resolution ...
        if (accType == model.AccountTypeAsset || accType == model.AccountTypeLiability) &&
            model.IsInvestmentAccount(split.AccountName) {
            hasInvestmentAccount = true
        } else if accType == model.AccountTypeAsset || accType == model.AccountTypeLiability {
            otherAssetLiabCount++
        }
        // ... existing accumulation ...
    }

    if isOpening {
        return model.TxTypeOpening, nil
    }

    if hasInvestmentAccount && otherAssetLiabCount >= 1 {
        return model.TxTypeInvestment, nil
    }

    // ... existing branches unchanged ...
}
```

`AccountName` is available on `SplitDetail` and is sufficient to detect the prefix without an extra repo lookup.

**New: extend `ValidateSplitsMatchType`** with the Investment case:

```go
case model.TxTypeInvestment:
    var hasInvestment, hasOtherAssetOrLiab bool
    for _, s := range splits {
        accType, err := ts.resolveAccountType(ctx, s)
        if err != nil {
            return err
        }
        if (accType == model.AccountTypeAsset || accType == model.AccountTypeLiability) {
            if model.IsInvestmentAccount(s.AccountName) {
                hasInvestment = true
            } else {
                hasOtherAssetOrLiab = true
            }
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

Note: when `ValidateSplitsMatchType` is called from the API layer, `SplitDetail.AccountName` may be empty if the caller supplied `AccountID` only. In that case, the existing `resolveAccountType` repo lookup also returns the full account row — extend the resolver to populate `AccountName` when missing, or look it up explicitly. Concretely: resolve the name once per split via `accRepo.GetAccountByID` if `s.AccountName == ""`, and check the prefix on the resolved name.

**Extend `GetDisplayAccount`** with the Investment branch:

```go
case "Investment":
    for _, split := range splits {
        if model.IsInvestmentAccount(split.AccountName) {
            return split.AccountName, nil
        }
    }
```

**Replace `GetDisplayAmount`** with type-aware logic. Today it returns the max positive across all splits, which produces the wrong number for Investment. New signature:

```go
func (ts *TransactionService) GetDisplayAmount(splits []model.SplitDetail, txType string) (int64, string)
```

For `txType == "Investment"`: scan splits, take the max `|amount|` among Asset/Liability splits whose account name does **not** start with `Assets:Investments:`. Currency comes from that split.

For all other types: existing behavior (max positive amount). The signature change cascades to [BuildTransactionListItems](internal/service/transaction_classifier.go:234) and any other caller.

**Extend `GetDisplayOffsetAccount`** with the Investment branch: the offset is the non-Investments Asset/Liability account. Reuse the existing `(multiple)` fallback when more than one qualifies.

**Extend `GetAllowedAccounts`** with the Investment branch:

```go
case model.TxTypeInvestment:
    return ts.filterAccountsByTypes(allAccounts, []model.AccountType{
        model.AccountTypeAsset,
        model.AccountTypeLiability,
        model.AccountTypeRevenue,
        model.AccountTypeExpense,
    })
```

### Service Rule Registry (`internal/service/transaction_service.go`)

`GetTransactionRule` gains an Investment entry. Since Investment transactions touch three or more accounts (not the simple source/dest model that powers the two-account `kea add` interactive flow), the rule's `SourceTypes`/`DestTypes` are advisory: source = `A,L` (cash or another investment), dest = `A,L` (the investment account):

```go
case model.TxTypeInvestment:
    return model.TransactionRule{
        TxType:      model.TxTypeInvestment,
        SourceTypes: []string{"A", "L"},
        DestTypes:   []string{"A", "L"},
    }, nil
```

A clean Investment transaction in the `kea add` interactive flow uses these to drive the initial two-account capture (cash → investment account). Additional Revenue/Expense splits are added via the multi-split editor in `kea transaction edit` after creation. (See **Open Question** below.)

### Migration

**`0009_backfill_investment_type.up.sql`**

Reclassify existing rows whose split shape matches Investment. Apply only to rows currently typed as `'Income'`, `'Transfer'`, or `'Other'` — never overwrite `'Opening'`, `'Expense'` (won't match anyway), or `'Investment'`.

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

**`0009_backfill_investment_type.down.sql`** — reclassify back using the legacy classifier. Conservative form: rerun the same shape detection but set `type` to `'Income'` for transactions containing a Revenue split, `'Transfer'` for those with two A/L only (no Revenue), else `'Other'`. The migration is meant to be reversible; perfect round-trip is not required since the legacy classifier itself is lossy on these rows.

### `cmd/add`

`addFlags.Type` accepts `"investment"`. `runFromFlags` parses it via `model.ParseTransactionType`. The flag-mode path supports two-account creation only (cash → investment); to add a fee or realized-gain split, the user runs `kea transaction edit <id>` afterward. Document this constraint in the `--type` flag help text:

```
--type    Transaction type: expense, income, transfer, investment
          (investment in flag mode creates two splits;
           use 'kea transaction edit' to add fee or gain splits)
```

`PromptTransactionType()` in `ui/prompts/transaction.go` adds `"Record Investment"`. `ParseTransactionTypeLabel` maps it to `TxTypeInvestment`. `modeUIConfigs` in `cmd/add_actions.go` adds:

```go
model.TxTypeInvestment: {"Cash Account:", "Investment Account:"},
```

The interactive flow's two-account model picks the cash side first (any Asset/Liability except `Assets:Investments:*`) and the investment side second (filtered to `Assets:Investments:*` only). This requires a small extension to `selectAccount`'s filtering — see **Open Question** for whether to add a per-account name-prefix filter or accept all A/L and post-filter.

### `cmd/transaction/edit`

The "Change Type" action ([edit_actions.go:258](cmd/transaction/edit_actions.go:258)) calls `PromptTransactionType` and re-validates. No structural change beyond accepting the new label.

The "Change Type" action's hidden-on-Opening guard at [edit.go:108](cmd/transaction/edit.go:108) does not need to exclude Investment.

### `cmd/transaction/list`

`BuildTransactionListItems` now calls `GetDisplayAmount(splits, txType)`. No structural change at this call site beyond passing the type.

### Display label

The list view's Type column shows the raw `TxTypeInvestment` string (`"Investment"`). No mapping table is needed; matches how `"Expense"`, `"Income"`, etc. render today.

### Web API

JSON marshalling of `TransactionType` already round-trips through `MarshalJSON` / `UnmarshalJSON` ([types.go:163](internal/model/types.go:163)) and rejects unknown values via `IsValid`. Adding the new constant to `IsValid` makes API requests carrying `"type": "Investment"` accept and emit naturally. No handler changes.

### SPA (`spa/src/`)

The SPA carries a TypeScript mirror of the Go classifier. All of these change in parallel:

- **[`lib/types.ts:37`](spa/src/lib/types.ts:37)** — add `'Investment'` to the `TransactionType` union.
- **[`lib/determineType.ts`](spa/src/lib/determineType.ts)** — add the same Investment branch as the Go classifier, using `s.account_name.startsWith('Assets:Investments:')` against `account_type === 'A' || 'L'`. Place before the Income/Expense/Transfer branches.
- **[`lib/transactionDisplay.ts`](spa/src/lib/transactionDisplay.ts)** — extend `displayAccount`, `displayOffsetAccount`, `displayAmount` with the Investment branch:
  - `displayAccount`: return the split where `account_name.startsWith('Assets:Investments:')`.
  - `displayAmount`: return `|amount|` of the max-magnitude Asset/Liability split whose name does **not** start with `Assets:Investments:`. Sign convention: positive (matches Transfer's positive-amount convention).
  - `displayOffsetAccount`: the non-Investments Asset/Liability account, or `(multiple)`.
- **[`components/transactions/TypeBadge.tsx:4`](spa/src/components/transactions/TypeBadge.tsx:4)** — add an `Investment` entry to `TYPE_CLASSES`. Suggested palette: `bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200` (distinct from Transfer's blue).
- **`components/transactions/FilterBar.tsx`, `SimpleFields.tsx`, `TransactionForm.tsx`** — wherever the existing types are enumerated for dropdowns or form options, add `'Investment'`. Specific lines to be located during implementation.
- **Tests in `spa/src/test/`** — add Investment fixtures to `determineType`, `transactionDisplay`, and any list/form snapshot tests.

The SPA add-transaction form is two-split today; investment splits with fees are created via the edit modal's multi-split editor. No new SPA form layout required.

### Tests

- **`internal/model/types_test.go`** — extend `TestParseTransactionType`, `TestTransactionType_IsValid`, `TestTransactionType_MarshalJSON`, `TestTransactionType_UnmarshalJSON` tables.
- **New: `IsInvestmentAccount` test** covering the prefix, near-matches (`Assets:Investment:foo`, `Assets:Investments` with no colon), and the empty string.
- **`internal/service/transaction_classifier_test.go`** — table-driven cases for `DetermineType`:
  - Buy with fee → Investment
  - Sell with realized gain → Investment
  - Sell with no gain (clean broker transfer) → Investment
  - Transaction with `Assets:Investments:*` but no other A/L → falls through (treated as Other / existing behavior)
  - Existing Income/Expense/Transfer cases still classify correctly (no regression)
- **`ValidateSplitsMatchType`** test cases: missing investment split → error; missing cash side → error; both present → ok; with optional Revenue → ok; with optional Expense → ok.
- **`GetDisplayAmount`** test cases for Investment: returns bank-side magnitude for buy and sell.
- **`GetDisplayAccount`** / **`GetDisplayOffsetAccount`** Investment cases.
- **`GetAllowedAccounts`** Investment case returns A/L/R/E.
- **`GetTransactionRule`** Investment case.
- **Migration backfill test** — write a row of each shape (sell with gain, buy with fee, plain transfer between brokerages, plus a control non-investment Income and Transfer that must not be touched) and verify the migration sets `type` correctly only for the matching rows.

## Implementation Order

1. **Model** — `TxTypeInvestment` constant, `IsValid`, `ParseTransactionType`, `IsInvestmentAccount` helper.
2. **Classifier** — extend `DetermineType`, `ValidateSplitsMatchType`, `GetDisplayAccount`, `GetDisplayOffsetAccount`, `GetAllowedAccounts`; change `GetDisplayAmount` signature.
3. **Service rule registry** — extend `GetTransactionRule`.
4. **Tests for steps 1–3** — full coverage before moving on.
5. **Migration `0009`** — backfill existing rows; migration tests.
6. **`cmd/add`** — flag, interactive prompt label, mode UI config.
7. **`cmd/transaction/edit`** — verify "Change Type" handles Investment.
8. **`cmd/transaction/list`** — wire the `txType` arg to `GetDisplayAmount`.
9. **SPA** — type-options constant, color/badge.
10. **End-to-end smoke** — `make build`, full `go test ./...`, run `kea add --type investment ...` and verify the list view.

## Open Question

**Should the `kea add` flag-mode path for Investment accept fee/gain in a single command, or require post-edit?** Two paths:

- **A. Two-split only in flag mode** (this spec's current decision). `--type investment` produces a transaction with cash and investment splits balanced; fees and realized gains added via `kea transaction edit`. Simplest; matches today's flag-mode shape; explained in `--help` text.
- **B. Flag-mode multi-split.** Add `--fee` / `--realized-gain` flags. More flags, more validation, but a one-shot CLI workflow.

Recommendation: **A**. The interactive `kea add` flow is also two-account today; preserving symmetry keeps the flag-mode contract simple. Investment transactions with fees are infrequent enough that a follow-up `edit` is acceptable. To revisit if usage shows the edit step is a friction point.

If the user prefers **B** during spec review, the implementation plan will add the additional flags and validation steps.
