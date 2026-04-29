# cmd/ Style Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify design patterns across all files under `cmd/` so that every command consistently uses flags structs, minimal provider interfaces, and proper error propagation.

**Architecture:** Each task targets one or two files and makes an isolated, compile-safe change. No new features are added; only structure and wiring change. All changes must keep `go build ./...` and `go test ./...` passing.

**Tech Stack:** Go, Cobra, pterm, internal service/model packages.

---

## File Map

| File | Change |
|------|--------|
| `cmd/report.go` | rename local var `r` → `runner`; return error from `RunE` instead of swallowing |
| `cmd/add_types.go` | remove `modeUIConfigs` var |
| `cmd/add_actions.go` | move `modeUIConfigs` here (it is config data used by actions, not a type) |
| `cmd/account/delete.go` | extract `deleteFlags` struct; define `AccountDeleteProvider` interface; inject `svc.Account()` instead of full `*service.Service`; return errors instead of swallowing with pterm |
| `cmd/account/list.go` | define `AccountListProvider` interface; inject `svc.Account()` instead of full `*service.Service` |
| `cmd/transaction/delete.go` | extract `deleteFlags` struct; inject `svc.Transaction()` via `TransactionDeleteProvider` instead of full `*service.Service`; return errors instead of swallowing |
| `cmd/transaction/show.go` | extract `showFlags` struct; return errors instead of swallowing with pterm |
| `cmd/transaction/clear.go` | extract `clearFlags` struct; define `ClearProvider` interface; inject `svc.Transaction()` instead of full `*service.Service`; return errors instead of swallowing |
| `cmd/info.go` | extract `infoFlags` struct; define `InfoProvider` interface; inject `svc` via interface instead of `*service.Service` |
| `cmd/account/create.go` | refactor `createRunner` to remove mutable state fields; introduce `createInput` value type; `runFromFlags` and `runInteractive` return `(createInput, error)` |
| `cmd/account/create_actions.go` | update all action methods to accept/return `createInput` instead of mutating runner fields |

---

## Task 1: Fix `report.go` local variable name and error propagation

**Files:**
- Modify: `cmd/report.go`

Runner variable is named `r` which clashes with the receiver convention used everywhere else. Error from `r.run()` is swallowed inside `RunE` via pterm — errors should propagate so root.go handles display consistently.

- [ ] **Step 1: Rename `r` to `runner` and return error**

In `cmd/report.go`, change `RunE`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    runner := &reportRunner{
        flags:    flags,
        provider: svc.Transaction(),
        view:     newReportView(flags),
    }
    return runner.run()
},
```

- [ ] **Step 2: Remove the now-unused `pterm` import**

Remove `"github.com/pterm/pterm"` from the import block. (It was only used for the swallowed error path.)

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./...
```

Expected: compiles, tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/report.go
git commit -m "refactor: propagate report error through RunE; rename runner var"
```

---

## Task 2: Move `modeUIConfigs` out of `add_types.go`

**Files:**
- Modify: `cmd/add_types.go`
- Modify: `cmd/add_actions.go`

`_types.go` files should only contain interfaces, flag structs, and input structs. `modeUIConfigs` is a runtime config map used by action logic — it belongs in `add_actions.go`.

- [ ] **Step 1: Move the var to `add_actions.go`**

Add to the bottom of `cmd/add_actions.go`:

```go
var modeUIConfigs = map[model.TransactionType]struct{ Src, Dst string }{
	model.ModeExpense:  {"Payment Source:", "Expense Type:"},
	model.ModeIncome:   {"Revenue Type:", "Deposit To:"},
	model.ModeTransfer: {"From Account:", "To Account:"},
}
```

- [ ] **Step 2: Remove the var from `add_types.go`**

Delete the `modeUIConfigs` declaration from `cmd/add_types.go`.

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/add_types.go cmd/add_actions.go
git commit -m "refactor: move modeUIConfigs from add_types to add_actions"
```

---

## Task 3: Standardize `account/delete.go`

