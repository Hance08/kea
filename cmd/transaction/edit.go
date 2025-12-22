package transaction

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/hance08/kea/internal/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type EditCommandRunner struct {
	svc *service.Service
}

func NewEditCmd(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <transaction-id>",
		Short: "Edit a transaction",
		Long:  `Edit a transaction's description, date, status, and splits interactively.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &EditCommandRunner{
				svc: svc,
			}
			return runner.Run(args)
		},
	}
}

func (r *EditCommandRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Prevent editing opening balance transaction
	if txID == 1 {
		pterm.Error.Println("Cannot edit the opening balance transaction")
		return nil
	}

	// Get current transaction details
	detail, err := r.svc.Transaction.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	// Show current transaction info
	pterm.DefaultSection.Printf("Editing Transaction #%d", txID)
	if err := views.RenderTransactionDetail(detail); err != nil {
		return err
	}

	// Main edit menu
	for {
		// Build menu options based on transaction complexity
		menuOptions := []string{
			"Basic Info (description, date, status)",
		}

		// Add quick edit options for simple transactions (2 splits)
		if len(detail.Splits) == 2 {
			menuOptions = append(menuOptions,
				"Change Account (quick edit)",
				"Change Amount (both sides)",
			)
		}

		menuOptions = append(menuOptions,
			"Edit Splits (Advanced)",
			"Save & Exit",
			"Cancel (discard changes)",
		)

		var editChoice string
		editChoice, err := prompts.PromptSelect(
			"What would you like to edit?",
			menuOptions,
			"",
		)
		if err != nil {
			return err
		}

		switch editChoice {
		case "Basic Info (description, date, status)":
			if err := r.editBasicInfo(detail); err != nil {
				pterm.Error.Printf("Failed to edit basic info: %v\n", err)
			}

		case "Change Account (quick edit)":
			if err := r.editAccount(detail); err != nil {
				pterm.Error.Printf("Failed to change account: %v\n", err)
			}

		case "Change Amount (both sides)":
			if err := r.editAmount(detail); err != nil {
				pterm.Error.Printf("Failed to change amount: %v\n", err)
			}

		case "Edit Splits (Advanced)":
			if err := r.editSplits(detail); err != nil {
				pterm.Error.Printf("Failed to edit splits: %v\n", err)
			}

		case "Save & Exit":
			// Validate before saving
			splits := detail.ToSplitInputs()
			if err := r.svc.Transaction.ValidateTransactionEdit(splits); err != nil {
				pterm.Error.Printf("Cannot save: %v\n", err)
				pterm.Warning.Println("Please fix the errors before saving")
				continue
			}

			// Save changes
			if err := r.saveTransactionChanges(txID, detail); err != nil {
				pterm.Error.Printf("Failed to save changes: %v\n", err)
				return nil
			}

			pterm.Success.Printf("Transaction #%d updated successfully\n", txID)
			ui.Separator()
			return nil

		case "Cancel (discard changes)":
			pterm.Info.Println("Changes discarded")
			return nil
		}
	}
}

func (r *EditCommandRunner) editBasicInfo(detail *service.TransactionDetail) error {
	// Edit description
	newDescription, err := prompts.PromptInput("Description:", detail.Description, nil)
	if err != nil {
		return err
	}
	detail.Description = newDescription

	// Edit date
	currentDate := time.Unix(detail.Timestamp, 0).Format("2006-01-02")
	newDateStr, err := prompts.PromptDate("Date (YYYY-MM-DD):", currentDate, "")
	if err != nil {
		return err
	}

	newDate, err := time.Parse("2006-01-02", newDateStr)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}
	detail.Timestamp = newDate.Unix()

	// Edit status
	defaultStatus := "Cleared"
	if detail.Status == 0 {
		defaultStatus = "Pending"
	}

	newStatus, err := prompts.PromptTransactionStatus(defaultStatus)
	if err != nil {
		return err
	}

	if newStatus == "Pending" {
		detail.Status = 0
	} else {
		detail.Status = 1
	}

	pterm.Success.Println("Basic info updated")
	ui.Separator()
	return nil
}

// editAccount allows quick account switching for simple transactions
// Works best for 2-split transactions (Expense/Income/Transfer)
func (r *EditCommandRunner) editAccount(detail *service.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		pterm.Warning.Println("This feature works best with 2-split transactions")
		pterm.Info.Println("Use 'Edit Splits (Advanced)' for complex transactions")
		return nil
	}

	// Detect transaction type and check if editing is allowed
	txType, err := r.svc.Transaction.DetermineType(detail.Splits)
	if err != nil {
		return err
	}

	if txType == service.TxTypeOpening {
		pterm.Warning.Println("Cannot use quick edit for Opening Balance transactions")
		pterm.Info.Println("Use 'Edit Splits (Advanced)' if you need to modify this transaction")
		return nil
	}

	// Get all accounts
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// Display current splits with role labels
	pterm.DefaultSection.Printf("Current transaction type: %s", txType)
	pterm.DefaultSection.Printf("Current splits:")

	roleLabels := views.GetSplitRoleLabels(detail.Splits, txType)

	for i, split := range detail.Splits {
		amount := utils.FormatFromCents(split.Amount)
		sign := "+"
		if split.Amount < 0 {
			sign = ""
		}
		pterm.Printf("  %d. %s (%s): %s%s %s\n",
			i+1, split.AccountName, roleLabels[i], sign, amount, split.Currency)
	}
	fmt.Println()

	// Let user choose which split to change
	splitOptions := []string{
		fmt.Sprintf("1. %s", detail.Splits[0].AccountName),
		fmt.Sprintf("2. %s", detail.Splits[1].AccountName),
		"Cancel",
	}

	splitChoice, err := prompts.PromptSelect(
		"Which account do you want to change?",
		splitOptions,
		"",
	)
	if err != nil {
		return err
	}

	if splitChoice == "Cancel" {
		return nil
	}

	// Determine which split to edit (0 or 1)
	splitIndex := 0
	if splitChoice[0] == '2' {
		splitIndex = 1
	}

	split := &detail.Splits[splitIndex]

	currentAccount, err := r.svc.Account.GetAccountByName(split.AccountName)
	if err != nil {
		return fmt.Errorf("failed to get current account details: %w", err)
	}

	filteredAccounts := r.svc.Transaction.GetAllowedAccounts(
		txType,
		currentAccount.Type,
		accounts,
	)

	if len(filteredAccounts) == 0 {
		pterm.Warning.Println("No suitable accounts found for this change")
		return nil
	}

	// Build account selection list
	var accountNames []string
	for _, acc := range filteredAccounts {
		accountNames = append(accountNames, acc.Name)
	}

	// Select new account
	selectedAccount, err := prompts.PromptSelect(
		fmt.Sprintf("Select new %s:", roleLabels[splitIndex]),
		accountNames,
		split.AccountName,
	)
	if err != nil {
		return err
	}

	// Update the split
	account, err := r.svc.Account.GetAccountByName(selectedAccount)
	if err != nil {
		return err
	}

	split.AccountID = account.ID
	split.AccountName = account.Name
	split.Currency = account.Currency

	pterm.Success.Printf("Account changed to: %s\n", account.Name)
	ui.Separator()
	return nil
}

// editAmount allows quick amount editing for simple transactions
// Automatically adjusts both sides to maintain balance
func (r *EditCommandRunner) editAmount(detail *service.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		pterm.Warning.Println("This feature works best with 2-split transactions")
		pterm.Info.Println("Use 'Edit Splits (Advanced)' for complex transactions")
		return nil
	}

	// Display current amount
	currentAmount := detail.Splits[0].Amount
	if currentAmount < 0 {
		currentAmount = -currentAmount
	}
	currentAmountStr := utils.FormatFromCents(currentAmount)

	pterm.DefaultSection.Printf("Current amount: %s %s", currentAmountStr, detail.Splits[0].Currency)
	tableData := pterm.TableData{
		{"#", "Account", "Amount"},
	}

	var balance int64
	for i, split := range detail.Splits {
		amount := utils.FormatFromCents(split.Amount)
		sign := "+"
		if split.Amount < 0 {
			sign = "-"
			amount = utils.FormatFromCents(-split.Amount)
		}
		memo := split.Memo
		if memo == "" {
			memo = "-"
		}
		tableData = append(tableData, []string{
			fmt.Sprintf("%d", i+1),
			split.AccountName,
			fmt.Sprintf("%s%s %s", sign, amount, split.Currency),
			memo,
		})
		balance += split.Amount
	}

	if err := pterm.DefaultTable.WithHasHeader().WithData(tableData).Render(); err != nil {
		return fmt.Errorf("failed to render table with data: %w", err)
	}

	// Input new amount
	newAmountStr, err := prompts.PromptInput("Enter new amount (positive number):", currentAmountStr, nil)
	if err != nil {
		return err
	}

	newAmount, err := utils.ParseToCents(newAmountStr)
	if err != nil {
		return err
	}

	// Make sure it's positive
	if newAmount < 0 {
		newAmount = -newAmount
	}

	// Adjust both splits while maintaining their signs
	if detail.Splits[0].Amount > 0 {
		detail.Splits[0].Amount = newAmount
		detail.Splits[1].Amount = -newAmount
	} else {
		detail.Splits[0].Amount = -newAmount
		detail.Splits[1].Amount = newAmount
	}

	pterm.Success.Printf("Amount changed to: %s %s\n",
		utils.FormatFromCents(newAmount),
		detail.Splits[0].Currency)
	ui.Separator()
	return nil
}

func (r *EditCommandRunner) editSplits(detail *service.TransactionDetail) error {
	for {
		// Display current splits with balance
		if err := views.RenderTransactionDetail(detail); err != nil {
			return err
		}

		options := []string{
			"Add Split",
			"Edit Split",
			"Delete Split",
			"Done (return to main menu)",
		}

		action, err := prompts.PromptSelect(
			"Splits Editor:",
			options,
			"",
		)
		if err != nil {
			return err
		}

		switch action {
		case "Add Split":
			if err := r.addSplit(detail); err != nil {
				pterm.Error.Printf("Failed to add split: %v\n", err)
			}

		case "Edit Split":
			if err := r.editOneSplit(detail); err != nil {
				pterm.Error.Printf("Failed to edit split: %v\n", err)
			}

		case "Delete Split":
			if err := r.deleteSplit(detail); err != nil {
				pterm.Error.Printf("Failed to delete split: %v\n", err)
			}

		case "Done (return to main menu)":
			return nil
		}
	}
}

func (r *EditCommandRunner) addSplit(detail *service.TransactionDetail) error {
	// Select account
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	var accountNames []string
	for _, acc := range accounts {
		accountNames = append(accountNames, acc.Name)
	}

	selectedAccount, err := prompts.PromptSelect(
		"Select account:",
		accountNames,
		"",
	)
	if err != nil {
		return err
	}

	// Get account details
	account, err := r.svc.Account.GetAccountByName(selectedAccount)
	if err != nil {
		return err
	}

	// Input amount
	amountStr, err := prompts.PromptInput("Amount (use negative for credit):", "", nil)
	if err != nil {
		return err
	}

	amount, err := utils.ParseToCents(amountStr)
	if err != nil {
		return err
	}

	// Input memo
	memo, err := prompts.PromptInput("Memo (optional):", "", nil)
	if err != nil {
		return err
	}

	// Add split to detail
	newSplit := service.SplitDetail{
		ID:          0, // New split
		AccountID:   account.ID,
		AccountName: account.Name,
		Amount:      amount,
		Currency:    account.Currency,
		Memo:        memo,
	}
	detail.Splits = append(detail.Splits, newSplit)

	pterm.Success.Println("Split added")
	ui.Separator()
	return nil
}

func (r *EditCommandRunner) editOneSplit(detail *service.TransactionDetail) error {
	if len(detail.Splits) == 0 {
		return fmt.Errorf("no splits to edit")
	}

	// Select split to edit
	var splitOptions []string
	for i, split := range detail.Splits {
		amount := utils.FormatFromCents(split.Amount)
		splitOptions = append(splitOptions, fmt.Sprintf("#%d: %s (%s %s)", i+1, split.AccountName, amount, split.Currency))
	}

	selectedSplit, err := prompts.PromptSelect(
		"Select split to edit:",
		splitOptions,
		"",
	)
	if err != nil {
		return err
	}

	// Find split index
	var splitIndex int
	fmt.Sscanf(selectedSplit, "#%d:", &splitIndex)
	splitIndex-- // Convert to 0-based index

	split := &detail.Splits[splitIndex]

	// Edit account
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	var accountNames []string
	for _, acc := range accounts {
		accountNames = append(accountNames, acc.Name)
	}

	selectedAccount, err := prompts.PromptSelect(
		"Account:",
		accountNames,
		split.AccountName, // Default
	)
	if err != nil {
		return err
	}

	account, err := r.svc.Account.GetAccountByName(selectedAccount)
	if err != nil {
		return err
	}

	// Edit amount
	currentAmount := utils.FormatFromCents(split.Amount)
	amountStr, err := prompts.PromptInput("Amount:", currentAmount, nil)
	if err != nil {
		return err
	}

	amount, err := utils.ParseToCents(amountStr)
	if err != nil {
		return err
	}

	// Edit memo
	memo, err := prompts.PromptInput("Memo:", split.Memo, nil)
	if err != nil {
		return err
	}

	// Update split
	split.AccountID = account.ID
	split.AccountName = account.Name
	split.Amount = amount
	split.Currency = account.Currency
	split.Memo = memo

	pterm.Success.Println("Split updated")
	ui.Separator()
	return nil
}

func (r *EditCommandRunner) deleteSplit(detail *service.TransactionDetail) error {
	if len(detail.Splits) <= 2 {
		return fmt.Errorf("cannot delete: transaction must have at least 2 splits")
	}

	// Select split to delete
	var splitOptions []string
	for i, split := range detail.Splits {
		amount := utils.FormatFromCents(split.Amount)
		splitOptions = append(splitOptions, fmt.Sprintf("#%d: %s (%s %s)", i+1, split.AccountName, amount, split.Currency))
	}

	selectedSplit, err := prompts.PromptSelect(
		"Select split to edit:",
		splitOptions,
		"",
	)
	if err != nil {
		return err
	}

	// Find split index
	var splitIndex int
	fmt.Sscanf(selectedSplit, "#%d:", &splitIndex)
	splitIndex-- // Convert to 0-based index

	// Confirm deletion
	confirm, err := prompts.PromptConfirm(
		fmt.Sprintf("Delete split: %s?", detail.Splits[splitIndex].AccountName),
		false, // Default value
	)
	if err != nil {
		return err
	}

	if !confirm {
		pterm.Info.Println("Deletion cancelled")
		return nil
	}

	// Delete split
	detail.Splits = append(detail.Splits[:splitIndex], detail.Splits[splitIndex+1:]...)

	pterm.Success.Println("Split deleted")
	ui.Separator()
	return nil
}

func (r *EditCommandRunner) saveTransactionChanges(txID int64, detail *service.TransactionDetail) error {
	splits := detail.ToSplitInputs()
	return r.svc.Transaction.UpdateTransactionComplete(
		txID,
		detail.Description,
		detail.Timestamp,
		detail.Status,
		splits,
	)
}
