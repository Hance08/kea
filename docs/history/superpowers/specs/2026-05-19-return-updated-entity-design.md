# Return Updated Entity from Mutation Methods

**Issue:** [#105](https://github.com/Hance08/kea/issues/105)
**Date:** 2026-05-19

## Problem

Two issues with account mutation methods in the service layer:

1. `RenameAccount` accepts only a leaf segment for the new name, forcing callers to reconstruct the full path — logic that is duplicated between the service internals and `cmd/account/edit_actions.go`.
2. Both `RenameAccount` and `UpdateAccountMetadata` return only `error`. Callers (and a future web layer) must re-fetch the account to get the updated state.

## Design

### Signature Changes

**`RenameAccount`** (`account_ops.go`):

```go
// Before
func (as *AccountService) RenameAccount(ctx context.Context, oldName, newSegment string) error

// After
func (as *AccountService) RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error)
```

- Accepts `newFullName` (the complete account path) instead of `newSegment` (leaf only).
- Validates the last segment of `newFullName` (no colons, not empty, not reserved).
- Returns the freshly fetched `*model.Account` after a successful rename.

**`UpdateAccountMetadata`** (`account_service.go`):

```go
// Before
func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error

// After
func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) (*model.Account, error)
```

- Same parameters; now returns `*model.Account` after mutation.

### Internal Logic Changes

**`RenameAccount`:**

1. Fetch account by `oldName`.
2. Guard: system account check (unchanged).
3. Extract the last segment from `newFullName` and validate it via `ValidateAccountName`.
4. Check that `newFullName` doesn't already exist (unchanged, but now uses `newFullName` directly instead of reconstructing it).
5. Call `repo.RenameAccount(ctx, oldName, newFullName)`.
6. Re-fetch the account by `newFullName` via `repo.GetAccountByName` and return it.

**`UpdateAccountMetadata`:**

1. Fetch account by ID (unchanged).
2. Guard: system account check (unchanged).
3. Call `repo.UpdateAccountMetadata(ctx, accountID, description, isHidden)`.
4. Re-fetch the account by ID via `repo.GetAccountByID` and return it.

### CLI Caller Update

**`cmd/account/edit_actions.go` — `applyChanges`:**

- Move the full-name reconstruction (prefix + new segment) *before* calling `RenameAccount`, passing the full name.
- Use the returned `*model.Account` instead of manually tracking `finalName`.
- For `UpdateAccountMetadata`, use the returned account (though `finalName` is the primary output here, so the return value simplifies the flow).

### Validation Change

`RenameAccount` currently validates the raw `newSegment` parameter. After this change, it receives `newFullName` and must extract the leaf segment for validation. The extraction uses the same `strings.LastIndex(name, ":")` logic that was previously used for reconstruction — but now it's used to *decompose* the input rather than *compose* it.

### Test Updates

All existing tests for both methods need signature updates:

- `TestRenameAccount` tests (8 tests in `account_ops_test.go`): change `newSegment` args to `newFullName`, assert `*model.Account` return on success cases, verify the returned account has the correct new name.
- `TestUpdateAccountMetadata` tests (4 tests in `account_service_test.go`): assert `*model.Account` return on success, verify returned account reflects updated description/hidden.
- Mock `GetAccountByName` and `GetAccountByID` must return the post-mutation state for re-fetch calls.

## Scope

- `internal/service/account_ops.go` — `RenameAccount` signature + logic
- `internal/service/account_service.go` — `UpdateAccountMetadata` signature + logic
- `internal/service/account_ops_test.go` — rename tests
- `internal/service/account_service_test.go` — metadata tests
- `cmd/account/edit_actions.go` — caller update
