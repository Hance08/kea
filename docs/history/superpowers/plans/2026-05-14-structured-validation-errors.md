# Structured Validation Errors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `ValidationError` type to the service layer so an HTTP API layer can distinguish validation failures (400) from infrastructure errors (500) using `errors.As`.

**Architecture:** Introduce `ValidationError` struct in `internal/service/errors.go` with `Field` (optional) and `Message` fields. Convert all pure-validation `fmt.Errorf()` calls across the service layer to return `&ValidationError{...}`. Leave sentinel-wrapping errors (`ErrNotEditable`, `ErrReconciled`, etc.) and infrastructure errors (`"failed to ..."`) untouched — those already map cleanly to HTTP status codes via `errors.Is`. Update existing tests to assert `errors.As(err, &ve)` where appropriate.

**Tech Stack:** Go standard library (`errors`, `fmt`), testify (`assert`, `require`)

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/service/errors.go` | Modify | Add `ValidationError` type + helper constructors |
| `internal/service/errors_test.go` | Create | Unit tests for `ValidationError` behavior |
| `internal/service/account_validation.go` | Modify | Convert ~16 validation `fmt.Errorf` → `ValidationError` |
| `internal/service/account_ops.go` | Modify | Convert ~6 pure-validation errors |
| `internal/service/account_service.go` | Modify | Convert ~4 validation errors in `resolveAndValidateAccount` |
| `internal/service/transaction_ops.go` | Modify | Convert ~12 pure-validation errors |
| `internal/service/transaction_validation.go` | Modify | Convert ~6 balance/currency errors |
| `internal/service/transaction_classifier.go` | Modify | Convert ~6 type-structure errors |
| `internal/service/transaction_service.go` | Modify | Convert 1 validation error |
| `internal/service/reconcile_ops.go` | Modify | Convert ~4 validation errors |
| `internal/service/report_service.go` | Modify | Convert ~4 date-format errors |
| `internal/service/account_ops_test.go` | Modify | Add `errors.As` assertions for validation paths |
| `internal/service/account_validation_test.go` | Modify | Assert returned errors are `*ValidationError` |
| `internal/service/transaction_ops_test.go` | Modify | Add `errors.As` assertions for validation paths |
| `internal/service/transaction_validation_test.go` | Modify | Assert returned errors are `*ValidationError` |

### What NOT to change

These error patterns stay as-is:

- **Sentinel-wrapping errors** — e.g., `fmt.Errorf("account %q: %w", name, ErrNotEditable)`. The API layer uses `errors.Is(err, ErrNotEditable)` → 403.
- **Infrastructure errors** — e.g., `fmt.Errorf("failed to create transaction: %w", err)`. These are repo/DB failures → 500.
- **Errors that wrap `ErrNotFound`** — e.g., `fmt.Errorf("account %q: %w", name, ErrNotFound)`. The API layer uses `errors.Is(err, ErrNotFound)` → 404.
- **Errors that wrap `ErrCircularParent`** — already a sentinel, detectable via `errors.Is`.

---

### Task 1: Define ValidationError type

**Files:**
- Modify: `internal/service/errors.go`
- Create: `internal/service/errors_test.go`

- [ ] **Step 1: Write failing tests for ValidationError**

Create `internal/service/errors_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: "name", Message: "name is required"}
	assert.Equal(t, "name is required", ve.Error())
}

func TestValidationError_ErrorsAs(t *testing.T) {
	ve := &ValidationError{Field: "amount", Message: "amount must be positive"}
	wrapped := fmt.Errorf("split #1: %w", ve)

	var target *ValidationError
	assert.True(t, errors.As(wrapped, &target))
	assert.Equal(t, "amount", target.Field)
	assert.Equal(t, "amount must be positive", target.Message)
}

func TestValidationError_Unwrap(t *testing.T) {
	inner := errors.New("parse error")
	ve := &ValidationError{Field: "date", Message: "invalid date", Err: inner}

	assert.True(t, errors.Is(ve, inner))
}

func TestValidationError_NilUnwrap(t *testing.T) {
	ve := &ValidationError{Field: "name", Message: "name is required"}
	assert.Nil(t, ve.Unwrap())
}

func TestValidationErrorf(t *testing.T) {
	err := validationErrorf("splits", "must have at least %d splits (got %d)", 2, 1)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "splits", ve.Field)
	assert.Equal(t, "must have at least 2 splits (got 1)", ve.Message)
}

