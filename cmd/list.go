/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/store"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	listAccount string
	listLimit   int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent transactions",
	Long: `List recent transactions from your accounting records.

This command displays a table of transactions with their details including
date, type, account, description, amount, and status.`,
	Example: `  # List recent transactions
  kea list

  # List transactions for a specific account
  kea list --account "Assets:Cash"

  # Limit the number of transactions
  kea list --limit 50`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listAccount, "account", "a", "", "Filter transactions by account name")
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 20, "Maximum number of transactions to display")
}

func runList(cmd *cobra.Command, args []string) error {

	var transactions []*store.Transaction
	var err error

	if listAccount != "" {
		// List transactions for specific account
		transactions, err = logic.GetTransactionHistory(listAccount, listLimit)
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}
		pterm.Info.Printf("Showing transactions for account: %s\n\n", listAccount)
	} else {
		// List all recent transactions
		transactions, err = logic.GetRecentTransactions(listLimit)
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}
		pterm.DefaultSection.Printf("Showing recent transactions (limit: %d)", listLimit)
	}

	if len(transactions) == 0 {
		pterm.Warning.Println("No transactions found")
		return nil
	}

	// Display transactions table
	tableData := pterm.TableData{
		{"ID", "Date", "Type", "Account", "Description", "Amount", "Status"},
	}

	for _, tx := range transactions {
		date := time.Unix(tx.Timestamp, 0).Format("2006-01-02")
		status := "Cleared"
		if tx.Status == 0 {
			status = "Pending"
		}

		// Get transaction type
		txType, err := getTransactionType(tx.ID)
		if err != nil {
			txType = "-"
		}

		// Get transaction account based on type
		account, err := getTransactionAccount(tx.ID, txType)
		if err != nil {
			account = "-"
		}

		// Get transaction amount
		amount, err := getTransactionAmount(tx.ID)
		if err != nil {
			amount = "-"
		}

		// Apply color based on transaction type
		var coloredType, coloredAccount, coloredAmount string
		switch txType {
		case "Expense":
			coloredType = pterm.Red(txType)
			coloredAccount = pterm.Red(account)
			coloredAmount = pterm.Red(amount)
		case "Income":
			coloredType = pterm.Green(txType)
			coloredAccount = pterm.Green(account)
			coloredAmount = pterm.Green(amount)
		case "Transfer":
			coloredType = pterm.Blue(txType)
			coloredAccount = pterm.Blue(account)
			coloredAmount = pterm.Blue(amount)
		default:
			coloredType = txType
			coloredAccount = account
			coloredAmount = amount
		}

		tableData = append(tableData, []string{
			fmt.Sprintf("%d", tx.ID),
			date,
			coloredType,
			coloredAccount,
			tx.Description,
			coloredAmount,
			status,
		})
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	pterm.Info.Printf("Total: %d transactions\n", len(transactions))

	return nil
}

// getTransactionAmount retrieves the main amount of a transaction (largest positive split)
func getTransactionAmount(txID int64) (string, error) {
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		return "", err
	}

	if len(detail.Splits) == 0 {
		return "0.00", nil
	}

	// Find the largest positive amount (the "to" account in typical transactions)
	var maxAmount int64
	var currency string

	for _, split := range detail.Splits {
		if split.Amount > maxAmount {
			maxAmount = split.Amount
			currency = split.Currency
		}
	}

	// Format amount
	amountFloat := float64(maxAmount) / 100.0
	return fmt.Sprintf("%.2f %s", amountFloat, currency), nil
}

// getTransactionType determines the type of transaction based on account types involved
// Returns: "Expense", "Income", "Transfer", or "Other"
func getTransactionType(txID int64) (string, error) {
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		return "", err
	}

	if len(detail.Splits) == 0 {
		return "Other", nil
	}

	// Get account types for all splits
	accountTypes := make(map[string]bool)
	isOpening := false
	for _, split := range detail.Splits {
		// Get account by ID to find its type
		account, err := logic.GetAccountByName(split.AccountName)
		if err == nil {
			accountTypes[account.Type] = true
		}
		if account.Description == "Opening Balances (System Account)" {
			isOpening = true
		}
	}

	// Determine transaction type based on account types
	hasExpense := accountTypes["E"]
	hasRevenue := accountTypes["R"]
	hasAsset := accountTypes["A"]
	hasLiability := accountTypes["L"]

	if isOpening {
		return "Opening", nil
	}
	if hasExpense && (hasAsset || hasLiability) {
		return "Expense", nil
	}
	if hasRevenue && hasAsset {
		return "Income", nil
	}
	if (hasAsset || hasLiability) && !hasExpense && !hasRevenue && !isOpening {
		return "Transfer", nil
	}

	return "Other", nil
}

// getTransactionAccount returns the relevant account name based on transaction type
func getTransactionAccount(txID int64, transType string) (string, error) {
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		return "", err
	}

	if len(detail.Splits) == 0 {
		return "-", nil
	}

	switch transType {
	case "Expense":
		// Find and return the Expense account (E type)
		for _, split := range detail.Splits {
			account, err := logic.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "E" {
				return split.AccountName, nil
			}
		}

	case "Income":
		// Find and return the Revenue account (R type)
		for _, split := range detail.Splits {
			account, err := logic.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "R" {
				return split.AccountName, nil
			}
		}

	case "Transfer":
		// Find and return the Asset account with positive amount (receiving account)
		for _, split := range detail.Splits {
			if split.Amount > 0 {
				account, err := logic.GetAccountByName(split.AccountName)
				if err == nil && (account.Type == "A" || account.Type == "L") {
					return split.AccountName, nil
				}
			}
		}

	case "Opening":
		// For opening transactions, return the non-equity account
		for _, split := range detail.Splits {
			account, err := logic.GetAccountByName(split.AccountName)
			if err == nil && account.Type != "C" {
				return split.AccountName, nil
			}
		}

	case "Other":
		// For other types, return the first account with positive amount
		for _, split := range detail.Splits {
			if split.Amount > 0 {
				return split.AccountName, nil
			}
		}
	}

	// Fallback: return first account name
	if len(detail.Splits) > 0 {
		return detail.Splits[0].AccountName, nil
	}

	return "-", nil
}
