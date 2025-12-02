/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hance08/kea/internal/logic/accounting"
	"github.com/hance08/kea/internal/store"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// surveyOpts contains custom options for all survey prompts
var surveyOpts = []survey.AskOpt{
	survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}),
}

// transactionCmd represents the transaction command
var transactionCmd = &cobra.Command{
	Use:   "transaction",
	Short: "Manage transactions",
	Long:  `Manage transactions: view details, delete, or modify transaction status.`,
}

// transactionShowCmd shows details of a specific transaction
var transactionShowCmd = &cobra.Command{
	Use:   "show <transaction-id>",
	Short: "Show transaction details",
	Long:  `Display detailed information about a specific transaction including all splits.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTransactionShow,
}

// transactionDeleteCmd deletes a transaction
var transactionDeleteCmd = &cobra.Command{
	Use:   "delete <transaction-id>",
	Short: "Delete a transaction",
	Long:  `Delete a transaction and all its associated splits. This action cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTransactionDelete,
}

// transactionClearCmd marks a transaction as cleared
var transactionClearCmd = &cobra.Command{
	Use:   "clear <transaction-id>",
	Short: "Mark transaction as cleared",
	Long:  `Mark a pending transaction as cleared (confirmed).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTransactionClear,
}

// transactionEditCmd edits a transaction
var transactionEditCmd = &cobra.Command{
	Use:   "edit <transaction-id>",
	Short: "Edit a transaction",
	Long:  `Edit a transaction's description, date, status, and splits interactively.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTransactionEdit,
}

func init() {
	rootCmd.AddCommand(transactionCmd)
	transactionCmd.AddCommand(transactionShowCmd)
	transactionCmd.AddCommand(transactionDeleteCmd)
	transactionCmd.AddCommand(transactionClearCmd)
	transactionCmd.AddCommand(transactionEditCmd)
}

func runTransactionShow(cmd *cobra.Command, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Get transaction details
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	// Basic info table
	date := time.Unix(detail.Timestamp, 0).Format("2006-01-02 15:04:05")
	status := "Cleared"
	if detail.Status == 0 {
		status = "Pending"
	}

	pterm.DefaultSection.Println("Transaction Info")
	infoData := pterm.TableData{
		{"Field", "Value"},
		{"ID", fmt.Sprintf("%d", detail.ID)},
		{"Date", date},
		{"Description", detail.Description},
		{"Status", status},
	}

	pterm.DefaultTable.WithHasHeader().WithData(infoData).Render()

	// Splits table
	pterm.DefaultSection.Println("Splits (Double-Entry)")

	splitsData := pterm.TableData{
		{"Account", "Amount", "Type", "Memo"},
	}

	var total int64
	for _, split := range detail.Splits {
		amountStr := fmt.Sprintf("%.2f %s", float64(split.Amount)/100.0, split.Currency)
		typeStr := "Debit +"
		if split.Amount < 0 {
			typeStr = "Credit -"
			amountStr = fmt.Sprintf("%.2f %s", float64(-split.Amount)/100.0, split.Currency)
		}

		accountName := split.AccountName
		if accountName == "" {
			accountName = fmt.Sprintf("[ID: %d]", split.AccountID)
		}

		memo := split.Memo
		if memo == "" {
			memo = "-"
		}

		splitsData = append(splitsData, []string{
			accountName,
			amountStr,
			typeStr,
			memo,
		})

		total += split.Amount
	}

	pterm.DefaultTable.WithHasHeader().WithData(splitsData).Render()

	return nil
}

func runTransactionDelete(cmd *cobra.Command, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Get transaction details first to show what will be deleted
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to delete transaction: %v\n", err)
		return nil
	}
	if detail.ID == 1 {
		pterm.Error.Println("Can't delete the opening transaction")
		return nil
	}

	// Show transaction summary
	date := time.Unix(detail.Timestamp, 0).Format("2006-01-02")
	pterm.Warning.Printf("About to delete transaction #%d:\n", detail.ID)
	deletionInfo := pterm.TableData{
		{"Date", date},
		{"Description", detail.Description},
		{"Splits", fmt.Sprint(len(detail.Splits))},
	}

	pterm.DefaultTable.WithData(deletionInfo).Render()

	// Confirm deletion
	pterm.Warning.Println("This action cannot be undone!")

	var confirmation bool
	confirmPrompt := &survey.Confirm{
		Message: "Do you want to delete this transaction?",
		Default: false,
	}
	if err := survey.AskOne(confirmPrompt, &confirmation, surveyOpts...); err != nil {
		return err
	}

	if !confirmation {
		pterm.Info.Println("Deletion cancelled")
		return nil
	}

	// Delete transaction
	if err := logic.DeleteTransaction(txID); err != nil {
		pterm.Error.Printf("Failed to delete transaction: %v\n", err)
		return nil
	}

	pterm.Success.Printf("Transaction #%d deleted successfully\n", txID)
	printSeparator()
	return nil
}