func TestValidationErrorf_EmptyField(t *testing.T) {
	err := validationErrorf("", "source and destination cannot be the same")

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "", ve.Field)
}

func TestValidationWrap(t *testing.T) {
	inner := &ValidationError{Field: "", Message: "can't be empty"}
	err := validationWrap("name", "invalid account name", inner)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "name", ve.Field)
	assert.Contains(t, ve.Message, "invalid account name")
	assert.Contains(t, ve.Message, "can't be empty")
	assert.True(t, errors.Is(err, inner))
}

func TestValidationError_NotMatchSentinels(t *testing.T) {
	ve := &ValidationError{Field: "name", Message: "bad name"}
	assert.False(t, errors.Is(ve, ErrNotFound))
	assert.False(t, errors.Is(ve, ErrNotEditable))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestValidation -v`
Expected: FAIL — `ValidationError` type not defined

- [ ] **Step 3: Implement ValidationError type and helpers**

Add to `internal/service/errors.go` after the existing sentinel vars:

```go
// ValidationError represents a user-input validation failure.
type ValidationError struct {
	Field   string // which field failed (empty for cross-field validations)
	Message string
	Err     error  // optional wrapped error
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return e.Err }

func validationErrorf(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func validationWrap(field, prefix string, err error) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: prefix + ": " + err.Error(),
		Err:     err,
	}
}
```

Also add `"fmt"` to the imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestValidation -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/errors.go internal/service/errors_test.go
git commit -m "feat: add ValidationError type for structured error handling (issue #77)"
```

---

### Task 2: Convert account_validation.go

**Files:**
- Modify: `internal/service/account_validation.go`
- Modify: `internal/service/account_validation_test.go` (if it exists, otherwise test coverage comes from account_ops_test.go)

- [ ] **Step 1: Write a failing test that asserts ValidationError on account name validation**

Add to `internal/service/errors_test.go` (since `ValidateAccountName` requires an `AccountService` instance, and errors_test.go is in `package service`):

```go
func TestValidateAccountName_ReturnsValidationError(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name  string
		input string
		field string
	}{
		{"empty name", "", "name"},
		{"has colon", "foo:bar", "name"},
		{"too long", string(make([]byte, 256)), "name"},
		{"leading space", " foo", "name"},
		{"reserved name", "assets", "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateAccountName(tt.input)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
			assert.Equal(t, tt.field, ve.Field)
		})
	}
}

func TestValidateCurrency_ReturnsValidationError(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name  string
		input string
		field string
	}{
		{"too short", "US", "currency"},
		{"has digits", "U2D", "currency"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateCurrency(tt.input)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
			assert.Equal(t, tt.field, ve.Field)
		})
	}
}

func TestValidateFullAccountName_ReturnsValidationError(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name  string
		input string
		field string
	}{
		{"empty", "", "name"},
		{"bad root", "Foo:Bar", "name"},
		{"empty segment", "Assets::Checking", "name"},
		{"reserved in segment", "Assets:Equity", "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateFullAccountName(tt.input)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
			assert.Equal(t, tt.field, ve.Field)
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestValidateAccountName_ReturnsValidationError|TestValidateCurrency_ReturnsValidationError|TestValidateFullAccountName_ReturnsValidationError" -v`
Expected: FAIL — `errors.As` does not match `*ValidationError`

- [ ] **Step 3: Convert ValidateAccountName errors**

In `internal/service/account_validation.go`, replace all `fmt.Errorf(...)` returns in `ValidateAccountName` with `validationErrorf("name", ...)`:

```go
func (as *AccountService) ValidateAccountName(name string) error {
	if name != strings.TrimSpace(name) {
		return validationErrorf("name", "account name cannot start or end with spaces")
	}
	name = strings.TrimSpace(name)

	if name == "" {
		return validationErrorf("name", "account name can't be empty")
	}
	if strings.Contains(name, ":") {
		return validationErrorf("name", "account name cannot contain ':' character")
	}
	if model.ReservedNames[strings.ToLower(name)] {
		return validationErrorf("name", "'%s' is a reserved root account name", name)
	}
	if len(name) > model.AccountNameMaxLength {
		return validationErrorf("name", "account name too long (max %d characters)", model.AccountNameMaxLength)
	}
	return nil
}
```

