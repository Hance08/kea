# Fix: `CreateTransaction` split-account `ErrNotFound` translation

**Status:** Design approved 2026-06-05.
**Predecessor:** [2026-06-03-web-api-write-endpoints-design.md](2026-06-03-web-api-write-endpoints-design.md) — "Known rough edge" section.
**Sibling fix referenced:** [2026-06-04-web-api-reconcile-design.md](2026-06-04-web-api-reconcile-design.md) Task 3 (same class of bug, fixed for reconcile preview/commit account lookup).

## Problem

`TransactionService.CreateTransaction` resolves each split's account by name via `ts.accRepo.GetAccountByName(ctx, splitInput.AccountName)` and on error wraps with `fmt.Errorf("split #%d: %w", i+1, err)` ([`internal/service/transaction_ops.go`](../../../internal/service/transaction_ops.go), inside the split loop near the top of `CreateTransaction`).

When the account name does not exist, the inner error is `repository.ErrNotFound`. The wrap preserves it via `%w`, so `errors.Is(err, repository.ErrNotFound)` is true upstream — but `errors.Is(err, service.ErrNotFound)` is false.

`internal/api/errors.go`'s `mapError` matches only on `service.ErrNotFound` and `service.ErrReconciled` (plus `ErrValidation`). The repository sentinel falls through to the default branch, producing **HTTP 500** for a request that is in fact a malformed client request.

The symmetric case already does the right thing in `UpdateTransaction`'s split loop (`internal/service/transaction_ops.go`, line ~350):

```go
account, err := repo.GetAccountByID(ctx, split.AccountID)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return validationErrorf("splits", "account ID %d not found", split.AccountID)
    }
    return err
}
```

This produces a clean **400** with `error.code == "validation"` and `error.field == "splits"`.

`CreateTransaction` is the only remaining `GetAccountByName` / `GetAccountByID` call in `internal/service/` that leaks `repository.ErrNotFound` past the service boundary (audited 2026-06-05 — all other call sites translate to either `service.ErrNotFound` or a validation error).

## Decision

Translate at the service boundary by returning a validation error from `CreateTransaction`'s split loop.

Three options were considered:

1. **Validation error → 400** *(chosen)*. Mirrors `UpdateTransaction`'s existing handling of the same logical error, gives Create/Update API symmetry, and tags the bad field (`splits`) so SPA forms can highlight the input.
2. `service.ErrNotFound` → 404. Literal mirror of the reconcile preview/commit fix. Rejected because 404 is the wrong semantic for a resource referenced *inside a request body*, and because it would make Create and Update disagree on the same logical error.
3. Extend `mapError` to also translate `repository.ErrNotFound`. Rejected because it couples the API layer to the repository layer and contradicts the service-boundary discipline established in PR #179.

## Change

### 1. `internal/service/transaction_ops.go` — `CreateTransaction` split loop

Replace the unconditional wrap with a typed translation:

```go
account, err := ts.accRepo.GetAccountByName(ctx, splitInput.AccountName)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return 0, validationErrorf("splits",
            "split #%d: account %q not found", i+1, splitInput.AccountName)
    }
    return 0, fmt.Errorf("split #%d: %w", i+1, err)
}
```

The non-`ErrNotFound` branch keeps the existing `fmt.Errorf` wrap so genuine repository failures (DB errors, context cancellation, etc.) still surface as 500.

`repository` is already imported in this file; no new imports required.

### 2. `internal/service/transaction_ops_test.go` — new service-layer test

Add `TestCreateTransaction_UnknownSplitAccount_ReturnsValidationError`:

- Seed the mock with one valid account (`Assets:Bank`) plus the system opening-balance account if the helper requires it.
- Construct a 2-split `model.TransactionDetail` where one split references a name not present in the mock (e.g. `"Expenses:DoesNotExist"`).
- Call `svc.Transaction().CreateTransaction(ctx, input)`.
- Assert `errors.Is(err, ErrValidation)` is true.
- Assert the underlying `*ValidationError` has `Field == "splits"` and a message containing the unknown account name. (Use the same extraction pattern as existing validation tests in the file.)

This is the service-layer guarantee. The API test (below) confirms it surfaces correctly through `mapError`.

### 3. `internal/api/transactions_write_test.go` — flip the existing 500 assertion

Rename `TestHandleCreateTransaction_NonexistentAccount_Currently500` → `TestHandleCreateTransaction_NonexistentAccount`. Remove the stale "Known rough edge" comment block above it. Change the assertion from `StatusInternalServerError` to `StatusBadRequest`, and additionally:

- Decode the response body into the API's standard error envelope.
- Assert `error.code == "validation"` and `error.field == "splits"`.

This brings the API contract in line with the existing `TestHandleUpdateTransaction_*` cases for the same logical error.

### 4. `docs/superpowers/specs/2026-06-03-web-api-write-endpoints-design.md` — tidy stale notes

Three locations reference this rough edge:
- §"Known rough edge — `repository.ErrNotFound` in split account lookup" (line ~282)
- The test inventory note (line ~329) "Split referencing a nonexistent account → currently returns 500"
- The deferred-work list (line ~394) "Fix for `repository.ErrNotFound` vs `service.ErrNotFound` mismatch..."

Replace each with a one-line resolution pointer to this spec. The original write-endpoints spec is gitignored (personal design library), so this is hygiene, not a PR artifact — but worth doing to keep the local spec set coherent.

## Out of scope

- `mapError` is **not** extended to handle `repository.ErrNotFound`. The service↔API boundary stays as it is.
- No changes to repository code, mock helpers, or other endpoints.
- No audit of non-account `GetAccountBy*` callers outside `internal/service/` — the grep audit covers the service layer only, which is the relevant translation surface.

## Verification

- `go test ./internal/service/... -run TestCreateTransaction` — new test passes.
- `go test ./internal/api/... -run TestHandleCreateTransaction` — flipped test passes; other Create/Update tests remain green.
- `go test ./...` and `go build ./...` clean.

## Commit shape

Single commit. The service-layer translation and the API test flip are tightly coupled — the test is the API contract change.

```
fix(service): translate repository.ErrNotFound to validation error in CreateTransaction split lookup

A request body referencing a nonexistent split account name previously
surfaced as HTTP 500 because the service layer wrapped repository.ErrNotFound
without translating it. CreateTransaction now mirrors UpdateTransaction's
existing handling and returns a validationErrorf("splits", ...), yielding a
400 with a field-tagged error envelope.
```
