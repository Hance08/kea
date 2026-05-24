# Description & Memo Field Validation

**Issue:** #123  
**Date:** 2026-05-24  
**Branch:** `fix/issue-123-description-memo-validation`

## Problem

`CreateTransaction`, `UpdateTransactionComplete`, `CreateAccount`, and `UpdateAccountMetadata` do not validate the length or content of Description or Memo fields. Account names have `AccountNameMaxLength` but descriptions and memos have none. This allows multi-MB strings via the API and provides no defense-in-depth against stored XSS payloads.

The default description `"-"` is set only in `cmd/add_actions.go`, not in the service layer. API-created transactions can have truly empty descriptions.

## Approach

Inline validation in each service method using the existing `validationErrorf()` pattern (Approach A — no shared helpers, no model-layer `Validate()` methods).

## Constants

Added to `internal/model/types.go` alongside `AccountNameMaxLength`:

| Constant             | Value | Rationale                                        |
|----------------------|-------|--------------------------------------------------|
| `DescriptionMaxLength` | 500   | Generous for human descriptions, prevents DB bloat |
| `MemoMaxLength`        | 200   | Split-level notes, shorter than descriptions       |

## Validation Rules

### Description
- **Required:** must not be empty after `strings.TrimSpace()`.
- **Max length:** must not exceed `DescriptionMaxLength` (500) characters.
- The `"-"` default remains a CLI convention in `cmd/add_actions.go`. The service rejects empty descriptions; each client decides its own default.

### Memo
- **Optional:** empty string is allowed (splits don't always need a memo).
- **Max length:** must not exceed `MemoMaxLength` (200) characters when provided.

## Affected Methods

### 1. `CreateTransaction` (`internal/service/transaction_ops.go:24`)
- Validate `input.Description` (required + max length) in the existing validation block (after split count check).
- Validate each `splitInput.Memo` (max length only) in the existing split loop.

### 2. `UpdateTransactionComplete` (`internal/service/transaction_ops.go:255`)
- Validate `input.Description` (required + max length) in the existing validation block (after status check).
- Validate each split's `Memo` (max length only) before entering the `ExecTx` block.

### 3. `CreateAccount` (`internal/service/account_ops.go:49`)
- Validate `input.Description` (required + max length) before `validateAccountFields`.

### 4. `CreateAccountWithBalance` (`internal/service/account_ops.go:60`)
- Same description validation as `CreateAccount`, before `validateAccountFields`.

### 5. `UpdateAccountMetadata` (`internal/service/account_service.go:168`)
- Validate `description` parameter (required + max length) at method entry, before the `GetAccountByID` call.

## Error Format

Uses the existing `validationErrorf()` helper, returning `*ValidationError`:

```go
validationErrorf("description", "description is required")
validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
validationErrorf("memo", "split #%d memo too long (max %d characters)", i+1, model.MemoMaxLength)
```

## Testing

White-box tests using the existing mock infrastructure (`package service`). Each method gets tests for:

- Empty description is rejected with `ValidationError{Field: "description"}`
- Whitespace-only description is rejected
- Over-length description is rejected
- Over-length memo is rejected (for transaction methods)
- Valid inputs at the boundary (exactly max length) pass
- Existing tests continue to pass (no regressions)

## Out of Scope

- Moving the `"-"` default into the service layer (stays in `cmd/add_actions.go`).
- Content-based validation (e.g., character allowlists). Max length is sufficient defense-in-depth; output escaping is the responsibility of the presentation layer.
- Validating Description/Memo in store-layer methods (service is the validation boundary).