- [ ] **Step 4: Convert ValidateFullAccountName errors**

Replace all `fmt.Errorf(...)` returns in `ValidateFullAccountName`:

```go
func (as *AccountService) ValidateFullAccountName(fullName string) error {
	if fullName != strings.TrimSpace(fullName) {
		return validationErrorf("name", "account name cannot start or end with spaces")
	}
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return validationErrorf("name", "account name can't be empty")
	}
	if len(fullName) > model.AccountNameMaxLength {
		return validationErrorf("name", "account name too long (max %d characters)", model.AccountNameMaxLength)
	}

	parts := strings.Split(fullName, ":")
	if len(parts) == 0 {
		return validationErrorf("name", "invalid account name")
	}

	root := strings.ToLower(strings.TrimSpace(parts[0]))
	if !model.ReservedNames[root] {
		return validationErrorf("name", "account root must be one of: Assets, Liabilities, Equity, Revenue, Expenses")
	}

	for i, part := range parts {
		if part != strings.TrimSpace(part) {
			return validationErrorf("name", "account segment at level %d cannot start or end with spaces", i+1)
		}
		segment := strings.TrimSpace(part)
		if segment == "" {
			return validationErrorf("name", "account name has empty segment at level %d", i+1)
		}
		if strings.Contains(segment, ":") {
			return validationErrorf("name", "account segment '%s' cannot contain ':'", segment)
		}
		if len(segment) > model.AccountNameMaxLength {
			return validationErrorf("name", "account segment too long (max %d characters)", model.AccountNameMaxLength)
		}
		if i > 0 && model.ReservedNames[strings.ToLower(segment)] {
			return validationErrorf("name", "account segment '%s' cannot use reserved root account name", segment)
		}
	}

	return nil
}
```

- [ ] **Step 5: Convert ValidateCurrency errors**

```go
func (as *AccountService) ValidateCurrency(currency string) error {
	currency = strings.TrimSpace(strings.ToUpper(currency))

	if currency == "" {
		return nil
	}
	if len(currency) != 3 {
		return validationErrorf("currency", "currency code must be 3 characters (e.g. USD)")
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			return validationErrorf("currency", "currency code must contain only letters")
		}
	}
	return nil
}
```

- [ ] **Step 6: Remove unused `fmt` import if needed**

After conversion, `account_validation.go` may no longer use `fmt`. Remove it if the compiler complains.

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestValidateAccountName|TestValidateCurrency|TestValidateFullAccountName" -v`
Expected: PASS

- [ ] **Step 8: Run full test suite to check for regressions**

Run: `go test ./internal/service/ -v`
Expected: PASS — existing tests that check error messages via `assert.Contains` or `assert.ErrorContains` should still pass because `ValidationError.Error()` returns the same message string.

- [ ] **Step 9: Commit**

```bash
git add internal/service/account_validation.go internal/service/errors_test.go
git commit -m "refactor: convert account validation errors to ValidationError (issue #77)"
```

---

### Task 3: Convert account_ops.go and account_service.go

**Files:**
- Modify: `internal/service/account_ops.go`
- Modify: `internal/service/account_service.go`
- Modify: `internal/service/account_ops_test.go`

These files have a mix of validation errors and sentinel/infrastructure errors. Only convert the pure-validation ones.

- [ ] **Step 1: Write failing tests for ValidationError on account operations**

Add to `internal/service/errors_test.go`:

```go
func TestCreateAccount_ValidationErrors(t *testing.T) {
	accRepo := newMockAccountRepo()
	svc := newTestAccountService(accRepo, newMockTransactionRepo())

	tests := []struct {
		name    string
		accName string
		accType model.AccountType
		field   string
	}{
		{"invalid name", "", model.AccountTypeAsset, "name"},
		{"invalid type", "Assets:Cash", model.AccountType("X"), "type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateAccount(context.Background(), tt.accName, tt.accType, "USD", "", nil)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError for %s, got: %T: %v", tt.name, err, err)
		})
	}
}

func TestDeleteAccount_HasChildren_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.childMap[1] = true

	svc := newTestAccountService(accRepo, newMockTransactionRepo())
	err := svc.DeleteAccountByName(context.Background(), "Assets:Bank")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestDeleteAccount_HasTransactions_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.txExistsMap[1] = true

	svc := newTestAccountService(accRepo, newMockTransactionRepo())
	err := svc.DeleteAccountByName(context.Background(), "Assets:Bank")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestRenameAccount_DuplicateName_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Old", Type: model.AccountTypeAsset})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Existing", Type: model.AccountTypeAsset})

	svc := newTestAccountService(accRepo, newMockTransactionRepo())
	err := svc.RenameAccount(context.Background(), "Assets:Old", "Existing")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestCreateAccountWithBalance_NonAL_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	svc := newTestAccountService(accRepo, newMockTransactionRepo())

	_, err := svc.CreateAccountWithBalance(context.Background(), "Revenue:Sales", model.AccountTypeRevenue, "USD", "", nil, 1000)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}
```