func runTransactionClear(cmd *cobra.Command, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Update status to cleared (1)
	if err := logic.UpdateTransactionStatus(txID, 1); err != nil {
		pterm.Error.Printf("Failed to update transaction status: %v\n", err)
		return nil
	}

	pterm.Success.Printf("Transaction #%d marked as cleared\n", txID)
	printSeparator()
	return nil
}

func runTransactionEdit(cmd *cobra.Command, args []string) error {
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
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	// Show current transaction info
	pterm.DefaultSection.Printf("Editing Transaction #%d", txID)
	displayTransactionDetail(detail)

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

		// Always show advanced option
		menuOptions = append(menuOptions,
			"Edit Splits (Advanced)",
			"Save & Exit",
			"Cancel (discard changes)",
		)

		var editChoice string
		editPrompt := &survey.Select{
			Message: "What would you like to edit?",
			Options: menuOptions,
		}
		if err := survey.AskOne(editPrompt, &editChoice, surveyOpts...); err != nil {
			return err
		}

		switch editChoice {
		case "Basic Info (description, date, status)":
			if err := editBasicInfo(detail); err != nil {
				pterm.Error.Printf("Failed to edit basic info: %v\n", err)
			}

		case "Change Account (quick edit)":
			if err := changeAccount(detail); err != nil {
				pterm.Error.Printf("Failed to change account: %v\n", err)
			}

		case "Change Amount (both sides)":
			if err := changeAmount(detail); err != nil {
				pterm.Error.Printf("Failed to change amount: %v\n", err)
			}

		case "Edit Splits (Advanced)":
			if err := editSplits(detail); err != nil {
				pterm.Error.Printf("Failed to edit splits: %v\n", err)
			}

		case "Save & Exit":
			// Validate before saving
			splits := convertToSplitInputs(detail.Splits)
			if err := logic.ValidateTransactionEdit(splits); err != nil {
				pterm.Error.Printf("Cannot save: %v\n", err)
				pterm.Warning.Println("Please fix the errors before saving")
				continue
			}

			// Save changes
			if err := saveTransactionChanges(txID, detail); err != nil {
				pterm.Error.Printf("Failed to save changes: %v\n", err)
				return nil
			}

			pterm.Success.Printf("Transaction #%d updated successfully\n", txID)
			printSeparator()
			return nil

		case "Cancel (discard changes)":
			pterm.Info.Println("Changes discarded")
			return nil
		}
	}
}

