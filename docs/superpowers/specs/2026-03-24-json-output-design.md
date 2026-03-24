# JSON Output Flag Design

**Date:** 2026-03-24
**Issue:** #5 — add `--json` flag to remaining commands for machine-readable output
**Scope:** All output-producing commands except `transaction edit`

---

## Goal

Enable scripting and automation by adding `--json / -j` to every command that produces output.
Only `report` has this today; this spec brings all remaining commands up to parity.

---

## Shared Infrastructure

### `ui/views/json.go` (new)

Extract and export the pattern already present in `ui/views/report_json.go`:

```go
// WriteJSON encodes v as indented JSON to stdout.
func WriteJSON(v any) error

// CentsToUnit converts int64 cents to float64 units (÷ 100).
func CentsToUnit(cents int64) float64
```

The existing unexported `writeJSON` and `centsToUnit` in `report_json.go` must be removed and replaced with calls to the new shared functions, eliminating the duplication.

### `ui/views/json_types.go` (new)

JSON DTO structs with `json` tags. Amounts are `float64` (units, not cents).
`TransactionStatus` is serialized via its `.String()` method (`"pending"`, `"cleared"`, `"reconciled"`).
`AccountType` is serialized as its single-letter code (`"A"`, `"L"`, `"C"`, `"R"`, `"E"`).

```go
type jsonAccount struct {
    ID          int64   `json:"id"`
    Name        string  `json:"name"`
    Type        string  `json:"type"`
    ParentID    *int64  `json:"parent_id"`
    Currency    string  `json:"currency"`
    Description string  `json:"description"`
    IsHidden    bool    `json:"is_hidden"`
    Balance     float64 `json:"balance"` // via GetAccountBalance → CentsToUnit
}

type jsonSplitDetail struct {
    ID          int64   `json:"id"`
    AccountID   int64   `json:"account_id"`
    AccountName string  `json:"account_name"`
    AccountType string  `json:"account_type"`
    Amount      float64 `json:"amount"`
    Currency    string  `json:"currency"`
    Memo        string  `json:"memo"`
}

type jsonTxDetail struct {
    ID          int64             `json:"id"`
    Date        string            `json:"date"`        // YYYY-MM-DD
    Description string            `json:"description"`
    Status      string            `json:"status"`      // via TransactionStatus.String()
    Splits      []jsonSplitDetail `json:"splits"`
}

type jsonTxListItem struct {
    ID          int64   `json:"id"`
    Date        string  `json:"date"`
    Type        string  `json:"type"`
    Account     string  `json:"account"`
    Offset      string  `json:"offset"`
    Description string  `json:"description"`
    Amount      float64 `json:"amount"`
    Currency    string  `json:"currency"`
    Status      string  `json:"status"`
}

type jsonSystemInfo struct {
    ConfigPath      string `json:"config_path"`
    DBPath          string `json:"db_path"`
    DBExists        bool   `json:"db_exists"`    // false = DB not yet created (created on first use)
    DefaultCurrency string `json:"default_currency"`
    AppDataDir      string `json:"app_data_dir"`
}
```

Converter functions (`toJSONAccount`, `toJSONTxDetail`, etc.) live alongside the structs in the same file.

For `jsonAccount.Balance`: `AccountService` exposes only `GetAccountBalanceFormatted` today. A new method `GetAccountBalance(id int64) (int64, error)` must be added to `AccountService` (delegating to the existing repository method of the same name). The "What Is NOT Changing" constraint about no service-layer changes does not apply to this one targeted addition. Apply `CentsToUnit` to the returned cents value. If the call fails, surface the error.

---

## Per-Command Changes

### `account list`
- Add `--json / -j` flag to `listFlags`
- In `Run()`: if JSON → fetch balances per account via `GetAccountBalance`, build `[]jsonAccount`, call `WriteJSON`
- Output: `[]jsonAccount`

### `account create`
- Add `--json / -j` flag to `createFlags`
- `--json` is only valid in flag mode. If `--json` is set but none of the identifying flags (`--name`, `--type`, `--parent`) are changed, return an error: `--json requires flags (--name, --type, ...)`
- Skip the final confirmation prompt when `--json` is set
- After account created: `WriteJSON(toJSONAccount(acc))`
- Output: `jsonAccount` (balance will be the opening balance provided)

### `account delete`
- Add `--json / -j` flag
- `--json` implies `--yes`: skip the inline `pterm.Info.Printf` account-info line and the confirmation prompt (all pterm output before deletion is suppressed)
- Output: `{"name": "Expenses:Food", "deleted": true}`

### `add`
- Add `--json / -j` flag to `addFlags`
- `--json` is only valid in flag mode. If `--json` is set but no identifying flags (`--desc`, `--amount`, `--from`, `--to`) are changed, return an error: `--json requires flags (--desc, --amount, --from, --to)`
- After transaction created: `WriteJSON(toJSONTxDetail(detail))`
- Output: `jsonTxDetail`

### `transaction list`
- Add `--json / -j` flag to `listFlags`
- In `Run()`: if JSON → build `[]jsonTxListItem` from the same data used for the table, call `WriteJSON`
- Output: `[]jsonTxListItem`

### `transaction show`
- Add `--json / -j` flag to `showRunner`
- In `Run()`: if JSON → `WriteJSON(toJSONTxDetail(detail))`
- Output: `jsonTxDetail`

### `transaction delete`
- Add `--json / -j` flag to `deleteRunner`
- `--json` implies `--yes`: skip both the preview render and the confirmation prompt
- Output: `{"id": 42, "deleted": true}`

### `transaction clear`
- Add `--json / -j` flag to `clearRunner`
- No confirmation prompt exists; `--json` only affects output format
- The status value is hardcoded as `"Cleared"` (matching `model.StatusCleared.String()`) based on the command's known effect; no re-fetch required
- Output: `{"id": 42, "status": "Cleared"}`

### `info`
- Add `--json / -j` flag to `infoRunner`
- In `Run()`: if JSON → `WriteJSON(toJSONSystemInfo(data))`
- Output: `jsonSystemInfo`

---

## Behaviour Rules

1. `--json` suppresses all TUI output (tables, colored pterm messages) — only JSON goes to stdout.
2. For delete commands (`account delete`, `transaction delete`), `--json` implies `--yes`: both the preview render and the confirmation prompt are suppressed.
3. For `account create` and `add`, `--json` requires flag mode. If no identifying flags are provided, the command returns a non-zero exit code with a descriptive error message instead of entering interactive mode.
4. All amounts serialized as `float64` units (÷ 100), consistent with `report --json`.
5. Dates serialized as `YYYY-MM-DD` strings.
6. `TransactionStatus` serialized via `.String()`: `"Pending"`, `"Cleared"`, `"Reconciled"`.
7. `AccountType` serialized as single-letter code: `"A"`, `"L"`, `"C"`, `"R"`, `"E"`.
8. `transaction clear` has no confirmation prompt; `--json` on this command only changes output format, not flow.

---

## What Is NOT Changing

- `transaction edit` — excluded from this spec (tracked separately in issue #6).
- All interactive flows remain unchanged when `--json` is not set.
- No changes to model layer or repository layer.
- One targeted addition to service layer: `AccountService.GetAccountBalance(id int64) (int64, error)`.
- No new view interface methods required.
- `WriteJSON` and `CentsToUnit` in `ui/views/json.go` are exported (uppercase). The private equivalents in `report_json.go` are removed and replaced with calls to the new shared functions.
- `jsonTxListItem` does not reuse `TransactionListItem` (which stores `Amount` as a formatted string); it maps from it with `Amount` as `float64`.
