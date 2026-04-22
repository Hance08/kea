# Account Edit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `kea account edit <account-name>` command that edits an account's name segment (with cascading rename to all descendants), description, and hidden status.

**Architecture:** Service layer handles name construction and validation; store handles the cascading SQL rename in a transaction; cmd layer follows the existing create/delete pattern with both flag and interactive modes.

**Tech Stack:** Go, cobra, charmbracelet/huh (prompts), pterm (output), SQLite (store)

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Modify | `internal/repository/interfaces.go` | Add `UpdateAccountMetadata` to `AccountRepository` |
| Modify | `internal/service/testhelper_test.go` | Add mock method + recorders |
| Modify | `internal/service/account_ops_test.go` | New tests for rename and metadata update |
| Modify | `internal/service/account_service.go` | New `UpdateAccountMetadata` service method |
| Modify | `internal/service/account_ops.go` | Updated `RenameAccount` service logic |
| Modify | `internal/store/sqlite_account.go` | Updated `RenameAccount` (cascade) + new `UpdateAccountMetadata` |
| Create | `cmd/account/edit_types.go` | `editFlags`, `editInput`, `EditProvider`, `EditView` |
| Create | `cmd/account/edit_actions.go` | `runFromFlags`, `runInteractive`, helpers |
| Create | `cmd/account/edit.go` | Cobra command + `editRunner.Run` |
| Modify | `cmd/account/account.go` | Register `NewEditCmd` |

---

## Task 1: Extend repository interface and update mock

**Files:**
- Modify: `internal/repository/interfaces.go`
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Add `UpdateAccountMetadata` to the `AccountRepository` interface**

In `internal/repository/interfaces.go`, add the new method after `RenameAccount`:

```go
type AccountRepository interface {
	CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts() ([]*model.Account, error)
	GetAccountByName(name string) (*model.Account, error)
	GetAccountByID(id int64) (*model.Account, error)
	AccountExists(name string) (bool, error)
	GetAccountsByType(accType model.AccountType) ([]*model.Account, error)
	GetAccountBalance(accountID int64) (int64, error)
	GetAllAccountBalances(asOf int64) (map[int64]int64, error)
	HasChildAccounts(accountID int64) (bool, error)
	AccountHasTransactions(accountID int64) (bool, error)
	DeleteAccount(accountID int64) error
	RenameAccount(oldName, newName string) error
	UpdateAccountMetadata(accountID int64, description string, isHidden bool) error
}
```

- [ ] **Step 2: Add recorders and `UpdateAccountMetadata` to `mockAccountRepo`**

In `internal/service/testhelper_test.go`, add these fields to the `mockAccountRepo` struct (after `txExistsMap`):

```go
	// call recorders
	renameCalls          []struct{ old, new string }
	updateMetadataCalls  []struct {
		id          int64
		description string
		isHidden    bool
	}
	updateMetadataErr error
```

Replace the existing `RenameAccount` mock method with a version that records calls:

```go
func (m *mockAccountRepo) RenameAccount(oldName, newName string) error {
	m.renameCalls = append(m.renameCalls, struct{ old, new string }{oldName, newName})
	acc, ok := m.accountsByName[oldName]
	if !ok {
		return fmt.Errorf("account %q not found", oldName)
	}
	delete(m.accountsByName, oldName)
	acc.Name = newName
	m.accountsByName[newName] = acc
	return nil
}
```

Add the new mock method after `RenameAccount`:

```go
func (m *mockAccountRepo) UpdateAccountMetadata(accountID int64, description string, isHidden bool) error {
	if m.updateMetadataErr != nil {
		return m.updateMetadataErr
	}
	acc, ok := m.accountsByID[accountID]
	if !ok {
		return fmt.Errorf("account ID %d not found", accountID)
	}
	m.updateMetadataCalls = append(m.updateMetadataCalls, struct {
		id          int64
		description string
		isHidden    bool
	}{accountID, description, isHidden})
	acc.Description = description
	acc.IsHidden = isHidden
	return nil
}
```

- [ ] **Step 3: Verify the mock still compiles**

```bash
go build ./internal/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/repository/interfaces.go internal/service/testhelper_test.go
git commit -m "feat(repo): add UpdateAccountMetadata to AccountRepository interface and mock"
```

---

## Task 2: Service tests for `RenameAccount` (updated behavior)

