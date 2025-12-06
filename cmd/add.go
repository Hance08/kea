/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	addDesc      string
	addAmount    string
	addFrom      string
	addTo        string
	addStatus    string
	addTimestamp string
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new transaction",
	Long: `Add a new transaction to your accounting system.

This command allows you to record financial transactions using double-entry bookkeeping.
You can use flags for quick entry or interactive mode for guided input.

Examples:
  # Interactive mode (recommended for beginners)
  kea add

  # Quick mode with flags
  kea add --desc "Buy Coffee" --amount 150 --from "Assets:Cash" --to "Expenses:Food:Coffee"
  
  # With pending status (default is cleared)
  kea add --desc "Pending cost" --amount 500 --from "Assets:Bank" --to "Expenses:Shopping" --status pending`,

	SilenceUsage: true,
	RunE:         runAddTransaction,
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&addDesc, "desc", "d", "", "Transaction description")
	addCmd.Flags().StringVarP(&addAmount, "amount", "a", "", "Transaction amount (e.g., 150 or 150.50)")
	addCmd.Flags().StringVarP(&addFrom, "from", "f", "", "Source account (where money comes from)")
	addCmd.Flags().StringVarP(&addTo, "to", "t", "", "Destination account (where money goes to)")
	addCmd.Flags().StringVarP(&addStatus, "status", "s", "cleared", "Transaction status: pending or cleared")
	addCmd.Flags().StringVar(&addTimestamp, "date", "", "Transaction date (YYYY-MM-DD), default is today")
}

func runAddTransaction(cmd *cobra.Command, args []string) error {
	var input service.TransactionInput

	// Check if using flag mode or interactive mode
	hasFlags := cmd.Flags().Changed("desc") || cmd.Flags().Changed("amount") ||
		cmd.Flags().Changed("from") || cmd.Flags().Changed("to")

	if hasFlags {
		// Flag mode: validate all required flags
		if addAmount == "" || addFrom == "" || addTo == "" {
			return fmt.Errorf("when using flags, --amount, --from, and --to are all required")
		}

		if addDesc == "" {
			addDesc = "-"
		}

		// Parse amount
		amountCents, err := svc.ParseAmountToCents(addAmount)
		if err != nil {
			return fmt.Errorf("invalid amount: %w", err)
		}

		// Validate accounts exist
		if exists, err := svc.CheckAccountExists(addFrom); err != nil || !exists {
			return fmt.Errorf("source account '%s' does not exist", addFrom)
		}
		if exists, err := svc.CheckAccountExists(addTo); err != nil || !exists {
			return fmt.Errorf("destination account '%s' does not exist", addTo)
		}

		// Parse status
		status := 1 // Default: cleared
		if strings.ToLower(addStatus) == "pending" {
			status = 0
		}

		// Parse timestamp
		var timestamp int64
		if addTimestamp != "" {
			t, err := time.Parse("2006-01-02", addTimestamp)
			if err != nil {
				return fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
			}
			timestamp = t.Unix()
		} else {
			timestamp = time.Now().Unix()
		}

		// Build transaction input
		input = service.TransactionInput{
			Timestamp:   timestamp,
			Description: addDesc,
			Status:      status,
			Splits: []service.TransactionSplitInput{
				{
					AccountName: addTo,
					Amount:      amountCents,
					Memo:        "",
				},
				{
					AccountName: addFrom,
					Amount:      -amountCents,
					Memo:        "",
				},
			},
		}
	} else {
		// Interactive mode
		var err error
		input, err = interactiveAddTransaction()
		if err != nil {
			return err
		}
	}

	// Create transaction
	txID, err := svc.CreateTransaction(input)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Success message
	pterm.Success.Printf("Transaction created successfully! (ID: %d)\n", txID)

	// Display transaction summary
	displayTransactionSummary(input)

	return nil
}