func displayTransactionDetail(detail *accounting.TransactionDetail) {
	date := time.Unix(detail.Timestamp, 0).Format("2006-01-02 15:04")
	status := "Cleared"
	if detail.Status == 0 {
		status = "Pending"
	}

	pterm.Info.Printf("Date: %s | Status: %s\n", date, status)
	pterm.Info.Printf("Description: %s\n", detail.Description)
	pterm.Info.Printf("Splits: %d\n\n", len(detail.Splits))

	// Display splits table
	tableData := pterm.TableData{
		{"#", "Account", "Amount", "Memo"},
	}

	var balance int64
	for i, split := range detail.Splits {
		amount := logic.FormatAmountFromCents(split.Amount)
		sign := "+"
		if split.Amount < 0 {
			sign = "-"
			amount = logic.FormatAmountFromCents(-split.Amount)
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

	// Add balance row
	balanceStr := "✓ Balanced"
	if balance != 0 {
		balanceStr = fmt.Sprintf("⚠ Unbalanced: %s", logic.FormatAmountFromCents(balance))
	}
	tableData = append(tableData, []string{"", "", balanceStr, ""})

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
}

func editBasicInfo(detail *accounting.TransactionDetail) error {
	// Edit description
	var newDescription string
	descPrompt := &survey.Input{
		Message: "Description:",
		Default: detail.Description,
	}
	if err := survey.AskOne(descPrompt, &newDescription, surveyOpts...); err != nil {
		return err
	}
	detail.Description = newDescription

	// Edit date
	currentDate := time.Unix(detail.Timestamp, 0).Format("2006-01-02")
	var newDateStr string
	datePrompt := &survey.Input{
		Message: "Date (YYYY-MM-DD):",
		Default: currentDate,
	}
	if err := survey.AskOne(datePrompt, &newDateStr, surveyOpts...); err != nil {
		return err
	}

	newDate, err := time.Parse("2006-01-02", newDateStr)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}
	detail.Timestamp = newDate.Unix()

	// Edit status
	statusOptions := []string{"Pending", "Cleared"}
	defaultStatus := "Cleared"
	if detail.Status == 0 {
		defaultStatus = "Pending"
	}

	var newStatus string
	statusPrompt := &survey.Select{
		Message: "Status:",
		Options: statusOptions,
		Default: defaultStatus,
	}
	if err := survey.AskOne(statusPrompt, &newStatus, surveyOpts...); err != nil {
		return err
	}

	if newStatus == "Pending" {
		detail.Status = 0
	} else {
		detail.Status = 1
	}

	pterm.Success.Println("Basic info updated")
	printSeparator()
	return nil
}

// changeAccount allows quick account switching for simple transactions
// Works best for 2-split transactions (Expense/Income/Transfer)
func changeAccount(detail *accounting.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		pterm.Warning.Println("This feature works best with 2-split transactions")
		pterm.Info.Println("Use 'Edit Splits (Advanced)' for complex transactions")
		return nil
	}

	// Detect transaction type and check if editing is allowed
	txType, err := detectTransactionType(detail)
	if err != nil {
		return err
	}

	if txType == "Opening" {
		pterm.Warning.Println("Cannot use quick edit for Opening Balance transactions")
		pterm.Info.Println("Use 'Edit Splits (Advanced)' if you need to modify this transaction")
		return nil
	}

	// Get all accounts
	accounts, err := logic.GetAllAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// Display current splits with role labels
	pterm.DefaultSection.Printf("Current transaction type: %s", txType)
	pterm.DefaultSection.Printf("Current splits:")

	roleLabels := getSplitRoleLabels(detail, txType)
	for i, split := range detail.Splits {
		amount := logic.FormatAmountFromCents(split.Amount)
		sign := "+"
		if split.Amount < 0 {
			sign = ""
		}
		pterm.Printf("  %d. %s (%s): %s%s %s\n",
			i+1, split.AccountName, roleLabels[i], sign, amount, split.Currency)
	}
	fmt.Println()

	// Let user choose which split to change
	var splitChoice string
	splitPrompt := &survey.Select{
		Message: "Which account do you want to change?",
		Options: []string{
			fmt.Sprintf("1. %s", detail.Splits[0].AccountName),
			fmt.Sprintf("2. %s", detail.Splits[1].AccountName),
			"Cancel",
		},
	}
	if err := survey.AskOne(splitPrompt, &splitChoice, surveyOpts...); err != nil {
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

	// Filter accounts based on transaction type and split role
	filteredAccounts := filterAccountsForChange(accounts, detail, txType, splitIndex)

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
	var selectedAccount string
	accountPrompt := &survey.Select{
		Message: fmt.Sprintf("Select new %s:", roleLabels[splitIndex]),
		Options: accountNames,
		Default: split.AccountName,
	}
	if err := survey.AskOne(accountPrompt, &selectedAccount, surveyOpts...); err != nil {
		return err
	}

	// Update the split
	account, err := logic.GetAccountByName(selectedAccount)
	if err != nil {
		return err
	}

	split.AccountID = account.ID
	split.AccountName = account.Name
	split.Currency = account.Currency

	pterm.Success.Printf("Account changed to: %s\n", account.Name)
	printSeparator()
	return nil
}

// detectTransactionType determines the type of transaction based on account types
func detectTransactionType(detail *accounting.TransactionDetail) (string, error) {
	if len(detail.Splits) != 2 {
		return "Complex", nil
	}

	// Get account types for both splits
	account1, err := logic.GetAccountByName(detail.Splits[0].AccountName)
	if err != nil {
		return "", err
	}
	account2, err := logic.GetAccountByName(detail.Splits[1].AccountName)
	if err != nil {
		return "", err
	}

	type1, type2 := account1.Type, account2.Type

	// Check for Opening Balance
	if type1 == "C" || type2 == "C" {
		if account1.Name == "Equity:OpeningBalances" || account2.Name == "Equity:OpeningBalances" {
			return "Opening", nil
		}
	}

	// Expense: (A or L) + E
	if (type1 == "A" || type1 == "L") && type2 == "E" {
		return "Expense", nil
	}
	if type1 == "E" && (type2 == "A" || type2 == "L") {
		return "Expense", nil
	}

	// Income: R + (A or L)
	if type1 == "R" && (type2 == "A" || type2 == "L") {
		return "Income", nil
	}
	if (type1 == "A" || type1 == "L") && type2 == "R" {
		return "Income", nil
	}

	// Transfer: (A or L) + (A or L)
	if (type1 == "A" || type1 == "L") && (type2 == "A" || type2 == "L") {
		return "Transfer", nil
	}

	return "Other", nil
}

// getSplitRoleLabels returns descriptive labels for each split based on transaction type
func getSplitRoleLabels(detail *accounting.TransactionDetail, txType string) []string {
	labels := make([]string, len(detail.Splits))

	if len(detail.Splits) != 2 {
		for i := range labels {
			labels[i] = "account"
		}
		return labels
	}

	switch txType {
	case "Expense":
		// Find which is the expense account
		account1, _ := logic.GetAccountByName(detail.Splits[0].AccountName)
		if account1 != nil && account1.Type == "E" {
			labels[0] = "expense category"
			labels[1] = "payment account"
		} else {
			labels[0] = "payment account"
			labels[1] = "expense category"
		}

	case "Income":
		// Find which is the revenue account
		account1, _ := logic.GetAccountByName(detail.Splits[0].AccountName)
		if account1 != nil && account1.Type == "R" {
			labels[0] = "income source"
			labels[1] = "receiving account"
		} else {
			labels[0] = "receiving account"
			labels[1] = "income source"
		}

	case "Transfer":
		// Determine by amount sign
		if detail.Splits[0].Amount > 0 {
			labels[0] = "receiving account"
			labels[1] = "source account"
		} else {
			labels[0] = "source account"
			labels[1] = "receiving account"
		}

	case "Opening":
		labels[0] = "account"
		labels[1] = "opening balance"

	default:
		labels[0] = "account"
		labels[1] = "account"
	}

	return labels
}

// filterAccountsForChange returns accounts suitable for the given split change
func filterAccountsForChange(accounts []*store.Account, detail *accounting.TransactionDetail, txType string, splitIndex int) []*store.Account {
	var filtered []*store.Account

	// Get the account type we're replacing
	currentAccount, err := logic.GetAccountByName(detail.Splits[splitIndex].AccountName)
	if err != nil {
		return accounts // Fallback to all accounts if error
	}

	switch txType {
	case "Expense":
		if currentAccount.Type == "E" {
			// Changing expense category - only show Expense accounts
			for _, acc := range accounts {
				if acc.Type == "E" {
					filtered = append(filtered, acc)
				}
			}
		} else {
			// Changing payment account - only show Assets and Liabilities
			for _, acc := range accounts {
				if acc.Type == "A" || acc.Type == "L" {
					filtered = append(filtered, acc)
				}
			}
		}

	case "Income":
		if currentAccount.Type == "R" {
			// Changing income source - only show Revenue accounts
			for _, acc := range accounts {
				if acc.Type == "R" {
					filtered = append(filtered, acc)
				}
			}
		} else {
			// Changing receiving account - only show Assets and Liabilities
			for _, acc := range accounts {
				if acc.Type == "A" || acc.Type == "L" {
					filtered = append(filtered, acc)
				}
			}
		}

	case "Transfer":
		// Both sides must be Assets or Liabilities
		for _, acc := range accounts {
			if acc.Type == "A" || acc.Type == "L" {
				filtered = append(filtered, acc)
			}
		}

	default:
		// For other types, return all accounts
		filtered = accounts
	}

	return filtered
}

// changeAmount allows quick amount editing for simple transactions
// Automatically adjusts both sides to maintain balance
func changeAmount(detail *accounting.TransactionDetail) error {
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
	currentAmountStr := logic.FormatAmountFromCents(currentAmount)

	pterm.DefaultSection.Printf("Current amount: %s %s", currentAmountStr, detail.Splits[0].Currency)
	tableData := pterm.TableData{
		{"#", "Account", "Amount"},
	}

	var balance int64
	for i, split := range detail.Splits {
		amount := logic.FormatAmountFromCents(split.Amount)
		sign := "+"
		if split.Amount < 0 {
			sign = "-"
			amount = logic.FormatAmountFromCents(-split.Amount)
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

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	// Input new amount
	var newAmountStr string
	amountPrompt := &survey.Input{
		Message: "Enter new amount (positive number):",
		Default: currentAmountStr,
	}
	if err := survey.AskOne(amountPrompt, &newAmountStr, surveyOpts...); err != nil {
		return err
	}

	newAmount, err := logic.ParseAmountToCents(newAmountStr)
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
		logic.FormatAmountFromCents(newAmount),
		detail.Splits[0].Currency)
	printSeparator()
	return nil
}

func editSplits(detail *accounting.TransactionDetail) error {
	for {
		// Display current splits with balance
		displayTransactionDetail(detail)

		var action string
		actionPrompt := &survey.Select{
			Message: "Splits Editor:",
			Options: []string{
				"Add Split",
				"Edit Split",
				"Delete Split",
				"Done (return to main menu)",
			},
		}
		if err := survey.AskOne(actionPrompt, &action, surveyOpts...); err != nil {
			return err
		}

		switch action {
		case "Add Split":
			if err := addSplit(detail); err != nil {
				pterm.Error.Printf("Failed to add split: %v\n", err)
			}

		case "Edit Split":
			if err := editOneSplit(detail); err != nil {
				pterm.Error.Printf("Failed to edit split: %v\n", err)
			}

		case "Delete Split":
			if err := deleteSplit(detail); err != nil {
				pterm.Error.Printf("Failed to delete split: %v\n", err)
			}

		case "Done (return to main menu)":
			return nil
		}
	}
}

func addSplit(detail *accounting.TransactionDetail) error {
	// Select account
	accounts, err := logic.GetAllAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	var accountNames []string
	for _, acc := range accounts {
		accountNames = append(accountNames, acc.Name)
	}

	var selectedAccount string
	accountPrompt := &survey.Select{
		Message: "Select account:",
		Options: accountNames,
	}
	if err := survey.AskOne(accountPrompt, &selectedAccount, surveyOpts...); err != nil {
		return err
	}

	// Get account details
	account, err := logic.GetAccountByName(selectedAccount)
	if err != nil {
		return err
	}

	// Input amount
	var amountStr string
	amountPrompt := &survey.Input{
		Message: "Amount (use negative for credit):",
	}
	if err := survey.AskOne(amountPrompt, &amountStr, surveyOpts...); err != nil {
		return err
	}

	amount, err := logic.ParseAmountToCents(amountStr)
	if err != nil {
		return err
	}

	// Input memo
	var memo string
	memoPrompt := &survey.Input{
		Message: "Memo (optional):",
	}
	if err := survey.AskOne(memoPrompt, &memo, surveyOpts...); err != nil {
		return err
	}

	// Add split to detail
	newSplit := accounting.SplitDetail{
		ID:          0, // New split
		AccountID:   account.ID,
		AccountName: account.Name,
		Amount:      amount,
		Currency:    account.Currency,
		Memo:        memo,
	}
	detail.Splits = append(detail.Splits, newSplit)

	pterm.Success.Println("Split added")
	printSeparator()
	return nil
}

func editOneSplit(detail *accounting.TransactionDetail) error {
	if len(detail.Splits) == 0 {
		return fmt.Errorf("no splits to edit")
	}

	// Select split to edit
	var splitOptions []string
	for i, split := range detail.Splits {
		amount := logic.FormatAmountFromCents(split.Amount)
		splitOptions = append(splitOptions, fmt.Sprintf("#%d: %s (%s %s)", i+1, split.AccountName, amount, split.Currency))
	}

	var selectedSplit string
	splitPrompt := &survey.Select{
		Message: "Select split to edit:",
		Options: splitOptions,
	}
	if err := survey.AskOne(splitPrompt, &selectedSplit, surveyOpts...); err != nil {
		return err
	}

	// Find split index
	var splitIndex int
	fmt.Sscanf(selectedSplit, "#%d:", &splitIndex)
	splitIndex-- // Convert to 0-based index

	split := &detail.Splits[splitIndex]

	// Edit account
	accounts, err := logic.GetAllAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	var accountNames []string
	for _, acc := range accounts {
		accountNames = append(accountNames, acc.Name)
	}

	var selectedAccount string
	accountPrompt := &survey.Select{
		Message: "Account:",
		Options: accountNames,
		Default: split.AccountName,
	}
	if err := survey.AskOne(accountPrompt, &selectedAccount, surveyOpts...); err != nil {
		return err
	}

	account, err := logic.GetAccountByName(selectedAccount)
	if err != nil {
		return err
	}

	// Edit amount
	currentAmount := logic.FormatAmountFromCents(split.Amount)
	var amountStr string
	amountPrompt := &survey.Input{
		Message: "Amount:",
		Default: currentAmount,
	}
	if err := survey.AskOne(amountPrompt, &amountStr, surveyOpts...); err != nil {
		return err
	}

	amount, err := logic.ParseAmountToCents(amountStr)
	if err != nil {
		return err
	}

	// Edit memo
	var memo string
	memoPrompt := &survey.Input{
		Message: "Memo:",
		Default: split.Memo,
	}
	if err := survey.AskOne(memoPrompt, &memo, surveyOpts...); err != nil {
		return err
	}

	// Update split
	split.AccountID = account.ID
	split.AccountName = account.Name
	split.Amount = amount
	split.Currency = account.Currency
	split.Memo = memo

	pterm.Success.Println("Split updated")
	printSeparator()
	return nil
}

func deleteSplit(detail *accounting.TransactionDetail) error {
	if len(detail.Splits) <= 2 {
		return fmt.Errorf("cannot delete: transaction must have at least 2 splits")
	}

	// Select split to delete
	var splitOptions []string
	for i, split := range detail.Splits {
		amount := logic.FormatAmountFromCents(split.Amount)
		splitOptions = append(splitOptions, fmt.Sprintf("#%d: %s (%s %s)", i+1, split.AccountName, amount, split.Currency))
	}

	var selectedSplit string
	splitPrompt := &survey.Select{
		Message: "Select split to delete:",
		Options: splitOptions,
	}
	if err := survey.AskOne(splitPrompt, &selectedSplit, surveyOpts...); err != nil {
		return err
	}

	// Find split index
	var splitIndex int
	fmt.Sscanf(selectedSplit, "#%d:", &splitIndex)
	splitIndex-- // Convert to 0-based index

	// Confirm deletion
	var confirm bool
	confirmPrompt := &survey.Confirm{
		Message: fmt.Sprintf("Delete split: %s?", detail.Splits[splitIndex].AccountName),
		Default: false,
	}
	if err := survey.AskOne(confirmPrompt, &confirm, surveyOpts...); err != nil {
		return err
	}

	if !confirm {
		pterm.Info.Println("Deletion cancelled")
		return nil
	}

	// Delete split
	detail.Splits = append(detail.Splits[:splitIndex], detail.Splits[splitIndex+1:]...)

	pterm.Success.Println("Split deleted")
	printSeparator()
	return nil
}

func convertToSplitInputs(splits []accounting.SplitDetail) []accounting.TransactionSplitInput {
	var inputs []accounting.TransactionSplitInput
	for _, split := range splits {
		inputs = append(inputs, accounting.TransactionSplitInput{
			ID:          split.ID,
			AccountName: split.AccountName,
			AccountID:   split.AccountID,
			Amount:      split.Amount,
			Currency:    split.Currency,
			Memo:        split.Memo,
		})
	}
	return inputs
}

func saveTransactionChanges(txID int64, detail *accounting.TransactionDetail) error {
	splits := convertToSplitInputs(detail.Splits)
	return logic.UpdateTransactionComplete(
		txID,
		detail.Description,
		detail.Timestamp,
		detail.Status,
		splits,
	)
}