**Files:**
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Add failing tests for `RenameAccount`**

Add the following test block at the end of `internal/service/account_ops_test.go`:

```go
// ──────────────────────────────────────────────
// RenameAccount
// ──────────────────────────────────────────────

func TestRenameAccount(t *testing.T) {
	t.Run("leaf account renamed with correct full name constructed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "Savings")
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, "Assets:Bank", accRepo.renameCalls[0].old)
		assert.Equal(t, "Assets:Savings", accRepo.renameCalls[0].new)
	})

	t.Run("repo called with old full name and new full name (not bare segment)", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank:Checking", "Current")
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, "Assets:Bank:Checking", accRepo.renameCalls[0].old)
		assert.Equal(t, "Assets:Bank:Current", accRepo.renameCalls[0].new)
	})

	t.Run("segment containing colon rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "Bad:Name")
		require.Error(t, err)
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("empty segment rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "")
		require.Error(t, err)
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("new full name already exists is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Savings", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "Savings")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("system account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount(model.OpeningBalancesAccountName("USD"), "Other")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.RenameAccount("Assets:Ghost", "NewName")
		require.Error(t, err)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/service/ -run TestRenameAccount -v
```

Expected: FAIL — `RenameAccount` doesn't construct full names or validate yet.

---

## Task 3: Update `RenameAccount` in the service layer

**Files:**
- Modify: `internal/service/account_ops.go`

- [ ] **Step 1: Replace the existing `RenameAccount` method**

In `internal/service/account_ops.go`, replace:

```go
func (as *AccountService) RenameAccount(oldName, newName string) error {
	return as.repo.RenameAccount(oldName, newName)
}
```

With:

```go
func (as *AccountService) RenameAccount(oldName, newSegment string) error {
	acc, err := as.repo.GetAccountByName(oldName)
	if err != nil {
		return err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	if err := as.ValidateAccountName(newSegment); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	var newFullName string
	if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
		newFullName = acc.Name[:idx+1] + newSegment
	} else {
		newFullName = newSegment
	}

	if err := as.ValidateFullAccountName(newFullName); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	exists, err := as.repo.AccountExists(newFullName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("account %q already exists", newFullName)
	}

	return as.repo.RenameAccount(acc.Name, newFullName)
}
```

Make sure `"strings"` is imported in `account_ops.go`. The file already imports it — verify with:

```bash
head -15 internal/service/account_ops.go
```

If `"strings"` is missing, add it to the import block.

- [ ] **Step 2: Run the rename tests**

```bash
go test ./internal/service/ -run TestRenameAccount -v
```

Expected: all PASS.

- [ ] **Step 3: Run full service suite to check for regressions**

```bash
go test ./internal/service/...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "feat(service): update RenameAccount to validate segment and construct full name"
```

---

## Task 4: Update `RenameAccount` in the store (cascade)

**Files:**
- Modify: `internal/store/sqlite_account.go`

- [ ] **Step 1: Replace `RenameAccount` in the store with a cascading version**

In `internal/store/sqlite_account.go`, replace the existing `RenameAccount` function with:

```go
// RenameAccount updates the name of an account and cascades the rename to all descendants.
// Both updates run in a single transaction.
func (s *Store) RenameAccount(oldName, newName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`UPDATE accounts SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return fmt.Errorf("failed to rename account %q: %w", oldName, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account %q not found", oldName)
	}

	_, err = tx.Exec(
		`UPDATE accounts SET name = replace(name, ?, ?) WHERE name LIKE ? || ':%'`,
		oldName, newName, oldName,
	)
	if err != nil {
		return fmt.Errorf("failed to cascade rename from %q to %q: %w", oldName, newName, err)
	}

	return tx.Commit()
}
```

- [ ] **Step 2: Build the store to verify it compiles**

```bash
go build ./internal/store/...
```

Expected: no errors.

- [ ] **Step 3: Run the full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite_account.go
git commit -m "feat(store): cascade rename to all descendant accounts in a single transaction"
```

---

## Task 5: Service tests for `UpdateAccountMetadata`

**Files:**
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Add failing tests for `UpdateAccountMetadata`**

Append to `internal/service/account_ops_test.go`:

```go
// ──────────────────────────────────────────────
// UpdateAccountMetadata
// ──────────────────────────────────────────────

func TestUpdateAccountMetadata(t *testing.T) {
	t.Run("description and hidden updated correctly", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Description: "old desc", IsHidden: false})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.UpdateAccountMetadata(1, "new desc", true)
		require.NoError(t, err)

		require.Len(t, accRepo.updateMetadataCalls, 1)
		call := accRepo.updateMetadataCalls[0]
		assert.Equal(t, int64(1), call.id)
		assert.Equal(t, "new desc", call.description)
		assert.True(t, call.isHidden)
	})

	t.Run("system account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 99, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.UpdateAccountMetadata(99, "desc", false)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
		assert.Empty(t, accRepo.updateMetadataCalls)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.UpdateAccountMetadata(999, "desc", false)
		require.Error(t, err)
	})

	t.Run("repo error propagated", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		accRepo.updateMetadataErr = errors.New("db error")
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.UpdateAccountMetadata(1, "desc", false)
		require.Error(t, err)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/service/ -run TestUpdateAccountMetadata -v
```

Expected: FAIL — `UpdateAccountMetadata` does not exist yet.

---

## Task 6: Implement `UpdateAccountMetadata` in store and service

**Files:**
- Modify: `internal/store/sqlite_account.go`
- Modify: `internal/service/account_service.go`

- [ ] **Step 1: Add `UpdateAccountMetadata` to the store**

Append to `internal/store/sqlite_account.go`:

```go
// UpdateAccountMetadata updates the description and is_hidden flag for an account.
func (s *Store) UpdateAccountMetadata(accountID int64, description string, isHidden bool) error {
	res, err := s.db.Exec(
		`UPDATE accounts SET description = ?, is_hidden = ? WHERE id = ?`,
		description, isHidden, accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to update account metadata for ID %d: %w", accountID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %d not found", accountID)
	}
	return nil
}
```

- [ ] **Step 2: Add `UpdateAccountMetadata` to `AccountService`**

Append to `internal/service/account_service.go`:

```go
func (as *AccountService) UpdateAccountMetadata(accountID int64, description string, isHidden bool) error {
	acc, err := as.repo.GetAccountByID(accountID)
	if err != nil {
		return err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	return as.repo.UpdateAccountMetadata(accountID, description, isHidden)
}
```

- [ ] **Step 3: Run metadata tests**

```bash
go test ./internal/service/ -run TestUpdateAccountMetadata -v
```

Expected: all PASS.

- [ ] **Step 4: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite_account.go internal/service/account_service.go internal/service/account_ops_test.go
git commit -m "feat(service,store): add UpdateAccountMetadata for description and hidden status"
```

---

## Task 7: Cmd edit types

**Files:**
- Create: `cmd/account/edit_types.go`

- [ ] **Step 1: Create `edit_types.go`**

Create `cmd/account/edit_types.go` with the following content:

```go
package account

import "github.com/hance08/kea/internal/model"

// EditProvider is the service interface required by the edit command.
type EditProvider interface {
	GetAccountByName(name string) (*model.Account, error)
	GetAccountBalance(id int64) (int64, error)
	RenameAccount(oldName, newSegment string) error
	UpdateAccountMetadata(id int64, description string, isHidden bool) error
	ValidateAccountName(name string) error
	CheckAccountExists(name string) (bool, error)
}

// EditView is the display interface required by the edit command.
type EditView interface {
	ShowSuccess(msg string)
}

type editFlags struct {
	Name     string
	Desc     string
	Hidden   bool
	NoHidden bool
	JSON     bool
}

// editInput carries only the fields that should change. nil pointer = no change.
type editInput struct {
	newName     *string
	description *string
	isHidden    *bool
}