**Files:**
- Modify: `cmd/account/delete.go`

Problems: (1) runner holds full `*service.Service` instead of minimal interface; (2) flags are inline `var` declarations instead of a struct; (3) errors are swallowed with pterm instead of propagated.

- [ ] **Step 1: Define `AccountDeleteProvider` interface and `deleteFlags` struct**

Replace the top of `cmd/account/delete.go` with:

```go
type AccountDeleteProvider interface {
	GetAccountByName(name string) (*model.Account, error)
	DeleteAccountByName(name string) error
}

type deleteFlags struct {
	Yes     bool
	JSONOut bool
}

type deleteRunner struct {
	svc  AccountDeleteProvider
	view deleteView
	yes  bool
	json bool
}

type deleteView interface {
	ShowInfo(format string, a ...any)
	ShowSuccess(format string, a ...any)
}
```

Wait — `account/delete.go` currently uses pterm directly for ShowInfo/ShowSuccess. For this simple command, rather than adding a full view interface, we'll keep pterm for success output but propagate errors. The runner struct just needs the minimal service interface.

Replace the struct and `NewDeleteCmd`:

```go
type AccountDeleteProvider interface {
	GetAccountByName(name string) (*model.Account, error)
	DeleteAccountByName(name string) error
}

type deleteFlags struct {
	Yes     bool
	JSONOut bool
}

type deleteRunner struct {
	svc  AccountDeleteProvider
	yes  bool
	json bool
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}

	cmd := &cobra.Command{
		Use:     "delete <account-name>",
		Aliases: []string{"del", "d"},
		Short:   "Delete an account with no transactions",
		Long:    "Delete an account that has no transactions, no child accounts, and is not the system opening balance account.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &deleteRunner{svc: svc.Account(), yes: flags.Yes || flags.JSONOut, json: flags.JSONOut}
			return runner.Run(args[0])
		},
	}

	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output result as JSON")

	return cmd
}
```

- [ ] **Step 2: Update `Run` to propagate errors**

```go
func (r *deleteRunner) Run(name string) error {
	acc, err := r.svc.GetAccountByName(name)
	if err != nil {
		return fmt.Errorf("failed to find account: %w", err)
	}

	if !r.json {
		pterm.Info.Printf("Account: %s | Type: %s | Currency: %s | Hidden: %t\n", acc.Name, acc.Type, acc.Currency, acc.IsHidden)
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

	if err := r.svc.DeleteAccountByName(acc.Name); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	if r.json {
		return views.WriteJSON(map[string]any{"name": acc.Name, "deleted": true})
	}
	pterm.Success.Printf("Account %q deleted\n", acc.Name)
	return nil
}
```

- [ ] **Step 3: Remove unused `*service.Service` import if no longer needed**

Check imports; remove `"github.com/hance08/kea/internal/service"` only if it's now unused. (It is still needed by `NewDeleteCmd` parameter type.)

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/account/delete.go
git commit -m "refactor: account delete — flags struct, provider interface, propagate errors"
```

---

## Task 4: Standardize `account/list.go`

**Files:**
- Modify: `cmd/account/list.go`

Problem: runner holds full `*service.Service`; should hold a minimal `AccountListProvider` interface and inject `svc.Account()`.

- [ ] **Step 1: Define `AccountListProvider` interface and update runner**

```go
type AccountListProvider interface {
	GetAccountsByType(accType model.AccountType) ([]*model.Account, error)
	GetAllAccounts() ([]*model.Account, error)
	GetAccountBalance(id int64) (int64, error)
	GetAccountBalanceFormatted(id int64) (string, error)
}

