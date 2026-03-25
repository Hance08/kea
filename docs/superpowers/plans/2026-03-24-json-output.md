# JSON Output Flag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--json / -j` flag to 9 commands so they emit machine-readable JSON output for scripting and automation.

**Architecture:** A shared `WriteJSON` helper and JSON DTO structs live in `ui/views/`. Each command branches on the `JSON` flag: text path unchanged, JSON path calls `WriteJSON(toJSONXxx(...))`. One service method is added to expose raw balance cents.

**Tech Stack:** Go, Cobra, `encoding/json`, existing `model` and `service` packages, testify for assertions.

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `ui/views/json.go` | Exported `WriteJSON` + `CentsToUnit` helpers |
| Create | `ui/views/json_types.go` | JSON DTOs + converter functions |
| Create | `ui/views/json_test.go` | Unit tests for DTOs and converters |
| Modify | `ui/views/report_json.go` | Remove private `writeJSON`/`centsToUnit`; call shared versions |
| Modify | `internal/service/account_service.go` | Add `GetAccountBalance(id int64) (int64, error)` |
| Modify | `internal/service/account_ops_test.go` | Test `GetAccountBalance` |
| Modify | `cmd/account/list.go` | Add `--json` flag |
| Modify | `cmd/account/create.go` | Add `--json` flag; guard interactive mode |
| Modify | `cmd/account/create_types.go` | Add `JSON bool` to `createFlags` |
| Modify | `cmd/account/delete.go` | Add `--json` flag; suppress TUI when set |
| Modify | `cmd/add.go` | Add `--json` flag |
| Modify | `cmd/add_types.go` | Add `JSON bool` to `addFlags` |
| Modify | `cmd/transaction/list.go` | Add `--json` flag |
| Modify | `cmd/transaction/show.go` | Add `--json` flag |
| Modify | `cmd/transaction/delete.go` | Add `--json` flag |
| Modify | `cmd/transaction/clear.go` | Add `--json` flag |
| Modify | `cmd/info.go` | Add `--json` flag |

---

## Task 1: Shared JSON infrastructure

**Files:**
- Create: `ui/views/json.go`
- Create: `ui/views/json_types.go`
- Create: `ui/views/json_test.go`
- Modify: `ui/views/report_json.go`

- [ ] **Step 1: Write failing tests for helpers and converters**

Create `ui/views/json_test.go`:

```go
package views

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCentsToUnit(t *testing.T) {
	assert.Equal(t, 1.0, CentsToUnit(100))
	assert.Equal(t, 1.5, CentsToUnit(150))
	assert.Equal(t, 0.0, CentsToUnit(0))
	assert.Equal(t, -1.5, CentsToUnit(-150))
}

func TestToJSONAccount(t *testing.T) {
	parentID := int64(1)
	acc := &model.Account{
		ID:          2,
		Name:        "Assets:Bank",
		Type:        model.AccountTypeAsset,
		ParentID:    &parentID,
		Currency:    "TWD",
		Description: "Main bank",
		IsHidden:    false,
	}
	got := ToJSONAccount(acc, 15000)
	assert.Equal(t, int64(2), got.ID)
	assert.Equal(t, "Assets:Bank", got.Name)
	assert.Equal(t, "A", got.Type)
	assert.Equal(t, &parentID, got.ParentID)
	assert.Equal(t, "TWD", got.Currency)
	assert.Equal(t, "Main bank", got.Description)
	assert.False(t, got.IsHidden)
	assert.Equal(t, 150.0, got.Balance)
}

func TestToJSONTxDetail(t *testing.T) {
	detail := &model.TransactionDetail{
		ID:          42,
		Timestamp:   1711238400, // 2024-03-24
		Description: "Buy coffee",
		Status:      model.StatusCleared,
		Splits: []model.SplitDetail{
			{ID: 1, AccountID: 10, AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -500, Currency: "TWD", Memo: ""},
			{ID: 2, AccountID: 20, AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "TWD", Memo: "lunch"},
		},
	}
	got := ToJSONTxDetail(detail)
	assert.Equal(t, int64(42), got.ID)
	assert.Equal(t, "Cleared", got.Status)
	assert.Len(t, got.Splits, 2)
	assert.Equal(t, -5.0, got.Splits[0].Amount)
	assert.Equal(t, 5.0, got.Splits[1].Amount)
	assert.Equal(t, "lunch", got.Splits[1].Memo)
}

func TestToJSONTxListItem(t *testing.T) {
	item := TransactionListItem{
		ID: 7, Date: "2024-03-24", Type: "Expense",
		Account: "Assets:Cash", Offset: "Expenses:Food",
		Description: "lunch", Amount: "5.00", Currency: "TWD", Status: "Cleared",
	}
	got := ToJSONTxListItem(item)
	assert.Equal(t, int64(7), got.ID)
	assert.Equal(t, 5.0, got.Amount)
	assert.Equal(t, "TWD", got.Currency)
}

func TestWriteJSON_validOutput(t *testing.T) {
	// Redirect stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := WriteJSON(map[string]string{"key": "value"})
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "value", result["key"])
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./ui/views/... -run "TestCentsToUnit|TestToJSON|TestWriteJSON" -v
```