Add `"context"` to imports if not already present.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateAccount_ValidationErrors|TestDeleteAccount_Has|TestRenameAccount_Duplicate|TestCreateAccountWithBalance_NonAL" -v`
Expected: FAIL

- [ ] **Step 3: Convert account_ops.go validation errors**

In `internal/service/account_ops.go`, convert these specific errors (leave sentinel-wrapping and infrastructure errors untouched):

Line 90 — invalid account type:
```go
// Before: return fmt.Errorf("invalid account type: %s", accType)
return validationErrorf("type", "invalid account type: %s", accType)
```

Line 84 — invalid account name (wraps inner validation error):
```go
// Before: return fmt.Errorf("invalid account name: %w", err)
return validationWrap("name", "invalid account name", err)
```

Line 87 — invalid currency (wraps inner validation error):
```go
// Before: return fmt.Errorf("invalid currency: %w", err)
return validationWrap("currency", "invalid currency", err)
```

Line 136 — opening balance on non-A/L:
```go
// Before: return fmt.Errorf("only Assets(A) and Liabilities(L) accounts can set an opening balance")
return validationErrorf("type", "only Assets(A) and Liabilities(L) accounts can set an opening balance")
```

Line 188 — has child accounts:
```go
// Before: return fmt.Errorf("account %q has child accounts; delete or move them first", acc.Name)
return validationErrorf("", "account %q has child accounts; delete or move them first", acc.Name)
```

Line 196 — has transactions:
```go
// Before: return fmt.Errorf("account %q has transactions and cannot be deleted", acc.Name)
return validationErrorf("", "account %q has transactions and cannot be deleted", acc.Name)
```

Line 220 — invalid account name in rename (wraps inner validation error):
```go
// Before: return fmt.Errorf("invalid account name: %w", err)
return validationWrap("name", "invalid account name", err)
```

Line 235 — duplicate name in rename:
```go
// Before: return fmt.Errorf("account %q already exists", newFullName)
return validationErrorf("name", "account %q already exists", newFullName)
```

- [ ] **Step 4: Convert account_service.go validation errors**

In `internal/service/account_service.go`, convert these errors in `resolveAndValidateAccount`:

Line 63 — invalid account type:
```go
// Before: return nil, fmt.Errorf("invalid account type '%s' (must be A, L, C, R, E)", acc.Type)
return nil, validationErrorf("type", "invalid account type '%s' (must be A, L, C, R, E)", acc.Type)
```

Line 92 — account is hidden:
```go
// Before: return nil, fmt.Errorf("account %q is hidden", acc.Name)
return nil, validationErrorf("account", "account %q is hidden", acc.Name)
```

Line 100 — parent account:
```go
// Before: return nil, fmt.Errorf("account %q is a parent account; select a leaf account instead", acc.Name)
return nil, validationErrorf("account", "account %q is a parent account; select a leaf account instead", acc.Name)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestCreateAccount|TestDeleteAccount|TestRenameAccount|TestCreateAccountWithBalance" -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_service.go internal/service/errors_test.go
git commit -m "refactor: convert account operation errors to ValidationError (issue #77)"
```

---

### Task 4: Convert transaction_ops.go

**Files:**
- Modify: `internal/service/transaction_ops.go`
- Modify: `internal/service/errors_test.go`

- [ ] **Step 1: Write failing tests for ValidationError on transaction operations**

Add to `internal/service/errors_test.go`:

