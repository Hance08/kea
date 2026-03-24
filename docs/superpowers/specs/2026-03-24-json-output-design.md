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

Extract the existing pattern from `ui/views/report_json.go` into reusable helpers:

```go
func WriteJSON(v any) error          // indented JSON to stdout
func CentsToUnit(cents int64) float64 // cents → float64 (÷ 100)
```

### `ui/views/json_types.go` (new)

JSON DTO structs with `json` tags. Amounts are `float64` (units, not cents).

```go
type jsonAccount struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    Type        string `json:"type"`
    ParentID    *int64 `json:"parent_id"`
    Currency    string `json:"currency"`
    Description string `json:"description"`
    IsHidden    bool   `json:"is_hidden"`
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
    ID          int64            `json:"id"`
    Date        string           `json:"date"`       // YYYY-MM-DD
    Description string           `json:"description"`
    Status      string           `json:"status"`     // "pending" | "cleared" | "reconciled"
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
    DBExists        bool   `json:"db_exists"`
    DefaultCurrency string `json:"default_currency"`
    AppDataDir      string `json:"app_data_dir"`
}
```

Converter functions (`toJSON*`) live in the same file alongside the structs.

---

## Per-Command Changes

### `account list`
- Add `--json / -j` flag to `listFlags`
- In `Run()`: if JSON → `WriteJSON(toJSONAccounts(accounts))`
- Output: `[]jsonAccount`

### `account create`
- Add `--json / -j` flag to `createFlags`
- Skip interactive confirmation when `--json` is set (scripting mode)
- In `Run()` after account created: if JSON → `WriteJSON(toJSONAccount(acc))`
- Output: `jsonAccount`

### `account delete`
- Add `--json / -j` flag
- `--json` implies `--yes` (skip confirmation)
- Output: `{"name": "...", "deleted": true}`

### `add`
- Add `--json / -j` flag to `addFlags`
- In `Run()` after transaction created: if JSON → `WriteJSON(toJSONTxDetail(detail))`
- Output: `jsonTxDetail`

### `transaction list`
- Add `--json / -j` flag to `listFlags`
- In `Run()`: if JSON → `WriteJSON(toJSONTxListItems(items))`
- Output: `[]jsonTxListItem`

### `transaction show`
- Add `--json / -j` flag
- In `Run()`: if JSON → `WriteJSON(toJSONTxDetail(detail))`
- Output: `jsonTxDetail`

### `transaction delete`
- Add `--json / -j` flag
- `--json` implies `--yes` (skip confirmation)
- Output: `{"id": 42, "deleted": true}`

### `transaction clear`
- Add `--json / -j` flag
- Output: `{"id": 42, "status": "cleared"}`

### `info`
- Add `--json / -j` flag
- In `Run()`: if JSON → `WriteJSON(toJSONSystemInfo(data))`
- Output: `jsonSystemInfo`

---

## Behaviour Rules

1. `--json` suppresses all TUI output (tables, colored messages) — only JSON goes to stdout.
2. For delete commands, `--json` implies `--yes`; no separate flag needed.
3. For `account create`, `--json` skips the final confirmation prompt.
4. All amounts serialized as `float64` units (÷ 100), consistent with `report --json`.
5. Dates serialized as `YYYY-MM-DD` strings.
6. `TransactionStatus` serialized as human-readable string: `"pending"`, `"cleared"`, `"reconciled"`.
7. `AccountType` serialized as single-letter code: `"A"`, `"L"`, `"C"`, `"R"`, `"E"`.

---

## What Is NOT Changing

- `transaction edit` — excluded from this spec (tracked separately in issue #6).
- All interactive flows remain unchanged when `--json` is not set.
- No changes to model layer, service layer, or repository layer.
- No new view interface methods required.