Expected: compilation error (functions not defined yet).

- [ ] **Step 3: Create `ui/views/json.go`**

```go
package views

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hance08/kea/internal/model"
)

// WriteJSON encodes v as indented JSON to stdout.
func WriteJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// CentsToUnit converts int64 cents to float64 currency units (÷ model.CentsPerUnit).
func CentsToUnit(cents int64) float64 {
	return float64(cents) / float64(model.CentsPerUnit)
}
```

- [ ] **Step 4: Create `ui/views/json_types.go`**

All DTO structs and converter functions are **exported** (uppercase first letter) so they can be used from `cmd/` packages.

```go
package views

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
)

// ── JSON DTOs ─────────────────────────────────────────────────────────────────
// Amounts are float64 currency units (not cents). Dates are YYYY-MM-DD strings.
// TransactionStatus is serialized via .String(). AccountType as single-letter code.

type JSONAccount struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	ParentID    *int64  `json:"parent_id"`
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	IsHidden    bool    `json:"is_hidden"`
	Balance     float64 `json:"balance"`
}

type JSONSplitDetail struct {
	ID          int64   `json:"id"`
	AccountID   int64   `json:"account_id"`
	AccountName string  `json:"account_name"`
	AccountType string  `json:"account_type"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Memo        string  `json:"memo"`
}