type listRunner struct {
	svc   AccountListProvider
	flags *listFlags
}
```

- [ ] **Step 2: Update `NewListCmd` to inject `svc.Account()`**

```go
RunE: func(cmd *cobra.Command, args []string) error {
    runner := &listRunner{
        svc:   svc.Account(),
        flags: flags,
    }
    return runner.Run()
},
```

- [ ] **Step 3: Update `Run` and `filterHiddenAccounts` to use new interface**

All calls already go through the interface methods — just update receiver field accesses from `r.svc.Account().X` to `r.svc.X`:

```go
func (r *listRunner) Run() error {
	var accounts []*model.Account
	var err error

	if r.flags.Type != "" {
		accounts, err = r.svc.GetAccountsByType(model.AccountType(r.flags.Type))
	} else {
		accounts, err = r.svc.GetAllAccounts()
	}
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	if !r.flags.ShowHidden {
		accounts = r.filterHiddenAccounts(accounts)
	}

	if r.flags.JSON {
		items := make([]views.JSONAccount, 0, len(accounts))
		for _, acc := range accounts {
			bal, err := r.svc.GetAccountBalance(acc.ID)
			if err != nil {
				return fmt.Errorf("failed to get balance for %s: %w", acc.Name, err)
			}
			items = append(items, views.ToJSONAccount(acc, bal))
		}
		return views.WriteJSON(items)
	}
	return views.NewAccountListView().Render(accounts, r.svc.GetAccountBalanceFormatted)
}
```

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/account/list.go
git commit -m "refactor: account list — inject AccountListProvider instead of full service"
```

---

## Task 5: Standardize `transaction/delete.go`

**Files:**
- Modify: `cmd/transaction/delete.go`

Problems: (1) flags are inline `var`; (2) runner holds full `*service.Service`; (3) errors swallowed via pterm.

- [ ] **Step 1: Extract `deleteFlags`, define `TxDeleteProvider`, update runner**

```go
type TxDeleteProvider interface {
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
	DeleteTransaction(txID int64) error
}

type deleteFlags struct {
	Yes     bool
	JSONOut bool
}

type deleteRunner struct {
	svc  TxDeleteProvider
	view TransactionDeleteView
	yes  bool
	json bool
}
```

- [ ] **Step 2: Update `NewDeleteCmd`**

```go
func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}

	cmd := &cobra.Command{
		Use:     "delete <transaction-id>",
		Short:   "Delete a transaction",
		Long:    `Delete a transaction and all its associated splits. This action cannot be undone.`,
		Aliases: []string{"del", "d"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &deleteRunner{
				svc:  svc.Transaction(),
				view: views.NewTransactionDeleteView(),
				yes:  flags.Yes || flags.JSONOut,
				json: flags.JSONOut,
			}
			return runner.Run(args)
		},
	}

	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output result as JSON")

	return cmd
}
```

- [ ] **Step 3: Update `Run` to propagate errors**

```go
func (r *deleteRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if !r.json {
		if err := r.view.RenderPreview(views.TransactionDeletePreview{
			ID:          detail.ID,
			Timestamp:   detail.Timestamp,
			Description: detail.Description,
			SplitCount:  len(detail.Splits),
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

	if err := r.svc.DeleteTransaction(txID); err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	if r.json {
		return views.WriteJSON(map[string]any{"id": txID, "deleted": true})
	}
	r.view.ShowSuccess(txID)
	return nil
}
```

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/transaction/delete.go
git commit -m "refactor: tx delete — flags struct, provider interface, propagate errors"
```

---

## Task 6: Standardize `transaction/show.go` and `transaction/clear.go`

**Files:**
- Modify: `cmd/transaction/show.go`
- Modify: `cmd/transaction/clear.go`

`show.go` already uses a provider interface — just add a `showFlags` struct and propagate errors.
`clear.go` holds full `*service.Service` — add `clearFlags` struct, `ClearProvider` interface, propagate errors.

### show.go

- [ ] **Step 1: Extract `showFlags` and update `NewShowCmd`**

```go
type showFlags struct {
	JSONOut bool
}