type editRunner struct {
	svc  EditProvider
	view EditView
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./cmd/...
```

Expected: no errors.

---

## Task 8: Cmd edit actions

**Files:**
- Create: `cmd/account/edit_actions.go`

- [ ] **Step 1: Create `edit_actions.go`**

Create `cmd/account/edit_actions.go`:

```go
package account

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func (r *editRunner) runFromFlags(acc *model.Account, flags *editFlags, cmd *cobra.Command) (editInput, error) {
	var input editInput

	if flags.Hidden && flags.NoHidden {
		return editInput{}, fmt.Errorf("--hidden and --no-hidden cannot both be set")
	}

	if cmd.Flags().Changed("name") {
		if err := r.svc.ValidateAccountName(flags.Name); err != nil {
			return editInput{}, fmt.Errorf("invalid name: %w", err)
		}
		input.newName = &flags.Name
	}

	if cmd.Flags().Changed("desc") {
		input.description = &flags.Desc
	}

	if cmd.Flags().Changed("hidden") {
		v := true
		input.isHidden = &v
	}

	if cmd.Flags().Changed("no-hidden") {
		v := false
		input.isHidden = &v
	}

	return input, nil
}

func (r *editRunner) runInteractive(acc *model.Account) (editInput, error) {
	var input editInput

	currentSegment := acc.Name
	if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
		currentSegment = acc.Name[idx+1:]
	}

	newSegment, err := r.promptNameSegment(currentSegment, acc.Name)
	if err != nil {
		return editInput{}, err
	}
	if newSegment != currentSegment {
		input.newName = &newSegment
	}

	newDesc, err := prompts.PromptInput(
		fmt.Sprintf("Description (current: %q):", acc.Description),
		acc.Description,
		nil,
	)
	if err != nil {
		return editInput{}, err
	}
	if newDesc != acc.Description {
		input.description = &newDesc
	}

	newHidden, err := prompts.PromptConfirm(
		fmt.Sprintf("Hide account? (current: %v)", acc.IsHidden),
		acc.IsHidden,
	)
	if err != nil {
		return editInput{}, err
	}
	if newHidden != acc.IsHidden {
		input.isHidden = &newHidden
	}

	if input.newName == nil && input.description == nil && input.isHidden == nil {
		return editInput{}, nil
	}

	r.showChangeSummary(acc, input)

	confirmed, err := prompts.PromptConfirm("Apply these changes?", true)
	if err != nil {
		return editInput{}, err
	}
	if !confirmed {
		return editInput{}, fmt.Errorf("edit cancelled")
	}

	return input, nil
}

func (r *editRunner) promptNameSegment(currentSegment, currentFullName string) (string, error) {
	prefix := ""
	if idx := strings.LastIndex(currentFullName, ":"); idx >= 0 {
		prefix = currentFullName[:idx]
	}

	validator := func(s string) error {
		if s == currentSegment {
			return nil
		}
		if err := r.svc.ValidateAccountName(s); err != nil {
			return err
		}
		var newFullName string
		if prefix != "" {
			newFullName = prefix + ":" + s
		} else {
			newFullName = s
		}
		exists, err := r.svc.CheckAccountExists(newFullName)
		if err != nil {
			return fmt.Errorf("failed to check existence: %w", err)
		}
		if exists {
			return fmt.Errorf("account %q already exists", newFullName)
		}
		return nil
	}

	return prompts.PromptInput(
		fmt.Sprintf("Name segment (prefix: %s):", currentFullName),
		currentSegment,
		validator,
	)
}

func (r *editRunner) showChangeSummary(acc *model.Account, input editInput) {
	if input.newName != nil {
		newFullName := *input.newName
		if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
			newFullName = acc.Name[:idx+1] + *input.newName
		}
		pterm.Info.Printf("Name:        %s → %s\n", acc.Name, newFullName)
	}
	if input.description != nil {
		pterm.Info.Printf("Description: %q → %q\n", acc.Description, *input.description)
	}
	if input.isHidden != nil {
		pterm.Info.Printf("Hidden:      %v → %v\n", acc.IsHidden, *input.isHidden)
	}
}

func (r *editRunner) applyChanges(acc *model.Account, input editInput) (string, error) {
	finalName := acc.Name

	if input.newName != nil {
		if err := r.svc.RenameAccount(acc.Name, *input.newName); err != nil {
			return "", fmt.Errorf("failed to rename account: %w", err)
		}
		if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
			finalName = acc.Name[:idx+1] + *input.newName
		} else {
			finalName = *input.newName
		}
	}

	if input.description != nil || input.isHidden != nil {
		desc := acc.Description
		hidden := acc.IsHidden
		if input.description != nil {
			desc = *input.description
		}
		if input.isHidden != nil {
			hidden = *input.isHidden
		}
		if err := r.svc.UpdateAccountMetadata(acc.ID, desc, hidden); err != nil {
			return "", fmt.Errorf("failed to update account: %w", err)
		}
	}

	return finalName, nil
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./cmd/...
```

Expected: no errors (edit.go does not exist yet — ignore that; this step checks for syntax errors in actions).

---

## Task 9: Cmd edit command and register

**Files:**
- Create: `cmd/account/edit.go`
- Modify: `cmd/account/account.go`

- [ ] **Step 1: Create `edit.go`**

Create `cmd/account/edit.go`:

```go
package account