type JSONTxDetail struct {
	ID          int64             `json:"id"`
	Date        string            `json:"date"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Splits      []JSONSplitDetail `json:"splits"`
}

type JSONTxListItem struct {
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

type JSONSystemInfo struct {
	ConfigPath      string `json:"config_path"`
	DBPath          string `json:"db_path"`
	// false means DB does not yet exist; it will be created on first use.
	DBExists        bool   `json:"db_exists"`
	DefaultCurrency string `json:"default_currency"`
	AppDataDir      string `json:"app_data_dir"`
}

// ── Converters ────────────────────────────────────────────────────────────────

func ToJSONAccount(acc *model.Account, balanceCents int64) JSONAccount {
	return JSONAccount{
		ID:          acc.ID,
		Name:        acc.Name,
		Type:        string(acc.Type),
		ParentID:    acc.ParentID,
		Currency:    acc.Currency,
		Description: acc.Description,
		IsHidden:    acc.IsHidden,
		Balance:     CentsToUnit(balanceCents),
	}
}

func toJSONSplit(s model.SplitDetail) JSONSplitDetail {
	return JSONSplitDetail{
		ID:          s.ID,
		AccountID:   s.AccountID,
		AccountName: s.AccountName,
		AccountType: string(s.AccountType),
		Amount:      CentsToUnit(s.Amount),
		Currency:    s.Currency,
		Memo:        s.Memo,
	}
}

func ToJSONTxDetail(d *model.TransactionDetail) JSONTxDetail {
	splits := make([]JSONSplitDetail, len(d.Splits))
	for i, s := range d.Splits {
		splits[i] = toJSONSplit(s)
	}
	return JSONTxDetail{
		ID:          d.ID,
		Date:        time.Unix(d.Timestamp, 0).UTC().Format("2006-01-02"),
		Description: d.Description,
		Status:      d.Status.String(),
		Splits:      splits,
	}
}

func ToJSONTxListItem(item TransactionListItem) JSONTxListItem {
	var amount float64
	fmt.Sscanf(item.Amount, "%f", &amount)
	return JSONTxListItem{
		ID:          item.ID,
		Date:        item.Date,
		Type:        item.Type,
		Account:     item.Account,
		Offset:      item.Offset,
		Description: item.Description,
		Amount:      amount,
		Currency:    item.Currency,
		Status:      item.Status,
	}
}

func ToJSONSystemInfo(info SystemInfo) JSONSystemInfo {
	return JSONSystemInfo{
		ConfigPath:      info.ConfigPath,
		DBPath:          info.DBPath,
		DBExists:        info.DBExists,
		DefaultCurrency: info.DefaultCurrency,
		AppDataDir:      info.AppDataDir,
	}
}
```

- [ ] **Step 5: Update `ui/views/report_json.go` — remove private duplicates**

Replace the private `writeJSON` and `centsToUnit` functions with calls to the shared exported versions. Find these two functions in `report_json.go` and delete them. Then update every call site in the same file:
- `writeJSON(...)` → `WriteJSON(...)`
- `centsToUnit(...)` → `CentsToUnit(...)`

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./ui/views/... -run "TestCentsToUnit|TestToJSON|TestWriteJSON" -v
```

Expected: all PASS.

- [ ] **Step 7: Run full test suite to confirm no regressions**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add ui/views/json.go ui/views/json_types.go ui/views/json_test.go ui/views/report_json.go
git commit -m "feat: add shared WriteJSON/CentsToUnit helpers and JSON DTOs"
```

---

## Task 2: AccountService.GetAccountBalance

**Files:**
- Modify: `internal/service/account_service.go`
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/service/account_ops_test.go`:

```go
func TestGetAccountBalance(t *testing.T) {
	repo := newMockAccountRepo()
	svc := newTestAccountService(repo, newMockTransactionRepo())

	repo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset})
	repo.balances[1] = 5000 // 50.00 in cents

	t.Run("returns raw cents", func(t *testing.T) {
		got, err := svc.GetAccountBalance(1)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), got)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo.getBalanceErr[99] = errors.New("not found")
		_, err := svc.GetAccountBalance(99)
		assert.Error(t, err)
	})
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/service/... -run TestGetAccountBalance -v
```

Expected: FAIL — `svc.GetAccountBalance undefined`.

- [ ] **Step 3: Add method to AccountService**

In `internal/service/account_service.go`, add after `GetAccountBalanceFormatted`:

```go
func (as *AccountService) GetAccountBalance(accountID int64) (int64, error) {
	return as.repo.GetAccountBalance(accountID)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/service/... -run TestGetAccountBalance -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_service.go internal/service/account_ops_test.go
git commit -m "feat: expose GetAccountBalance on AccountService"
```

---

## Task 3: account list --json

**Files:**
- Modify: `cmd/account/list.go`

- [ ] **Step 1: Add JSON flag and branch**

In `cmd/account/list.go`:

1. Add `JSON bool` to `listFlags`:
```go
type listFlags struct {
	Type       string
	ShowHidden bool
	JSON       bool
}
```

2. Register the flag in `NewListCmd` before `return cmd`:
```go
cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")
```

3. In `Run()`, replace the view render call with a branch. Note: all DTO structs and converters are exported (Task 1 Step 4), so they are accessible from this package:
```go
func (r *listRunner) Run() error {
	// ... existing fetch and filter logic unchanged ...

	if r.flags.JSON {
		items := make([]views.JSONAccount, 0, len(accounts))
		for _, acc := range accounts {
			bal, err := r.svc.Account().GetAccountBalance(acc.ID)
			if err != nil {
				return fmt.Errorf("failed to get balance for %s: %w", acc.Name, err)
			}
			items = append(items, views.ToJSONAccount(acc, bal))
		}
		return views.WriteJSON(items)
	}

	return views.NewAccountListView().Render(accounts, r.svc.Account().GetAccountBalanceFormatted)
}
```

- [ ] **Step 2: Verify build and run**

```bash
go build ./...
```

Expected: compiles cleanly.

- [ ] **Step 3: Manual smoke test**

```bash
./kea_test account list --json
```

Expected: valid JSON array of account objects with `balance` as float.

- [ ] **Step 4: Commit**

```bash
git add cmd/account/list.go ui/views/json_types.go ui/views/json_test.go
git commit -m "feat: add --json to account list"
```

---

## Task 4: account create --json

**Files:**
- Modify: `cmd/account/create_types.go`
- Modify: `cmd/account/create.go`

- [ ] **Step 1: Add `GetAccountBalance` to `CreateProvider` interface**

In `cmd/account/create_types.go`, add the method to `CreateProvider`:

```go
type CreateProvider interface {
	// ... all existing methods unchanged ...
	GetAccountBalance(id int64) (int64, error)
}
```

- [ ] **Step 2: Add JSON field to createFlags**

In `cmd/account/create_types.go`, add `JSON bool` to `createFlags`:

```go
type createFlags struct {
	Name        string
	Type        string
	Parent      string
	BalanceStr  string
	Currency    string
	Description string
	JSON        bool
}
```

- [ ] **Step 3: Register flag and guard interactive mode**

In `cmd/account/create.go`:

1. In `NewCreateCmd`, add flag registration before `return cmd`:
```go
cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output created account as JSON")
```

2. In `Run()`, add a guard after the `hasFlags` check:
```go
func (r *createRunner) Run(flags *createFlags, cmd *cobra.Command) error {
	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("parent")

	if flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires flags: --name and one of --type or --parent")
	}

	if hasFlags {
		err := r.runFromFlags(flags)
		// ... existing error handling (ErrAccountExists check) unchanged ...
		return nil
	}
	// interactive path unchanged
	return r.runInteractive()
}
```

3. In `runFromFlags`, replace `r.view.ShowSuccess(...)` with a JSON/text branch:
```go
	newAccount, err := r.createAccount()
	if err != nil {
		return err
	}

	if flags.JSON {
		bal, err := r.accSvc.GetAccountBalance(newAccount.ID)
		if err != nil {
			return err
		}
		return views.WriteJSON(views.ToJSONAccount(newAccount, bal))
	}

	r.view.ShowSuccess(fmt.Sprintf("Account create successfully ! (ID: %d)", newAccount.ID))
	return nil
