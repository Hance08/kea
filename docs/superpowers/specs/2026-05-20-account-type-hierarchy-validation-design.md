# Account Type Hierarchy Validation

**Issue:** [#151](https://github.com/Hance08/kea/issues/151)
**Date:** 2026-05-20

## Problem

`AccountService.CreateAccount` and `CreateAccountWithBalance` accept account name and type independently without verifying consistency. A direct service caller can persist accounts like `Expenses:Food` with type `A` (Asset), or create a child under an Asset parent with type `E` (Expense). Reports use the stored type while the UI displays hierarchy by name, causing data-integrity breaks.

## Approach

Approach B: root-segment check in `validateAccountFields` (pure field validation, no I/O), parent-type check extracted as a private helper called from both `CreateAccount` and `CreateAccountWithBalance` (needs repo access to load parent).

## Changes

### 1. New model helper — `AccountTypeFromRootName`

Location: `internal/model/types.go`

Inverse of the existing `RootName()` method. Maps a root segment string (case-insensitive) to its expected `AccountType`. Returns `(AccountType, bool)` following the same pattern as `RootName()`.

Covers all five roots: Assets→A, Liabilities→L, Equity→C, Revenue→R, Expenses→E.

### 2. Root-segment validation — in `validateAccountFields`

Location: `internal/service/account_ops.go`, inside `validateAccountFields`

After the `accType.IsValid()` check, extract the root segment from the full name (first part before `:`), call `model.AccountTypeFromRootName`, and reject with a `*ValidationError` (field `"type"`) if the expected type differs from the provided `accType`.

### 3. Parent-type validation — new private method `validateParentType`

Location: `internal/service/account_ops.go`

New method `validateParentType(ctx, parentID *int64, accType AccountType) error` on `AccountService`. If `parentID` is non-nil, loads the parent via `GetAccountByID` and rejects with a `*ValidationError` (field `"type"`) if `parent.Type != accType`.

Called from both `CreateAccount` and `CreateAccountWithBalance` after `validateAccountFields` succeeds.

### 4. No new sentinel errors

Both checks use the existing `validationErrorf("type", ...)` pattern, returning `*ValidationError`. No new sentinel error needed.

### 5. Tests

#### Model tests (`internal/model/`)
- `AccountTypeFromRootName` round-trips with `RootName()` for all five types
- Unknown root returns `("", false)`

#### Service tests (`internal/service/`)
- **Root mismatch**: `CreateAccount` with name `"Expenses:Food"`, type `AccountTypeAsset` → `*ValidationError` with field `"type"`
- **Parent mismatch**: parent `Assets:Bank` (type A), child `"Assets:Bank:Dining"` with type E → `*ValidationError` with field `"type"`
- **Happy path — root match**: `"Assets:Cash"` with type A succeeds
- **Happy path — parent match**: parent `Assets:Bank` (type A), child `"Assets:Bank:Savings"` with type A succeeds
- **CreateAccountWithBalance** gets the same coverage via the shared validation path

## Files modified

| File | Change |
|------|--------|
| `internal/model/types.go` | Add `AccountTypeFromRootName` |
| `internal/service/account_ops.go` | Add root check in `validateAccountFields`, add `validateParentType`, call it from both create methods |
| `internal/service/errors.go` | No changes |
| `internal/model/types_test.go` | Tests for `AccountTypeFromRootName` |
| `internal/service/account_ops_test.go` | Tests for root mismatch, parent mismatch, and happy paths |