func interactiveAddTransaction() (service.TransactionInput, error) {
	var input service.TransactionInput

	// Get all accounts
	accounts, err := svc.GetAllAccounts()
	if err != nil {
		return input, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Step 1: Select transaction type
	transactionType, err := prompts.PromptTransactionType()
	if err != nil {
		return input, err
	}

	// Determine transaction mode
	var mode string
	if strings.Contains(transactionType, "Expense") {
		mode = "expense"
	} else if strings.Contains(transactionType, "Income") {
		mode = "income"
	} else {
		mode = "transfer"
	}

	// Step 2: Get description (optional)
	description, err := prompts.PromptDescription("Transaction description (optional):", false)
	if err != nil {
		return input, err
	}
	if description == "" {
		description = "-"
	}

	// Step 3: Get amount
	amountStr, err := prompts.PromptAmount(
		"Amount:",
		"Enter the amount, no need currency symbol(e.g. 150 or 150.50)",
		nil, // No custom validator, will validate after
	)
	if err != nil {
		return input, err
	}
	if amountStr == "" {
		return input, fmt.Errorf("amount is required")
	}

	amountCents, err := svc.ParseAmountToCents(amountStr)
	if err != nil {
		return input, fmt.Errorf("invalid amount format: %w", err)
	}

	// Step 4 & 5: Select accounts based on mode
	var fromAccount, toAccount string

	switch mode {
	case "expense":
		// Record Expense: From Assets/Liabilities, To Expenses
		fromAccount, err = selectAccount(accounts, []string{"A", "L"}, "Payment Source:", true)
		if err != nil {
			return input, err
		}

		toAccount, err = selectAccount(accounts, []string{"E"}, "Expense Type:", false)
		if err != nil {
			return input, err
		}

	case "income":
		// Record Income: From Revenue, To Assets
		fromAccount, err = selectAccount(accounts, []string{"R"}, "Revenue Type:", false)
		if err != nil {
			return input, err
		}

		toAccount, err = selectAccount(accounts, []string{"A"}, "Deposit To:", true)
		if err != nil {
			return input, err
		}

	case "transfer":
		// Transfer: From Assets/Liabilities, To Assets/Liabilities
		fromAccount, err = selectAccount(accounts, []string{"A", "L"}, "From Account:", true)
		if err != nil {
			return input, err
		}

		toAccount, err = selectAccount(accounts, []string{"A", "L"}, "To Account:", true)
		if err != nil {
			return input, err
		}

		// Validate: cannot transfer to the same account
		if fromAccount == toAccount {
			return input, fmt.Errorf("cannot transfer to the same account")
		}
	}

	// Step 6: Transaction status
	statusStr, err := prompts.PromptTransactionStatus("Cleared")
	if err != nil {
		return input, err
	}

	status := 1
	if statusStr == "Pending" {
		status = 0
	}

	// Step 7: Transaction date
	dateStr, err := prompts.PromptTransactionDate()
	if err != nil {
		return input, err
	}

	timestamp, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return input, fmt.Errorf("invalid date format: %w", err)
	}

	// Build transaction input
	input = service.TransactionInput{
		Timestamp:   timestamp.Unix(),
		Description: description,
		Status:      status,
		Splits: []service.TransactionSplitInput{
			{
				AccountName: toAccount,
				Amount:      amountCents,
				Memo:        "",
			},
			{
				AccountName: fromAccount,
				Amount:      -amountCents,
				Memo:        "",
			},
		},
	}

	return input, nil
}

// selectAccount filters accounts by type and displays them with optional balance
func selectAccount(accounts []*store.Account, allowedTypes []string, message string, showBalance bool) (string, error) {
	var balanceGetter func(int64) (string, error)
	if showBalance {
		balanceGetter = svc.GetAccountBalanceFormatted
	}

	return prompts.PromptAccountSelection(accounts, allowedTypes, message, showBalance, balanceGetter)
}

// getDescriptionHelp returns contextual help text based on transaction mode
func getDescriptionHelp(mode string) string {
	switch mode {
	case "expense":
		return "e.g., Buying Coffee, Grocery Shopping, Taking a Taxi"
	case "income":
		return "e.g., Salary, Bonus, Investment Income"
	case "transfer":
		return "e.g., Withdrawal, Transfer, Credit Card Repayment"
	default:
		return "enter transaction description"
	}
}

func displayTransactionSummary(input service.TransactionInput) {
	pterm.DefaultSection.Println("Transaction Summary")

	// Format timestamp
	date := time.Unix(input.Timestamp, 0).Format("2006-01-02")

	// Create table data
	tableData := pterm.TableData{
		{"Field", "Value"},
		{"Date", date},
		{"Description", input.Description},
		{"Status", getStatusString(input.Status)},
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	// Display splits
	pterm.DefaultSection.Println("Splits (Double-Entry)")

	splitsData := pterm.TableData{
		{"Account", "Amount", "Type"},
	}

	for _, split := range input.Splits {
		amountStr := fmt.Sprintf("%.2f", float64(split.Amount)/100.0)
		typeStr := "Debit"
		if split.Amount < 0 {
			typeStr = "Credit"
			amountStr = fmt.Sprintf("%.2f", float64(-split.Amount)/100.0)
		}
		splitsData = append(splitsData, []string{split.AccountName, amountStr, typeStr})
	}

	pterm.DefaultTable.WithHasHeader().WithData(splitsData).Render()

	// Verify balance
	var total int64
	for _, split := range input.Splits {
		total += split.Amount
	}
	if total == 0 {
		pterm.Success.Println("✓ Splits balance verified (total = 0)")
	} else {
		pterm.Warning.Printf("⚠ Warning: Splits do not balance (total = %d)\n", total)
	}
}

func getStatusString(status int) string {
	if status == 0 {
		return "Pending"
	}
	return "Cleared"
}