import (
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func NewEditCmd(svc *service.Service) *cobra.Command {
	flags := &editFlags{}

	cmd := &cobra.Command{
		Use:     "edit <account-name>",
		Aliases: []string{"e"},
		Short:   "Edit an account's name, description, or hidden status.",
		Long: `Edit an account's name segment, description, or hidden status.

When renaming, only the last segment of the name changes — the parent path is preserved.
Renaming a parent account cascades to all its descendants automatically.

Example:
  kea account edit Assets:Bank --name Savings
  kea account edit Assets:Bank --desc "Main savings account"
  kea account edit Assets:Bank --hidden
  kea account edit Assets:Bank --no-hidden`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &editRunner{
				svc:  svc.Account(),
				view: views.NewAccountCreateView(),
			}
			return runner.Run(args[0], flags, cmd)
		},
	}

	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "New name segment (last part only)")
	cmd.Flags().StringVarP(&flags.Desc, "desc", "d", "", "New description")
	cmd.Flags().BoolVar(&flags.Hidden, "hidden", false, "Hide the account")
	cmd.Flags().BoolVar(&flags.NoHidden, "no-hidden", false, "Unhide the account")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "Output updated account as JSON")

	return cmd
}

func (r *editRunner) Run(accName string, flags *editFlags, cmd *cobra.Command) error {
	acc, err := r.svc.GetAccountByName(accName)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	hasFlags := cmd.Flags().Changed("name") || cmd.Flags().Changed("desc") ||
		cmd.Flags().Changed("hidden") || cmd.Flags().Changed("no-hidden")

	if flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires at least one of: --name, --desc, --hidden, --no-hidden")
	}

	var input editInput
	if hasFlags {
		input, err = r.runFromFlags(acc, flags, cmd)
	} else {
		input, err = r.runInteractive(acc)
	}
	if err != nil {
		return err
	}

	if input.newName == nil && input.description == nil && input.isHidden == nil {
		pterm.Info.Println("No changes made")
		return nil
	}

	finalName, err := r.applyChanges(acc, input)
	if err != nil {
		return err
	}

	if flags.JSON {
		updatedAcc, err := r.svc.GetAccountByName(finalName)
		if err != nil {
			return err
		}
		bal, err := r.svc.GetAccountBalance(updatedAcc.ID)
		if err != nil {
			return err
		}
		return views.WriteJSON(views.ToJSONAccount(updatedAcc, bal))
	}

	r.view.ShowSuccess("Account updated successfully")
	return nil
}
```

- [ ] **Step 2: Register `NewEditCmd` in `account.go`**

In `cmd/account/account.go`, add `NewEditCmd` after `NewCreateCmd`:

```go
func NewAccountCmd(svc *service.Service) *cobra.Command {
	accountCmd := &cobra.Command{
		Use:     "account",
		Aliases: []string{"ac"},
		Short:   "It can create, edit, delete account and show the list of all accounts.",
		Long:    `It can create, edit, delete account and show the list of all accounts.`,
	}

	accountCmd.AddCommand(NewCreateCmd(svc))
	accountCmd.AddCommand(NewEditCmd(svc))
	accountCmd.AddCommand(NewListCmd(svc))
	accountCmd.AddCommand(NewDeleteCmd(svc))

	return accountCmd
}
```

- [ ] **Step 3: Build everything**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Smoke test the command help**

```bash
go run ./cmd/kea account edit --help
```

Expected output includes:
```
Edit an account's name segment, description, or hidden status.
...
Usage:
  kea account edit <account-name> [flags]
...
Flags:
  -d, --desc string   New description
      --hidden        Hide the account
  -j, --json          Output updated account as JSON
  -n, --name string   New name segment (last part only)
      --no-hidden     Unhide the account
```

- [ ] **Step 6: Commit**

```bash
git add cmd/account/edit.go cmd/account/edit_types.go cmd/account/edit_actions.go cmd/account/account.go
git commit -m "feat(cmd): add account edit command with flag and interactive modes"
```
