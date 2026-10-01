# Account Edit Feature Design

**Date:** 2026-04-22
**Branch:** feat/account-edit

## Overview

Add a `kea account edit <account-name>` command that allows editing an account's name segment, description, and hidden status. Renaming cascades to all descendant accounts. Editing accounts with existing transactions is safe because all transactional data references accounts by `account_id` (integer FK), not by name string.

## Editable Fields

| Field | Flag | Notes |
|---|---|---|
| Name segment | `--name` | Last segment only (e.g. `"Savings"`, not `"Assets:Savings"`). Parent prefix is preserved. Cascades to all descendants. |
| Description | `--desc` | Free text. No side effects. |
| Hidden | `--hidden` / `--no-hidden` | Boolean. Toggles `is_hidden` on the account. |

**Excluded fields:** `type` (breaks double-entry rules), `currency` (silently misrepresents historical data), `parent` (restructuring the account tree is a separate, larger concern).

## Command Interface

```
kea account edit <account-name>                         # interactive mode
kea account edit <account-name> --name <new-segment>
kea account edit <account-name> --desc "new description"
kea account edit <account-name> --hidden
kea account edit <account-name> --no-hidden
kea account edit <account-name> --json                  # requires at least one other flag
```

- Positional argument: the account's current full name (e.g. `"Assets:Bank"`)
- Flag mode: applies only the explicitly changed flags, no confirmation prompt
- Interactive mode (no flags): prompts each field with current value pre-filled, shows change summary, requires confirmation
- `--json`: outputs the updated account as JSON (same shape as create's JSON output); requires at least one other flag

## Architecture

### Store layer (`internal/store/sqlite_account.go`)

**Modify `RenameAccount(oldName, newName string) error`** to cascade within a transaction:

```sql
-- 1. Rename the target account
UPDATE accounts SET name = ? WHERE name = ?

-- 2. Cascade to all descendants
UPDATE accounts SET name = replace(name, ?, ?) WHERE name LIKE ? || ':%'
```

Both updates execute inside a single SQLite transaction.

**Add `UpdateAccountMetadata(accountID int64, description string, isHidden bool) error`:**

```sql
UPDATE accounts SET description = ?, is_hidden = ? WHERE id = ?
```

**Add to `AccountRepository` interface** (`internal/repository/interfaces.go`):

```go
UpdateAccountMetadata(accountID int64, description string, isHidden bool) error
```

### Service layer (`internal/service/account_service.go`)

**Modify `RenameAccount(oldName, newName string) error`:**
1. Look up account by `oldName` to get its current full name
2. Derive parent prefix (everything before the last `:`, or empty for root accounts)
3. Validate `newName` segment with `ValidateAccountName`
4. Construct new full name: `parentPrefix + ":" + newName` (or just `newName` for root)
5. Check new full name does not already exist
6. Call store `RenameAccount(oldFullName, newFullName)`

**Add `UpdateAccountMetadata(id int64, description string, isHidden bool) error`:**
- Calls store `UpdateAccountMetadata` directly. No validation needed beyond the account existing.

### Cmd layer

**New files:**

- `cmd/account/edit.go` — cobra command wiring + `editRunner.Run()`
- `cmd/account/edit_types.go` — `editFlags`, `editInput`, `EditProvider` interface, `EditView` interface
- `cmd/account/edit_actions.go` — `runFromFlags`, `runInteractive`, helper methods

**`editFlags`:**
```go
type editFlags struct {
    Name        string
    Desc        string
    Hidden      bool
    NoHidden    bool
    JSON        bool
}
```

**`editInput`** — holds only the fields that actually changed:
```go
type editInput struct {
    newName     *string  // nil = no change
    description *string  // nil = no change
    isHidden    *bool    // nil = no change
}
```

**`EditProvider` interface:**
```go
type EditProvider interface {
    GetAccountByName(name string) (*model.Account, error)
    RenameAccount(oldName, newName string) error
    UpdateAccountMetadata(id int64, description string, isHidden bool) error
    ValidateAccountName(name string) error
    FormatAccountName(prefix, name string) string
}
```

**Flag mode logic:**
- Require at least one of `--name`, `--desc`, `--hidden`, `--no-hidden`
- Error if both `--hidden` and `--no-hidden` are set
- Apply only fields where `cmd.Flags().Changed(...)` is true

**Interactive mode flow:**
1. Fetch current account values
2. Prompt name segment (pre-filled with current segment), skip if unchanged
3. Prompt description (pre-filled with current value), skip if unchanged
4. Prompt hidden toggle (pre-filled with current value)
5. If nothing changed, print info and exit without calling service
6. Show summary of changes, confirm
7. Apply

**Register** in `cmd/account/account.go`:
```go
accountCmd.AddCommand(NewEditCmd(svc))
```

### UI layer

No new TUI component. `EditView` interface:
```go
type EditView interface {
    ShowSuccess(msg string)
}
```

Interactive prompts use existing `prompts` package helpers. Change summary printed with `pterm.Info`.

## Testing

Service-level tests only (white-box, `package service`), consistent with the rest of the codebase.

**Mock additions** in `testhelper_test.go`:
- `renameCalls []struct{ old, new string }` recorder on `mockAccountRepo`
- `updateMetadataCalls []struct{ id int64; desc string; hidden bool }` recorder
- `UpdateAccountMetadata` mock method

**Test cases (`internal/service/account_edit_test.go` or added to `account_ops_test.go`):**

| Test | Description |
|---|---|
| `RenameAccount_leaf` | Renames a leaf account; correct new full name constructed |
| `RenameAccount_cascades` | Mock verifies cascade call is made for parent accounts |
| `RenameAccount_invalidSegment` | Segment containing `:` is rejected |
| `RenameAccount_alreadyExists` | Rejected if new full name already taken |
| `UpdateAccountMetadata_ok` | Description and hidden updated correctly |
| `UpdateAccountMetadata_notFound` | Returns error when account ID not found |

## Constraints & Edge Cases

- **Root account rename:** root accounts (e.g. `Assets`, `Liabilities`) have no parent prefix. Renaming `Assets` → `Assetss` should still cascade to `Assets:Bank`, `Assets:Bank:Checking`, etc.
- **System account:** `Equity:OpeningBalances` must not be editable. Service checks for this and returns an error.
- **No-op edit:** if interactive mode produces no changes, exit cleanly without calling service methods.
- **Cascade atomicity:** both the target rename and descendant rename run in a single SQLite transaction; partial failure rolls back.