```go
func TestCreateTransaction_TooFewSplits_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	input := model.TransactionDetail{
		Type:   model.TxTypeExpense,
		Splits: []model.SplitDetail{{AccountName: "A", Amount: 100}},
	}
	_, err := svc.CreateTransaction(context.Background(), input)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "splits", ve.Field)
}

func TestCreateTransaction_EmptyType_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	input := model.TransactionDetail{
		Splits: []model.SplitDetail{
			{AccountName: "A", Amount: 100},
			{AccountName: "B", Amount: -100},
		},
	}
	_, err := svc.CreateTransaction(context.Background(), input)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "type", ve.Field)
}

func TestCreateSimpleTransaction_SameAccount_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.CreateSimpleTransaction(context.Background(), "Assets:Cash", "Assets:Cash", 100, "test", 0, 0, model.TxTypeTransfer)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestCreateSimpleTransaction_NegativeAmount_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.CreateSimpleTransaction(context.Background(), "A", "B", -5, "test", 0, 0, model.TxTypeTransfer)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "amount", ve.Field)
}

func TestUpdateTransactionStatus_InvalidStatus_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	err := svc.UpdateTransactionStatus(context.Background(), 5, model.TransactionStatus(99))
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "status", ve.Field)
}

func TestParseTransactionDate_InvalidFormat_ReturnsValidationError(t *testing.T) {
	txRepo := newMockTransactionRepo()
	accRepo := newMockAccountRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.ParseTransactionDate("not-a-date")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "date", ve.Field)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateTransaction_TooFewSplits|TestCreateTransaction_EmptyType|TestCreateSimpleTransaction_Same|TestCreateSimpleTransaction_Negative|TestUpdateTransactionStatus_Invalid|TestParseTransactionDate_Invalid" -v`
Expected: FAIL

- [ ] **Step 3: Convert transaction_ops.go validation errors**

In `internal/service/transaction_ops.go`, convert these specific errors (leave sentinel-wrapping and infrastructure errors untouched):

Line 29 — too few splits:
```go
return 0, validationErrorf("splits", "transaction must have at least 2 splits (got %d)", len(input.Splits))
```

Line 33 — missing type:
```go
return 0, validationErrorf("type", "transaction type is required")
```

Line 112 — hidden account in `checkAccountSelectable`:
```go
return validationErrorf("account", "account %q is hidden", account.Name)
```

Line 119 — parent account in `checkAccountSelectable`:
```go
return validationErrorf("account", "account %q is a parent account; select a leaf account instead", account.Name)
```

Line 135 — same account:
```go
return model.TransactionDetail{}, validationErrorf("account", "source and destination accounts cannot be the same")
```

Line 138 — non-positive amount:
```go
return model.TransactionDetail{}, validationErrorf("amount", "amount must be positive")
```

Line 229 — invalid status (UpdateTransactionStatus):
```go
return validationErrorf("status", "invalid status: must be 0 (Pending) or 1 (Cleared)")
```

Line 253 — invalid status (UpdateTransactionComplete):
```go
return validationErrorf("status", "invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
```

Line 271 — too few splits (UpdateTransactionComplete):
```go
return validationErrorf("splits", "transaction must have at least 2 splits for double-entry bookkeeping")
```

Line 307 — duplicate split ID:
```go
return validationErrorf("splits", "duplicate split ID %d in input", split.ID)
```

Line 311 — foreign split ID:
```go
return validationErrorf("splits", "split ID %d does not belong to transaction %d", split.ID, txID)
```

Line 399 — invalid date:
```go
return 0, &ValidationError{Field: "date", Message: fmt.Sprintf("invalid date format, use %s: %s", model.DateFormat, err), Err: err}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestCreateTransaction_TooFew|TestCreateTransaction_Empty|TestCreateSimple|TestUpdateTransactionStatus_Invalid|TestParseTransactionDate" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/errors_test.go
git commit -m "refactor: convert transaction operation errors to ValidationError (issue #77)"
```

---

### Task 5: Convert transaction_validation.go and transaction_classifier.go

**Files:**
- Modify: `internal/service/transaction_validation.go`
- Modify: `internal/service/transaction_classifier.go`
- Modify: `internal/service/errors_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/service/errors_test.go`:

```go
func TestValidateSplitsBalance_ReturnsValidationError(t *testing.T) {
	txRepo := newMockTransactionRepo()
	accRepo := newMockAccountRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	splits := []model.Split{
		{AccountID: 1, Amount: 100, Currency: "USD"},
		{AccountID: 2, Amount: -50, Currency: "USD"},
	}
	err := svc.ValidateSplitsBalance(splits)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "splits", ve.Field)
}

func TestValidateSplitsBalance_MixedCurrency_ReturnsValidationError(t *testing.T) {
	txRepo := newMockTransactionRepo()
	accRepo := newMockAccountRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	splits := []model.Split{
		{AccountID: 1, Amount: 100, Currency: "USD"},
		{AccountID: 2, Amount: -100, Currency: "EUR"},
	}
	err := svc.ValidateSplitsBalance(splits)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "splits", ve.Field)
}

func TestValidateSplitsMatchType_BadExpense_ReturnsValidationError(t *testing.T) {
	txRepo := newMockTransactionRepo()
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	svc := newTestTransactionService(accRepo, txRepo)

	splits := []model.SplitDetail{
		{AccountName: "Assets:Cash", AccountID: 1, Amount: 100},
		{AccountName: "Assets:Bank", AccountID: 2, Amount: -100},
	}
	err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeExpense, splits)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestValidateSplitsBalance_ReturnsValidationError|TestValidateSplitsBalance_MixedCurrency|TestValidateSplitsMatchType_Bad" -v`
Expected: FAIL

- [ ] **Step 3: Convert transaction_validation.go errors**

In `internal/service/transaction_validation.go`, convert all `fmt.Errorf` calls:

Line 23 — mixed currency (ValidateSplitsBalance):
```go
return validationErrorf("splits", "splits must all use the same currency (got %q and %q)", baseCurrency, s.Currency)
```

Line 29 — unbalanced (ValidateSplitsBalance):
```go
return validationErrorf("splits", "splits do not balance: total is %d cents (%.2f), must be 0. In double-entry bookkeeping, debits must equal credits", total, float64(total)/100)
```

Line 49 — mixed currency (ValidateSplitDetailsBalance):
```go
return validationErrorf("splits", "splits must all use the same currency (got %q and %q)", baseCurrency, s.Currency)
```

Line 55 — unbalanced (ValidateSplitDetailsBalance):
```go
return validationErrorf("splits", "splits do not balance: total is %d cents (%.2f), must be 0. In double-entry bookkeeping, debits must equal credits", total, float64(total)/100)
```

Line 67 — too few splits (ValidateSplitsMatchType):
```go
return validationErrorf("splits", "transaction must have at least 2 splits")
```

Line 78 — account not found (ValidateSplitsMatchType):
```go
return validationErrorf("splits", "split #%d: account ID %d not found", i+1, sd.AccountID)
```

- [ ] **Step 4: Convert transaction_classifier.go errors**

In `internal/service/transaction_classifier.go`, convert all `fmt.Errorf` calls in `validateSplitTypesForTxType`:

Line 311:
```go
return validationErrorf("type", "expense transaction requires at least one Expense account")
```

Line 314:
```go
return validationErrorf("type", "expense transaction requires at least one Asset or Liability account")
```

Line 332:
```go
return validationErrorf("type", "income transaction requires at least one Revenue account")
```

Line 335:
```go
return validationErrorf("type", "income transaction requires at least one Asset or Liability account")
```

Line 345:
```go
return validationErrorf("type", "transfer transaction must only contain Asset and Liability accounts (found account type %q)", sd.AccountType)
```

Line 350:
```go
return validationErrorf("type", "unknown transaction type %q", txType)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestValidateSplitsBalance|TestValidateSplitsMatchType" -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/service/transaction_validation.go internal/service/transaction_classifier.go internal/service/errors_test.go
git commit -m "refactor: convert transaction validation/classifier errors to ValidationError (issue #77)"
```

---

### Task 6: Convert remaining files (transaction_service, reconcile_ops, report_service)

**Files:**
- Modify: `internal/service/transaction_service.go`
- Modify: `internal/service/reconcile_ops.go`
- Modify: `internal/service/report_service.go`
- Modify: `internal/service/errors_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/service/errors_test.go`:

```go
func TestPreviewReconcile_EmptyTxIDs_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.PreviewReconcile(context.Background(), 1, 0, nil)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "transactions", ve.Field)
}

func TestReconcile_InvalidTxID_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.ReconcileTransactions(context.Background(), 1, 0, []int64{999})
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "transactions", ve.Field)
}

func TestParseMonth_Invalid_ReturnsValidationError(t *testing.T) {
	_, _, _, err := parseMonth("bad")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "month", ve.Field)
}

func TestParseDateRange_Invalid_ReturnsValidationError(t *testing.T) {
	_, _, _, err := parseDateRange("bad", "")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestParseDateRange_EndBeforeStart_ReturnsValidationError(t *testing.T) {
	_, _, _, err := parseDateRange("2026-01-15", "2026-01-01")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestPreviewReconcile_Empty|TestReconcile_InvalidTx|TestParseMonth_Invalid|TestParseDateRange" -v`
Expected: FAIL

- [ ] **Step 3: Convert transaction_service.go**

In `internal/service/transaction_service.go`, line 47:
```go
// Before: return nil, fmt.Errorf("unknown transaction mode: %s", mode)
return nil, validationErrorf("type", "unknown transaction mode: %s", mode)
```

- [ ] **Step 4: Convert reconcile_ops.go validation errors**

Line 41, 92 — no transactions selected:
```go
return 0, validationErrorf("transactions", "no transactions selected for reconciliation")
```

Line 67, 122 — transaction not in unreconciled set:
```go
return 0, validationErrorf("transactions", "transaction ID %d is not in the unreconciled set for this account", id)
```

Leave all `"failed to ..."` infrastructure errors untouched.

- [ ] **Step 5: Convert report_service.go validation errors**

Line 389 — invalid month format:
```go
err = validationErrorf("month", "invalid month format %q, expected YYYY-MM", month)
```

Line 412 — invalid from-date:
```go
err = validationErrorf("from", "invalid from-date format %q, expected YYYY-MM-DD", from)
```

Line 422 — invalid to-date:
```go
err = validationErrorf("to", "invalid to-date format %q, expected YYYY-MM-DD", to)
```

Line 429 — end before start:
```go
err = validationErrorf("to", "end date must be on or after start date")
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestPreviewReconcile_Empty|TestReconcile_InvalidTx|TestParseMonth|TestParseDateRange" -v`
Expected: PASS

- [ ] **Step 7: Run full test suite and build**

Run: `go test ./... && make build`
Expected: ALL PASS, build succeeds

- [ ] **Step 8: Commit**

```bash
git add internal/service/transaction_service.go internal/service/reconcile_ops.go internal/service/report_service.go internal/service/errors_test.go
git commit -m "refactor: convert remaining service errors to ValidationError (issue #77)"
```

---

### Task 7: Final verification and cleanup

- [ ] **Step 1: Verify all validation errors are converted**

Run a grep to find any remaining `fmt.Errorf` in service files that should have been converted:

```bash
grep -n 'fmt\.Errorf' internal/service/*.go | grep -v '_test.go' | grep -v 'failed to' | grep -v '%w'
```

Any remaining hits should either be:
- Infrastructure errors (contain "failed to")
- Sentinel-wrapping errors (contain `%w` with a sentinel)
- False positives that are correctly NOT validation errors

Manually review each hit and convert any missed validation errors.

- [ ] **Step 2: Verify `errors.As` works end-to-end**

Run the full test suite one more time:
```bash
go test ./... -v
```
Expected: ALL PASS

- [ ] **Step 3: Commit any cleanup**

If any missed errors were found and converted in step 1:
```bash
git add internal/service/*.go
git commit -m "refactor: convert remaining missed validation errors (issue #77)"
```

---

## HTTP Status Code Mapping Reference

After this change, the API layer can map errors as follows:

```go
func httpStatusFromError(err error) int {
    var ve *service.ValidationError
    switch {
    case errors.As(err, &ve):
        return http.StatusBadRequest           // 400
    case errors.Is(err, service.ErrNotFound):
        return http.StatusNotFound             // 404
    case errors.Is(err, service.ErrAlreadyExists):
        return http.StatusConflict             // 409
    case errors.Is(err, service.ErrReconciled):
        return http.StatusConflict             // 409
    case errors.Is(err, service.ErrNotEditable):
        return http.StatusForbidden            // 403
    case errors.Is(err, service.ErrCircularParent):
        return http.StatusBadRequest           // 400
    default:
        return http.StatusInternalServerError   // 500
    }
}
```