```

- [ ] **Step 4: Verify build**

```bash
go build ./...
```

- [ ] **Step 5: Manual smoke test**

```bash
./kea_test account create --name TestJSON --type E --json
```

Expected: JSON object with the created account.

- [ ] **Step 6: Commit**

```bash
git add cmd/account/create_types.go cmd/account/create.go
git commit -m "feat: add --json to account create"
```

---

## Task 5: account delete --json

**Files:**
- Modify: `cmd/account/delete.go`

- [ ] **Step 1: Add JSON flag and suppress TUI**

In `cmd/account/delete.go`:

1. Add `json bool` to `deleteRunner`:
```go
type deleteRunner struct {
	svc  *service.Service
	yes  bool
	json bool
}
```

2. In `NewDeleteCmd`, add flag and pass to runner:
```go
var jsonOut bool
cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output result as JSON")
// in RunE:
runner := &deleteRunner{svc: svc, yes: yes || jsonOut, json: jsonOut}
```

Note: `yes: yes || jsonOut` ensures `--json` implies skip-confirmation.

3. In `Run()`, suppress the pterm info line and confirmation when `json` is true, and emit JSON on success:
```go
func (r *deleteRunner) Run(name string) error {
	acc, err := r.svc.Account().GetAccountByName(name)
	if err != nil {
		pterm.Error.Printf("Failed to delete account: %v\n", err)
		return nil
	}

	if !r.json {
		pterm.Info.Printf("Account: %s | Type: %s | Currency: %s | Hidden: %t\n",
			acc.Name, acc.Type, acc.Currency, acc.IsHidden)
	}

	if !r.yes {
		confirm, err := prompts.PromptConfirm("This will permanently delete the account. Continue?", false)
		if err != nil {
			return err
		}
		if !confirm {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}

	if err := r.svc.Account().DeleteAccountByName(acc.Name); err != nil {
		pterm.Error.Printf("Failed to delete account: %v\n", err)
		return nil
	}

	if r.json {
		return views.WriteJSON(map[string]any{"name": acc.Name, "deleted": true})
	}
	pterm.Success.Printf("Account %q deleted\n", acc.Name)
	return nil
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/account/delete.go
git commit -m "feat: add --json to account delete"
```

---

## Task 6: add --json

**Files:**
- Modify: `cmd/add_types.go`
- Modify: `cmd/add.go`

- [ ] **Step 1: Add JSON field to addFlags**

In `cmd/add_types.go`, add `JSON bool` to `addFlags`:

```go
type addFlags struct {
	Description string
	Amount      string
	From        string
	To          string
	Status      string
	Timestamp   string
	Type        string
	JSON        bool
}
```

- [ ] **Step 2: Register flag**

In `cmd/add.go` `NewAddCmd`, add before `return cmd`:
```go
cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output created transaction as JSON")
```

- [ ] **Step 3: Guard interactive mode and branch output**

In `cmd/add.go` `Run()`, add the `--json` guard and output branch. The actual `Run()` calls `r.txSvc.CreateSimpleTransaction(...)` inline — preserve that call exactly and add the JSON branch after it:

```go
func (r *addRunner) Run() error {
	hasFlags := r.cmd.Flags().Changed("desc") || r.cmd.Flags().Changed("amount") ||
		r.cmd.Flags().Changed("from") || r.cmd.Flags().Changed("to") ||
		r.cmd.Flags().Changed("type")

	if r.flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires flags: --desc, --amount, --from, --to")
	}

	var input addTransactionInput
	var err error
	if hasFlags {
		input, err = r.runFromFlags()
	} else {
		input, err = r.runInteractive()
	}
	if err != nil {
		return err
	}

	result, err := r.txSvc.CreateSimpleTransaction(
		input.FromAccountID,
		input.ToAccountID,
		input.AmountCents,
		input.Description,
		input.Timestamp,
		input.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	if r.flags.JSON {
		return views.WriteJSON(views.ToJSONTxDetail(&result))
	}
	return r.addView.Render(&result, true)
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/add_types.go cmd/add.go
git commit -m "feat: add --json to add command"
```

---

## Task 7: transaction list --json

**Files:**
- Modify: `cmd/transaction/list.go`

- [ ] **Step 1: Add JSON flag and branch**

In `cmd/transaction/list.go`:

1. Add `JSON bool` to `listFlags`.
2. Register flag in `NewListCmd`.
3. In `Run()`, after `buildViewItems`, add branch:

```go
func (r *listRunner) Run() error {
	transactions, err := r.fetchTransactions()
	if err != nil {
		return err
	}

	viewItems := r.buildViewItems(transactions)

	if r.flags.JSON {
		jsonItems := make([]views.JSONTxListItem, len(viewItems))
		for i, item := range viewItems {
			jsonItems[i] = views.ToJSONTxListItem(item)
		}
		return views.WriteJSON(jsonItems)
	}

	return r.view.Render(viewItems, r.flags.Limit)
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/transaction/list.go
git commit -m "feat: add --json to transaction list"
```

---

## Task 8: transaction show --json

**Files:**
- Modify: `cmd/transaction/show.go`

- [ ] **Step 1: Add JSON flag and branch**

In `cmd/transaction/show.go`:

1. Add `json bool` to `showRunner`.
2. In `NewShowCmd`, turn the command into a two-step setup:

```go
func NewShowCmd(svc *service.Service) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show transaction details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &showRunner{
				svc:  svc.Transaction(),
				view: views.NewTransactionDetailView(),
				json: jsonOut,
			}
			return runner.Run(args)
		},
	}

	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output as JSON")
	return cmd
}
```

3. In `Run()`, add branch after fetching `detail`:

```go
	if r.json {
		return views.WriteJSON(views.ToJSONTxDetail(detail))
	}
	return r.view.Render(detail, false)
```

- [ ] **Step 2: Verify build and commit**

```bash
go build ./...
git add cmd/transaction/show.go
git commit -m "feat: add --json to transaction show"
```

---

## Task 9: transaction delete --json

**Files:**
- Modify: `cmd/transaction/delete.go`

- [ ] **Step 1: Add JSON flag**

In `cmd/transaction/delete.go`:

1. Add `json bool` to `deleteRunner`.
2. In `NewDeleteCmd`, add flag and set `yes: yes || jsonOut` in runner construction (same pattern as account delete).
3. In `Run()`, when `r.json` is set: skip `RenderPreview` and confirmation, and emit JSON on success:

```go
func (r *deleteRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.Transaction().GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to delete transaction: %v\n", err)
		return nil
	}

	if !r.json {
		if err := r.view.RenderPreview(views.TransactionDeletePreview{
			ID: detail.ID, Timestamp: detail.Timestamp,
			Description: detail.Description, SplitCount: len(detail.Splits),
		}); err != nil {
			return err
		}
	}

	if !r.yes {
		confirmation, err := prompts.PromptConfirm("Do you want to delete this transaction?", false)
		if err != nil {
			return err
		}
		if !confirmation {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}

	if err := r.svc.Transaction().DeleteTransaction(txID); err != nil {
		pterm.Error.Printf("Failed to delete transaction: %v\n", err)
		return nil
	}

	if r.json {
		return views.WriteJSON(map[string]any{"id": txID, "deleted": true})
	}
	r.view.ShowSuccess(txID)
	return nil
}
```

- [ ] **Step 2: Verify build and commit**

```bash
go build ./...
git add cmd/transaction/delete.go
git commit -m "feat: add --json to transaction delete"
```

---

## Task 10: transaction clear --json

**Files:**
- Modify: `cmd/transaction/clear.go`

- [ ] **Step 1: Add JSON flag and branch**

In `cmd/transaction/clear.go`:

1. Add `json bool` to `clearRunner`.
2. In `NewClearCmd`, convert to two-step setup with flag:

```go
func NewClearCmd(svc *service.Service) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Mark transaction as cleared",
		Long:  `Mark a pending transaction as cleared (confirmed).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &clearRunner{svc: svc, json: jsonOut}
			return runner.Run(args)
		},
	}

	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output result as JSON")
	return cmd
}
```

3. In `Run()`, add branch after successful update:

```go
	if err := r.svc.Transaction().UpdateTransactionStatus(txID, 1); err != nil {
		pterm.Error.Printf("Failed to update transaction status: %v\n", err)
		return nil
	}

	if r.json {
		return views.WriteJSON(map[string]any{"id": txID, "status": model.StatusCleared.String()})
	}
	pterm.Success.Printf("Transaction (ID: %d) marked as cleared\n", txID)
	return nil
```

Add import for `model` package to `clear.go`.

- [ ] **Step 2: Verify build and commit**

```bash
go build ./...
git add cmd/transaction/clear.go
git commit -m "feat: add --json to transaction clear"
```

---

## Task 11: info --json

**Files:**
- Modify: `cmd/info.go`

- [ ] **Step 1: Add JSON flag and branch**

In `cmd/info.go`:

1. Add `json bool` to `infoRunner`.
2. In `NewInfoCmd`, convert to two-step setup:

```go
func NewInfoCmd(svc *service.Service) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &infoRunner{svc: svc, view: views.NewSystemInfoView(), json: jsonOut}
			return runner.Run()
		},
	}

	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output as JSON")
	return cmd
}
```

3. In `Run()`, add branch at the end:

```go
	if r.json {
		return views.WriteJSON(views.ToJSONSystemInfo(info))
	}
	return r.view.Render(info)
```

- [ ] **Step 2: Verify full build and test suite**

```bash
go build ./...
go test ./...
```

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/info.go
git commit -m "feat: add --json to info command"
```

---

## Final Verification

- [ ] **Build the binary**

```bash
make build
```

- [ ] **Smoke test each command**

```bash
./kea_test account list --json
./kea_test account create --name SmokeTest --type E --json
./kea_test account delete SmokeTest --json
./kea_test add --desc "Test" --amount 10 --from "Assets:Cash" --to "Expenses:Food" --json
./kea_test transaction list --json
./kea_test transaction show 2 --json
./kea_test transaction clear 2 --json
./kea_test transaction delete 2 --json
./kea_test info --json
```

- [ ] **Verify --json errors when no flags on guarded commands**

```bash
./kea_test account create --json   # should error
./kea_test add --json              # should error
```

- [ ] **Run full test suite**

```bash
go test ./...
```

Expected: all PASS.