func NewShowCmd(svc *service.Service) *cobra.Command {
	flags := &showFlags{}
	cmd := &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show transaction details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &showRunner{
				svc:     svc.Transaction(),
				view:    views.NewTransactionDetailView(),
				jsonOut: flags.JSONOut,
			}
			return runner.Run(args)
		},
	}
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output as JSON")
	return cmd
}
```

- [ ] **Step 2: Update `Run` in `show.go` to propagate errors**

```go
func (r *showRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if r.jsonOut {
		return views.WriteJSON(views.ToJSONTxDetail(detail))
	}
	return r.view.Render(detail, false)
}
```

Remove pterm import from `show.go`.

### clear.go

- [ ] **Step 3: Define `ClearProvider`, extract `clearFlags`, update `NewClearCmd`**

```go
type ClearProvider interface {
	UpdateTransactionStatus(txID int64, status int) error
}

type clearFlags struct {
	JSONOut bool
}

type clearRunner struct {
	svc  ClearProvider
	json bool
}

func NewClearCmd(svc *service.Service) *cobra.Command {
	flags := &clearFlags{}
	cmd := &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Mark transaction as cleared",
		Long:  `Mark a pending transaction as cleared (confirmed).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &clearRunner{svc: svc.Transaction(), json: flags.JSONOut}
			return runner.Run(args)
		},
	}
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output result as JSON")
	return cmd
}
```

- [ ] **Step 4: Update `Run` in `clear.go` to propagate errors**

```go
func (r *clearRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	if err := r.svc.UpdateTransactionStatus(txID, 1); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	if r.json {
		return views.WriteJSON(map[string]any{"id": txID, "status": model.StatusCleared.String()})
	}
	pterm.Success.Printf("Transaction (ID: %d) marked as cleared\n", txID)
	return nil
}
```

Remove pterm from `show.go` only (clear.go still uses it for success output).

- [ ] **Step 5: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 6: Commit**

```bash
git add cmd/transaction/show.go cmd/transaction/clear.go
git commit -m "refactor: tx show/clear — flags struct, provider interface, propagate errors"
```

---

## Task 7: Standardize `info.go`

**Files:**
- Modify: `cmd/info.go`

Problem: runner holds full `*service.Service` just to call `svc.Config()`. Should use a minimal interface. Flags should use a struct.

- [ ] **Step 1: Define `InfoProvider` interface and `infoFlags` struct**

```go
type InfoProvider interface {
	Config() *config.Config
}

type infoFlags struct {
	JSONOut bool
}
```

Add import `"github.com/hance08/kea/internal/config"` if not present.

- [ ] **Step 2: Update runner and `NewInfoCmd`**

```go
type infoRunner struct {
	svc  InfoProvider
	view SystemInfoView
	json bool
}

func NewInfoCmd(svc *service.Service) *cobra.Command {
	flags := &infoFlags{}
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &infoRunner{svc: svc, view: views.NewSystemInfoView(), json: flags.JSONOut}
			return runner.Run()
		},
	}
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output as JSON")
	return cmd
}
```

Note: `*service.Service` already satisfies `InfoProvider` if it has a `Config() *config.Config` method — no change needed at the call site.

- [ ] **Step 3: Update `Run` to use interface**

Replace `r.svc.Config()` calls — they stay the same since the interface is satisfied by `*service.Service`. Just update the field types.

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/info.go
git commit -m "refactor: info — flags struct and InfoProvider interface"
```

---

## Task 8: Refactor `createRunner` to remove mutable state

**Files:**
- Modify: `cmd/account/create.go`
- Modify: `cmd/account/create_actions.go`
- Modify: `cmd/account/create_types.go`

This is the most complex task. The `createRunner` currently stores intermediate state (`name`, `fullName`, `parentID`, `accountType`, `currency`, `balanceCents`, `description`, `defaultCurrency`) that accumulates across prompt steps. The runner should only hold dependencies.

The fix: introduce a `createInput` value type that is assembled and returned by `runFromFlags` / `runInteractive`, then passed to `createAccount`.

- [ ] **Step 1: Add `createInput` to `create_types.go`**

```go
type createInput struct {
	fullName     string
	accountType  model.AccountType
	currency     string
	description  string
	parentID     *int64
	balanceCents int64
}
```

- [ ] **Step 2: Update `createRunner` in `create.go` — remove state fields**

```go
type createRunner struct {
	defaultCurrency string
	accSvc          CreateProvider
	view            CreateView
}
```

- [ ] **Step 3: Update `Run` in `create.go` to use `createInput`**

```go
func (r *createRunner) Run(flags *createFlags, cmd *cobra.Command) error {
	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("parent") || cmd.Flags().Changed("balance") ||
		cmd.Flags().Changed("currency") || cmd.Flags().Changed("desc")

	if flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires flags: --name and one of --type or --parent")
	}

	var input createInput
	var err error

	if hasFlags {
		input, err = r.runFromFlags(flags)
	} else {
		input, err = r.runInteractive()
	}
	if err != nil {
		if errors.Is(err, store.ErrAccountExists) {
			if flags.JSON {
				return err
			}
			pterm.Error.Println("Account already exists")
			return nil
		}
		return err
	}

	newAccount, err := r.createAccount(input)
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
	r.view.ShowSuccess(fmt.Sprintf("Account created successfully (ID: %d)", newAccount.ID))
	return nil
}
```

(Also fix the typo: "create successfully !" → "created successfully".)

- [ ] **Step 4: Update `runFromFlags` in `create.go` to return `(createInput, error)`**

```go
func (r *createRunner) runFromFlags(flags *createFlags) (createInput, error) {
	if flags.Parent == "" && flags.Type == "" {
		return createInput{}, fmt.Errorf("must enter at least one of --type or --parent flag")
	}
	if flags.Parent != "" && flags.Type != "" {
		return createInput{}, fmt.Errorf("--type and --parent flags cannot be used at the same time")
	}

	if err := r.accSvc.ValidateAccountName(flags.Name); err != nil {
		return createInput{}, fmt.Errorf("invalid account name: %w", err)
	}

	var input createInput
	input.description = flags.Description

	if flags.Parent != "" {
		if err := r.buildFromParentName(flags.Parent, flags.Currency, &input); err != nil {
			return createInput{}, err
		}
	} else {
		if err := r.buildFromTypeFlag(flags.Type, flags.Currency, &input); err != nil {
			return createInput{}, err
		}
	}

	input.fullName = r.accSvc.FormatAccountName(input.fullName, flags.Name)
	if err := r.accSvc.ValidateFullAccountName(input.fullName); err != nil {
		return createInput{}, fmt.Errorf("validate account name: %w", err)
	}

	balanceCents, err := utils.ParseAmount(flags.BalanceStr)
	if err != nil {
		return createInput{}, fmt.Errorf("invalid balance format '%s': please enter a number (e.g. 100 or 100.50)", flags.BalanceStr)
	}
	input.balanceCents = balanceCents

	return input, nil
}
```

- [ ] **Step 5: Update `runInteractive` in `create.go` to return `(createInput, error)`**

```go
func (r *createRunner) runInteractive() (createInput, error) {
	var input createInput

	isSubAccount, err := prompts.PromptIsSubAccount()
	if err != nil {
		return createInput{}, err
	}

	if isSubAccount {
		parentAccount, err := r.promptParent()
		if err != nil {
			return createInput{}, err
		}
		nameInput, err := r.promptName(parentAccount.Name)
		if err != nil {
			return createInput{}, err
		}
		r.applyParentSettings(parentAccount, parentAccount.Currency, &input)
		input.fullName = r.accSvc.FormatAccountName(parentAccount.Name, nameInput)
	} else {
		accType, err := r.promptType()
		if err != nil {
			return createInput{}, err
		}
		rootName, err := r.accSvc.GetRootNameByType(accType)
		if err != nil {
			return createInput{}, err
		}
		nameInput, err := r.promptName(rootName)
		if err != nil {
			return createInput{}, err
		}
		if err := r.applyTypeSettings(rootName, accType, "", &input); err != nil {
			return createInput{}, err
		}
		input.fullName = r.accSvc.FormatAccountName(rootName, nameInput)
	}

	currency, err := r.promptCurrency(input)
	if err != nil {
		return createInput{}, err
	}
	input.currency = currency

	if input.accountType == "A" || input.accountType == "L" {
		balanceCents, err := r.promptBalance()
		if err != nil {
			return createInput{}, err
		}
		input.balanceCents = balanceCents
	}

	desc, err := r.promptDescription()
	if err != nil {
		return createInput{}, err
	}
	input.description = desc

	if err := r.view.RenderSummary(views.AccountSummaryItem{
		FullName:    input.fullName,
		Type:        input.accountType,
		Currency:    input.currency,
		Balance:     input.balanceCents,
		Description: input.description,
	}); err != nil {
		return createInput{}, err
	}

	if err := r.confirm(); err != nil {
		return createInput{}, err
	}

	return input, nil
}
```

- [ ] **Step 6: Update all action helpers in `create_actions.go` to accept `*createInput`**

Change signatures of `applyTypeSettings`, `applyParentSettings`, `buildFromParentName`, `buildFromTypeFlag` to take a `*createInput` parameter instead of mutating `r.*` fields. Update `createAccount` to accept `createInput`:

```go
func (r *createRunner) createAccount(input createInput) (*model.Account, error) {
	return r.accSvc.CreateAccountWithBalance(
		input.fullName,
		input.accountType,
		input.currency,
		input.description,
		input.parentID,
		input.balanceCents,
	)
}

