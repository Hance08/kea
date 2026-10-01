# Normalize Subaccount Currency Override — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure currency codes are always stored uppercase regardless of which code path creates an account — fixing issue #44.

**Architecture:** Move currency normalization into `AccountService.CreateAccount` so all callers get canonical uppercase codes. Remove the now-redundant normalization from `applyTypeSettings` in the cmd layer. Add validation in `applyParentSettings` (it currently skips `ValidateCurrency`). Add a service-layer test proving lowercase input is stored uppercase.

**Tech Stack:** Go, testify

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/service/account_ops.go` | Modify | Normalize currency in `CreateAccount` |
| `internal/service/account_ops_test.go` | Modify | Add test for lowercase currency normalization |
| `cmd/account/create_actions.go` | Modify | Remove redundant normalization from `applyTypeSettings`, add validation call in `applyParentSettings` |

---

### Task 1: Add service-layer test for lowercase currency normalization

**Files:**
- Modify: `internal/service/account_ops_test.go:170-225` (inside `TestCreateAccount`)

- [ ] **Step 1: Write the failing test**

Add this subtest inside `TestCreateAccount`:

```go
t.Run("lowercase currency is normalized to uppercase", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    acc, err := svc.CreateAccount(context.Background(), "Assets:Bank", model.AccountTypeAsset, "usd", "My bank", nil)
    require.NoError(t, err)
    assert.Equal(t, "USD", acc.Currency)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestCreateAccount/lowercase_currency -v`
Expected: FAIL — `acc.Currency` is `"usd"`, not `"USD"`

- [ ] **Step 3: Commit failing test**

```bash
git add internal/service/account_ops_test.go
git commit -m "test: add test for lowercase currency normalization (issue #44)"
```

---

### Task 2: Normalize currency in `CreateAccount`

**Files:**
- Modify: `internal/service/account_ops.go:49-55`

- [ ] **Step 1: Add normalization before validation**

In `CreateAccount`, normalize the currency string before the `ValidateCurrency` call. Change lines 49-55 from:

```go
func (as *AccountService) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	if err := as.ValidateFullAccountName(name); err != nil {
		return nil, fmt.Errorf("invalid account name: %w", err)
	}
	if err := as.ValidateCurrency(currency); err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}
```

to:

```go
func (as *AccountService) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	if err := as.ValidateFullAccountName(name); err != nil {
		return nil, fmt.Errorf("invalid account name: %w", err)
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := as.ValidateCurrency(currency); err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./internal/service/ -run TestCreateAccount -v`
Expected: ALL subtests PASS, including the new `lowercase_currency` one.

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/service/account_ops.go
git commit -m "fix: normalize currency to uppercase in CreateAccount (issue #44)"
```

---

### Task 3: Clean up redundant cmd-layer normalization

**Files:**
- Modify: `cmd/account/create_actions.go:28-49`

- [ ] **Step 1: Remove redundant normalization from `applyTypeSettings`**

The `applyTypeSettings` method at line 34 does `strings.ToUpper(strings.TrimSpace(currencyOverride))`. Since `CreateAccount` now normalizes, simplify to just assign the value (validation still happens in `CreateAccount`). Change:

```go
func (r *createRunner) applyTypeSettings(accType, currencyOverride string, input *createInput) error {
	input.accountType = model.AccountType(accType)
	if currencyOverride != "" {
		if err := r.accSvc.ValidateCurrency(currencyOverride); err != nil {
			return err
		}
		input.currency = strings.ToUpper(strings.TrimSpace(currencyOverride))
	} else {
		input.currency = r.defaultCurrency
	}
	return nil
}
```

to:

```go
func (r *createRunner) applyTypeSettings(accType, currencyOverride string, input *createInput) error {
	input.accountType = model.AccountType(accType)
	if currencyOverride != "" {
		if err := r.accSvc.ValidateCurrency(currencyOverride); err != nil {
			return err
		}
		input.currency = currencyOverride
	} else {
		input.currency = r.defaultCurrency
	}
	return nil
}
```

- [ ] **Step 2: Add validation call in `applyParentSettings`**

The `applyParentSettings` method at line 41 skips validation entirely for the currency override. Add a validation call and change the signature to return an error. Change:

```go
func (r *createRunner) applyParentSettings(parent *model.Account, currencyOverride string, input *createInput) {
	input.accountType = parent.Type
	input.parentID = &parent.ID
	if currencyOverride != "" {
		input.currency = currencyOverride
	} else {
		input.currency = parent.Currency
	}
}
```

to:

```go
func (r *createRunner) applyParentSettings(parent *model.Account, currencyOverride string, input *createInput) error {
	input.accountType = parent.Type
	input.parentID = &parent.ID
	if currencyOverride != "" {
		if err := r.accSvc.ValidateCurrency(currencyOverride); err != nil {
			return err
		}
		input.currency = currencyOverride
	} else {
		input.currency = parent.Currency
	}
	return nil
}
```

- [ ] **Step 3: Update callers of `applyParentSettings` to handle the new error return**

Find all callers and update them. The caller is `buildFromParentName` at line 51. Change:

```go
func (r *createRunner) buildFromParentName(ctx context.Context, parentName, currency string, input *createInput) error {
	parentAccount, err := r.accSvc.GetAccountByName(ctx, parentName)
	if err != nil {
		return err
	}
	r.applyParentSettings(parentAccount, currency, input)
	input.fullName = parentAccount.Name // prefix for FormatAccountName in runFromFlags
	return nil
}
```

to:

```go
func (r *createRunner) buildFromParentName(ctx context.Context, parentName, currency string, input *createInput) error {
	parentAccount, err := r.accSvc.GetAccountByName(ctx, parentName)
	if err != nil {
		return err
	}
	if err := r.applyParentSettings(parentAccount, currency, input); err != nil {
		return err
	}
	input.fullName = parentAccount.Name // prefix for FormatAccountName in runFromFlags
	return nil
}
```

Also update the caller in the interactive (prompt) path at `cmd/account/create.go:166`. Change:

```go
r.applyParentSettings(parentAccount, parentAccount.Currency, &input)
```

to:

```go
if err := r.applyParentSettings(parentAccount, parentAccount.Currency, &input); err != nil {
    return createInput{}, err
}
```

- [ ] **Step 4: Remove unused `strings` import if needed**

After removing the `strings.ToUpper(strings.TrimSpace(...))` call, check if `strings` is still imported. If no other usage remains, remove the import.

- [ ] **Step 5: Build and run full test suite**

Run: `make build && go test ./...`
Expected: Build succeeds, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/account/create_actions.go
git commit -m "refactor: remove redundant currency normalization from cmd layer"
```
