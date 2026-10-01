# Fix: `CreateSimpleTransaction` `ErrNotFound` translation

**Status:** Design approved 2026-06-05.
**Sibling:** [`2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`](2026-06-05-fix-create-transaction-split-account-errnotfound-design.md) — fixed the same class of bug in `CreateTransaction`'s split loop (PR #180). This spec closes the remaining drift site.

## Problem

`internal/service/transaction_ops.go::CreateSimpleTransaction` (lines ~159–207) takes an optional `input.Type`. If `Type == ""`, it enters a type-inference branch (lines ~171–187) that calls `ts.accRepo.GetAccountByName` for both `FromAccount` and `ToAccount` and wraps each error unconditionally:

```go
fromAcc, err := ts.accRepo.GetAccountByName(ctx, input.FromAccount)
if err != nil {
    return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
}
toAcc, err := ts.accRepo.GetAccountByName(ctx, input.ToAccount)
if err != nil {
    return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
}
```

When the account name is unknown, the inner `err` is `repository.ErrNotFound`. The wrap preserves it via `%w`, but the service layer should never let that sentinel leak past its boundary — the established discipline is to translate to either `service.ErrNotFound` (mapped to HTTP 404 for path-bound IDs) or a `*ValidationError` (mapped to 400 for input fields).

### Current impact

`CreateSimpleTransaction` is reached today only from:
- `cmd/add.go:100` (CLI's `add` command)
- `internal/api/testhelper_test.go:48` (`seedTransaction` test helper)

The web API does **not** currently route through it — every API write endpoint (`CreateTransaction`, `CreateTransactionFromSplits`) requires `Type` to be set, which short-circuits the type-inference branch. Any future API endpoint that mirrors the CLI's `add` ergonomics would be exposed.

The CLI prints the raw error to the user. The current message is:

```
failed to resolve from account: account "X" not found: record not found
```

After the fix, the user sees:

```
from account "X" not found
```

A small but real user-visible improvement. Acceptable per design discussion.

## Decision

Translate `repository.ErrNotFound` to a `*ValidationError` with per-side field naming (`from_account` / `to_account`). The non-`ErrNotFound` branch keeps its existing `fmt.Errorf` wrap so genuine repository failures continue to surface as 500-class errors at any future API boundary.

Three field-naming options considered:

1. **`from_account` / `to_account`** *(chosen)*. Per-side specificity matches the structural information available — the caller knows exactly which input field to fix. Snake_case is the project's API field convention (`include_hidden`, `account_id`, `parent_id`). Ready for future API exposure without rename churn.
2. Generic `account`. Matches the existing "same account" validation at line 161, but loses the from/to distinction. Worse user-facing message.
3. CamelCase `fromAccount` / `toAccount`. Matches the Go struct field names but breaks project convention.

## Change

### `internal/service/transaction_ops.go`

Replace the two unconditional wraps in `CreateSimpleTransaction`'s type-inference branch:

```go
fromAcc, err := ts.accRepo.GetAccountByName(ctx, input.FromAccount)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return model.TransactionDetail{}, validationErrorf("from_account",
            "from account %q not found", input.FromAccount)
    }
    return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
}
toAcc, err := ts.accRepo.GetAccountByName(ctx, input.ToAccount)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return model.TransactionDetail{}, validationErrorf("to_account",
            "to account %q not found", input.ToAccount)
    }
    return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
}
```

`errors`, `fmt`, `repository`, and `model` are all already imported in the file (the post-PR-#180 import block has them).

### Tests

Two new subtests appended inside the existing `TestCreateSimpleTransaction` (`internal/service/transaction_ops_test.go::TestCreateSimpleTransaction`, around line 381). The file's convention is t.Run subtests inside one top-level test per service method.

```go
t.Run("unknown from account returns validation error on from_account", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    txRepo := newMockTransactionRepo()
    setupStandardAccounts(accRepo)
    svc := newTestTransactionService(accRepo, txRepo)

    _, err := svc.CreateSimpleTransaction(
        context.Background(),
        model.CreateSimpleTransactionInput{
            FromAccount: "Assets:NotReal",
            ToAccount:   "Expenses:Food",
            Amount:      100,
            Description: "x",
            Timestamp:   0,
            Status:      model.StatusPending,
            Type:        "", // empty -> enters the type-inference branch
        },
    )
    require.Error(t, err)
    var ve *ValidationError
    require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T: %v", err, err)
    assert.Equal(t, "from_account", ve.Field)
    assert.Contains(t, ve.Message, "Assets:NotReal")
})

t.Run("unknown to account returns validation error on to_account", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    txRepo := newMockTransactionRepo()
    setupStandardAccounts(accRepo)
    svc := newTestTransactionService(accRepo, txRepo)

    _, err := svc.CreateSimpleTransaction(
        context.Background(),
        model.CreateSimpleTransactionInput{
            FromAccount: "Assets:Bank",
            ToAccount:   "Expenses:NotReal",
            Amount:      100,
            Description: "x",
            Timestamp:   0,
            Status:      model.StatusPending,
            Type:        "", // empty -> enters the type-inference branch
        },
    )
    require.Error(t, err)
    var ve *ValidationError
    require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T: %v", err, err)
    assert.Equal(t, "to_account", ve.Field)
    assert.Contains(t, ve.Message, "Expenses:NotReal")
})
```

Both tests must set `Type: ""` — when `Type` is provided, the function skips the type-inference branch and delegates straight to `CreateTransaction` (which has its own split-loop translation, already fixed in PR #180). The bug surface only exists in the inference branch.

## Out of scope

- **CLI error formatting** (`cmd/add.go:100`). The CLI prints whatever error it receives. The improved message is a free side effect of the fix; no CLI-layer changes needed.
- **`CreateTransaction`'s split loop.** Already fixed in PR #180.
- **`DetermineType` and downstream calls.** Those errors are not `repository.ErrNotFound` and don't need translation.
- **A broader audit of every `repository.ErrNotFound` site.** PR #180's spec audited the service layer; this PR closes the one remaining drift the audit identified.

## Verification

- `go test ./internal/service/... -run TestCreateSimpleTransaction` — green; existing 6 subtests + 2 new.
- `go test ./...` and `go build ./...` — green.

## Commit shape

Single commit, scope `(service)`:

```
fix(service): translate repository.ErrNotFound in CreateSimpleTransaction type inference

CreateSimpleTransaction's type-inference branch wrapped GetAccountByName
errors unconditionally, leaking repository.ErrNotFound past the service
boundary. The web API never reaches this path (it requires Type to be set),
but the CLI's `add` command does — and any future API endpoint mirroring
the CLI's ergonomics would have inherited the leak.

Mirror the pattern just applied to CreateTransaction's split loop
(commit a8a08e2): translate repository.ErrNotFound to validationErrorf
with from_account / to_account field tags. The non-NotFound branch keeps
its existing wrap so genuine repo failures continue to surface.

Closes the audit identified in the 2026-06-05 ErrNotFound translation work.
```