func (r *createRunner) applyTypeSettings(rootName, accType, currencyOverride string, input *createInput) error {
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

func (r *createRunner) applyParentSettings(parent *model.Account, currencyOverride string, input *createInput) {
	input.accountType = parent.Type
	input.parentID = &parent.ID
	if currencyOverride != "" {
		input.currency = currencyOverride
	} else {
		input.currency = parent.Currency
	}
}

func (r *createRunner) buildFromParentName(parentName, currency string, input *createInput) error {
	parentAccount, err := r.accSvc.GetAccountByName(parentName)
	if err != nil {
		return err
	}
	r.applyParentSettings(parentAccount, currency, input)
	input.fullName = parentAccount.Name // capture prefix so runFromFlags can call FormatAccountName
	return nil
}

func (r *createRunner) buildFromTypeFlag(accType, currency string, input *createInput) error {
	rootName, err := r.accSvc.GetRootNameByType(accType)
	if err != nil {
		return fmt.Errorf("get root name: %w", err)
	}
	if err := r.applyTypeSettings(rootName, accType, currency, input); err != nil {
		return err
	}
	input.fullName = rootName // capture prefix so runFromFlags can call FormatAccountName
	return nil
}
```

Update `promptCurrency` to accept `createInput` to check `parentID`:

```go
func (r *createRunner) promptCurrency(input createInput) (string, error) {
	defaultCurrency := input.currency
	if defaultCurrency == "" {
		defaultCurrency = r.defaultCurrency
	}
	isInherited := input.parentID != nil
	return prompts.PromptCurrency(defaultCurrency, isInherited, r.accSvc.ValidateCurrency)
}
```

- [ ] **Step 7: Verify**

```bash
go build ./...
go test ./...
```

- [ ] **Step 8: Commit**

```bash
git add cmd/account/create.go cmd/account/create_actions.go cmd/account/create_types.go
git commit -m "refactor: createRunner — remove mutable state; introduce createInput value type"
```

---

## Summary

After all 8 tasks, the codebase achieves:

| Principle | Before | After |
|-----------|--------|-------|
| Runner holds only deps, not state | ❌ createRunner | ✅ all runners |
| Minimal provider interfaces | ❌ 4 commands use full `*service.Service` | ✅ all commands |
| Flags always in a struct | ❌ 5 commands use inline `var` | ✅ all commands |
| Errors propagate from `Run()` | ❌ 4 commands swallow with pterm | ✅ all commands |
| Types files contain only types | ❌ modeUIConfigs in add_types.go | ✅ all types files |
| report.go runner variable name | ❌ `r` | ✅ `runner` |
