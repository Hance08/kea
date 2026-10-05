# Split Account Type Trust Boundary Fix

**Date:** 2026-05-20
**Issue:** #153

## Problem

`ValidateSplitsMatchType` and three other functions (`DetermineType`, `GetDisplayAccount`, `GetDisplayOffsetAccount`) trust `SplitDetail.AccountType` when it is non-empty instead of resolving the actual account type from the repository. A direct service caller can supply a fake `AccountType` and bypass type validation, creating transactions whose declared type does not match the real accounts in the database.

Additionally, `CreateTransaction` passes the original caller-supplied `input.Splits` to `ValidateSplitsMatchType` rather than using already-resolved account data.

## Approach

Always resolve account types from the repository. Never treat `SplitDetail.AccountType` as authoritative for business logic.

## Design

### Shared helper: `resolveAccountType`

Add an unexported method on `TransactionService`:

```go
func (ts *TransactionService) resolveAccountType(ctx context.Context, s model.SplitDetail) (model.AccountType, error)
```

Resolution order:
1. If `s.AccountID > 0`: resolve via `accRepo.GetAccountByID(ctx, s.AccountID)`
2. Else if `s.AccountName != ""`: resolve via `accRepo.GetAccountByName(ctx, s.AccountName)`
3. Else: return an error (no identifier provided)

The `s.AccountType` field is never read.

### Functions updated

1. **`ValidateSplitsMatchType`** (transaction_classifier.go) — replace inline `resolveType` closure with `ts.resolveAccountType`.
2. **`DetermineType`** (transaction_classifier.go) — replace the inline resolution block with `ts.resolveAccountType`.
3. **`GetDisplayAccount`** (transaction_classifier.go) — replace inline `resolveType` closure with `ts.resolveAccountType`.
4. **`GetDisplayOffsetAccount`** (transaction_classifier.go) — replace inline `resolveAccountType` closure with `ts.resolveAccountType`.

### No caller changes

`CreateTransaction` and `UpdateTransactionComplete` remain unchanged. The validation function itself is now trustworthy regardless of what the caller passes in the splits.

## Testing

Service-layer tests using existing mock infrastructure:

- A split with a **lying `AccountType`** (e.g., Revenue account claimed as Expense) is rejected when it doesn't match the declared transaction type.
- A split with the **correct `AccountType`** still passes validation (repo is source of truth, field is ignored).
- Both `AccountID`-based and `AccountName`-based resolution paths are exercised.
- `DetermineType` returns the correct type based on repo data, not caller-supplied types.

## Scope

- No model changes.
- No repository interface changes.
- No caller/command changes.
- Only `internal/service/transaction_classifier.go` and its test file are modified.
