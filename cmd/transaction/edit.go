package transaction

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/hance08/kea/internal/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type editRunner struct {
	svc *service.Service
}

func NewEditCmd(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <transaction-id>",
		Short: "Edit a transaction",
		Long:  `Edit a transaction's description, date, status, and splits interactively.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &editRunner{
				svc: svc,
			}
			return runner.Run(args)
		},
	}
}

func (r *editRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Fetch Data
	detail, err := r.svc.Transaction.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	if !r.svc.Transaction.IsEditable(detail) {
		pterm.Error.Println("This transaction cannot be edited (System Transaction)")
		return nil
	}

	ui.PrintL1Title("Editing Transaction #%d", txID)

	if err := views.RenderTransactionDetail(detail); err != nil {
		return err
	}

	return r.runEditMenu(txID, detail)
}

func (r *editRunner) runEditMenu(txID int64, detail *service.TransactionDetail) error {
	for {
		menuOptions := r.buildMenuOptions(detail)

		choice, err := prompts.PromptSelect("What would you like to edit?", menuOptions, "")
		if err != nil {
			return err
		}

		switch choice {
		case "Basic Info (description, date, status)":
			if err := r.actionEditBasicInfo(detail); err != nil {
				pterm.Error.Printf("Failed: %v\n", err)
			}

		case "Change Account (quick edit)":
			if err := r.actionQuickChangeAccount(detail); err != nil {
				pterm.Error.Printf("Failed: %v\n", err)
			}

		case "Change Amount (both sides)":
			if err := r.actionQuickChangeAmount(detail); err != nil {
				pterm.Error.Printf("Failed: %v\n", err)
			}

		case "Edit Splits (Advanced)":
			if err := r.runSplitsMenu(detail); err != nil {
				pterm.Error.Printf("Failed: %v\n", err)
			}

		case "Save & Exit":
			if err := r.actionSave(txID, detail); err != nil {
				// Save failed, stay in loop
				pterm.Error.Printf("Cannot save: %v\n", err)
				pterm.Warning.Println("Please fix the errors before saving")
				continue
			}
			return nil

		case "Cancel (discard changes)":
			pterm.Info.Println("Changes discarded")
			return nil
		}
	}
}

func (r *editRunner) buildMenuOptions(detail *service.TransactionDetail) []string {
	options := []string{
		"Basic Info (description, date, status)",
	}

	// Quick edit options only for simple transactions
	if len(detail.Splits) == 2 {
		options = append(options,
			"Change Account (quick edit)",
			"Change Amount (both sides)",
		)
	}

	options = append(options,
		"Edit Splits (Advanced)",
		"Save & Exit",
		"Cancel (discard changes)",
	)
	return options
}

// ==========================================
// Actions: Basic & Quick Edits
// ==========================================

func (r *editRunner) actionEditBasicInfo(detail *service.TransactionDetail) error {
	// Description
	desc, err := prompts.PromptInput("Description:", detail.Description, nil)
	if err != nil {
		return err
	}
	detail.Description = desc

	// Date
	currentDate := time.Unix(detail.Timestamp, 0).Format("2006-01-02")
	dateStr, err := prompts.PromptDate("Date (YYYY-MM-DD):", currentDate, "")
	if err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", dateStr) // PromptDate validates format
	if err != nil {
		return fmt.Errorf("unexpected date format error: %w", err)
	}

	detail.Timestamp = t.Unix()

	// Status
	statusStr, err := prompts.PromptTransactionStatus(r.getStatusString(detail.Status))
	if err != nil {
		return err
	}
	detail.Status = r.getStatusValue(statusStr)

	pterm.Success.Println("Basic info updated")
	ui.Separator()
	return nil
}

func (r *editRunner) actionQuickChangeAccount(detail *service.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		return fmt.Errorf("quick edit supports only 2 splits")
	}

	// Determine Type (Asset/Expense/etc)
	txType, err := r.svc.Transaction.DetermineType(detail.Splits)
	if err != nil {
		return err
	}

	if txType == service.TxTypeOpening {
		pterm.Warning.Println("Cannot quick-edit Opening Balance transaction")
		return nil
	}

	// Show current state
	pterm.DefaultSection.Printf("Current splits (%s):", txType)
	views.RenderSimpleSplitList(detail.Splits, txType)

	// Select which split to change
	splitIndex, err := r.promptSplitSelection(detail)
	if err != nil {
		return err // User cancelled or error
	}
	if splitIndex == -1 {
		return nil // Cancelled
	}

	targetSplit := &detail.Splits[splitIndex]

	// Filter compatible accounts
	allAccounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return err
	}

	currentAcc, err := r.svc.Account.GetAccountByName(targetSplit.AccountName)
	if err != nil {
		return fmt.Errorf("failed to load account details for '%s': %w", targetSplit.AccountName, err)
	}

	compatibleAccounts := r.svc.Transaction.GetAllowedAccounts(txType, currentAcc.Type, allAccounts)

	if len(compatibleAccounts) == 0 {
		pterm.Warning.Println("No other compatible accounts found")
		return nil
	}

	// Select new account
	newAccName, err := r.promptAccountSelectionFromList(compatibleAccounts, targetSplit.AccountName)
	if err != nil {
		return err
	}

	newAcc, _ := r.svc.Account.GetAccountByName(newAccName)

	// Apply Change
	targetSplit.AccountID = newAcc.ID
	targetSplit.AccountName = newAcc.Name
	targetSplit.Currency = newAcc.Currency

	pterm.Success.Printf("Account changed to: %s\n", newAcc.Name)
	ui.Separator()
	return nil
}

func (r *editRunner) actionQuickChangeAmount(detail *service.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		return fmt.Errorf("quick edit supports only 2 splits")
	}

	// UI: Show Current
	currentAbsAmount := utils.AbsInt64(detail.Splits[0].Amount)

	// UI: Prompt
	newAmountStr, err := prompts.PromptInput("Enter new amount:", utils.FormatFromCents(currentAbsAmount), nil)
	if err != nil {
		return err
	}

	newAmount, err := utils.ParseToCents(newAmountStr)
	if err != nil {
		return err
	}
	newAbsAmount := utils.AbsInt64(newAmount)

	// Logic: Update both sides, preserving sign
	if err := detail.UpdateAmountPreservingBalance(newAbsAmount); err != nil {
		return err
	}

	pterm.Success.Println("Amount updated")
	ui.Separator()
	return nil
}

// ==========================================
// Sub-Menu: Advanced Splits Editor
// ==========================================

func (r *editRunner) runSplitsMenu(detail *service.TransactionDetail) error {
	for {
		if err := views.RenderTransactionDetail(detail); err != nil {
			return err
		}

		action, err := prompts.PromptSelect("Splits Editor:", []string{
			"Add Split", "Edit Split", "Delete Split", "Done",
		}, "")
		if err != nil {
			return err
		}

		switch action {
		case "Add Split":
			if err := r.actionAddSplit(detail); err != nil {
				return err
			}
		case "Edit Split":
			if err := r.actionEditSplit(detail); err != nil {
				return err
			}
		case "Delete Split":
			if err := r.actionDeleteSplit(detail); err != nil {
				return err
			}
		case "Done":
			return nil
		}
	}
}

func (r *editRunner) actionAddSplit(detail *service.TransactionDetail) error {
	// 1. Select Account
	accName, err := r.promptAccountSelection("")
	if err != nil {
		return err
	}

	acc, _ := r.svc.Account.GetAccountByName(accName)

	// 2. Input Amount
	amount, err := r.promptAmount("Amount (negative for credit):", 0)
	if err != nil {
		return err
	}

	// 3. Input Memo
	memo, err := prompts.PromptInput("Memo (optional):", "", nil)
	if err != nil {
		return err
	}

	// 4. Append
	detail.Splits = append(detail.Splits, service.SplitDetail{
		AccountID: acc.ID, AccountName: acc.Name, Currency: acc.Currency,
		Amount: amount, Memo: memo,
	})
	return nil
}

func (r *editRunner) actionEditSplit(detail *service.TransactionDetail) error {
	idx, err := r.promptSplitSelection(detail)
	if err != nil || idx == -1 {
		return err
	}

	split := &detail.Splits[idx]

	// Edit Account
	newAccName, err := r.promptAccountSelection(split.AccountName)
	if err != nil {
		return err
	}
	acc, _ := r.svc.Account.GetAccountByName(newAccName)

	// Edit Amount
	newAmount, err := r.promptAmount("Amount:", split.Amount)
	if err != nil {
		return err
	}

	// Edit Memo
	newMemo, err := prompts.PromptInput("Memo:", split.Memo, nil)
	if err != nil {
		return err
	}

	// Apply
	split.AccountID = acc.ID
	split.AccountName = acc.Name
	split.Currency = acc.Currency
	split.Amount = newAmount
	split.Memo = newMemo

	return nil
}

func (r *editRunner) actionDeleteSplit(detail *service.TransactionDetail) error {
	if len(detail.Splits) <= constants.MinSplitsCount {
		pterm.Warning.Printf("Transaction must have at least %d splits\n", constants.MinSplitsCount)
		return nil
	}

	idx, err := r.promptSplitSelection(detail)
	if err != nil || idx == -1 {
		return err
	}

	// Confirm
	if yes, _ := prompts.PromptConfirm("Delete this split?", false); !yes {
		return nil
	}

	// Remove (Slice deletion trick)
	detail.Splits = append(detail.Splits[:idx], detail.Splits[idx+1:]...)
	return nil
}

// ==========================================
// Finalize
// ==========================================

func (r *editRunner) actionSave(txID int64, detail *service.TransactionDetail) error {
	// Validate via Service
	splits := detail.ToSplitInputs()
	if err := r.svc.Transaction.ValidateTransactionEdit(splits); err != nil {
		return err
	}

	// Execute Update
	if err := r.svc.Transaction.UpdateTransactionComplete(
		txID, detail.Description, detail.Timestamp, detail.Status, splits,
	); err != nil {
		return err
	}

	pterm.Success.Printf("Transaction #%d saved successfully\n", txID)
	return nil
}

// ==========================================
// Helpers (UI & Logic)
// ==========================================

func (r *editRunner) getStatusString(status int) string {
	if status == 0 {
		return "Pending"
	}
	return "Cleared"
}

func (r *editRunner) getStatusValue(s string) int {
	if s == "Pending" {
		return 0
	}
	return 1
}

func (r *editRunner) promptAccountSelection(defaultName string) (string, error) {
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return "", err
	}
	return r.promptAccountSelectionFromList(accounts, defaultName)
}

func (r *editRunner) promptAccountSelectionFromList(accounts []*model.Account, defaultName string) (string, error) {
	var names []string
	for _, a := range accounts {
		names = append(names, a.Name)
	}
	return prompts.PromptSelect("Select Account:", names, defaultName)
}

func (r *editRunner) promptAmount(label string, defaultCents int64) (int64, error) {
	defaultStr := ""
	if defaultCents != 0 {
		defaultStr = utils.FormatFromCents(defaultCents)
	}
	valStr, err := prompts.PromptInput(label, defaultStr, nil)
	if err != nil {
		return 0, err
	}
	return utils.ParseToCents(valStr)
}

func (r *editRunner) promptSplitSelection(detail *service.TransactionDetail) (int, error) {
	var options []string
	for i, s := range detail.Splits {
		options = append(options, fmt.Sprintf("#%d %s (%s)", i+1, s.AccountName, utils.FormatFromCents(s.Amount)))
	}
	options = append(options, "Cancel")

	choice, err := prompts.PromptSelect("Select Split:", options, "")
	if err != nil {
		return -1, err
	}
	if choice == "Cancel" {
		return -1, nil
	}

	var idx int
	_, err = fmt.Sscanf(choice, "#%d", &idx)
	if err != nil {
		return -1, fmt.Errorf("failed to parse selection: %w", err)
	}
	return idx - 1, nil
}
