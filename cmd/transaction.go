/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

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

func init() {
	rootCmd.AddCommand(transactionCmd)
	transactionCmd.AddCommand(transactionShowCmd)
	transactionCmd.AddCommand(transactionDeleteCmd)
	transactionCmd.AddCommand(transactionClearCmd)
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
	if err := survey.AskOne(confirmPrompt, &confirmation); err != nil {
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
	return nil
}
