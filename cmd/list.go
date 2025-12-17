/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type listFlags struct {
	Account string
	Limit   int
}

type ListCommandRunner struct {
	svc   *service.Service
	flags *listFlags
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"tls"},
		Short:   "List recent transactions (alias: tls)",
		Long: `List recent transactions from your accounting records.

This command displays a table of transactions with their details including
date, type, account, description, amount, and status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &ListCommandRunner{
				svc:   svc,
				flags: flags,
			}
			return runner.Run()
		},
	}

	cmd.Flags().StringVarP(&flags.Account, "account", "a", "", "Filter transactions by account name")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "l", 20, "Maximum number of transactions to display")

	return cmd
}

func (r *ListCommandRunner) Run() error {
	var transactions []*store.Transaction
	var err error

	if r.flags.Account != "" {
		// List transactions for specific account
		transactions, err = r.svc.Transaction.GetTransactionHistory(r.flags.Account, r.flags.Limit)
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}
		pterm.Info.Printf("Showing transactions for account: %s\n\n", r.flags.Account)
	} else {
		// List all recent transactions
		transactions, err = r.svc.Transaction.GetRecentTransactions(r.flags.Limit)
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}
	}

	var viewItems []views.TransactionListItem

	for _, tx := range transactions {
		date := time.Unix(tx.Timestamp, 0).Format("2006-01-02")

		status := "Cleared"
		if tx.Status == 0 {
			status = "Pending"
		}

		txType, err := r.getTransactionType(tx.ID)
		if err != nil {
			txType = "-"
		}

		account, err := r.getTransactionAccount(tx.ID, txType)
		if err != nil {
			account = "-"
		}

		amount, err := r.getTransactionAmount(tx.ID)
		if err != nil {
			amount = "-"
		}

		viewItems = append(viewItems, views.TransactionListItem{
			ID:          tx.ID,
			Date:        date,
			Type:        txType,
			Account:     account,
			Description: tx.Description,
			Amount:      amount,
			Status:      status,
		})
	}

	views.NewTransactionListView().Render(viewItems, r.flags.Limit)

	return nil
}

// getTransactionAmount retrieves the main amount of a transaction (largest positive split)
func (r *ListCommandRunner) getTransactionAmount(txID int64) (string, error) {
	detail, err := r.svc.Transaction.GetTransactionByID(txID)
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
func (r *ListCommandRunner) getTransactionType(txID int64) (string, error) {
	detail, err := r.svc.Transaction.GetTransactionByID(txID)
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
		account, err := r.svc.Account.GetAccountByName(split.AccountName)
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
func (r *ListCommandRunner) getTransactionAccount(txID int64, transType string) (string, error) {
	detail, err := r.svc.Transaction.GetTransactionByID(txID)
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
			account, err := r.svc.Account.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "E" {
				return split.AccountName, nil
			}
		}

	case "Income":
		// Find and return the Revenue account (R type)
		for _, split := range detail.Splits {
			account, err := r.svc.Account.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "R" {
				return split.AccountName, nil
			}
		}

	case "Transfer":
		// Find and return the Asset account with positive amount (receiving account)
		for _, split := range detail.Splits {
			if split.Amount > 0 {
				account, err := r.svc.Account.GetAccountByName(split.AccountName)
				if err == nil && (account.Type == "A" || account.Type == "L") {
					return split.AccountName, nil
				}
			}
		}

	case "Opening":
		// For opening transactions, return the non-equity account
		for _, split := range detail.Splits {
			account, err := r.svc.Account.GetAccountByName(split.AccountName)
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
