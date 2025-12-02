/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package transaction

import (
	"github.com/hance08/kea/internal/logic/accounting"
	"github.com/spf13/cobra"
)

var (
	logic *accounting.AccountingLogic
)

// TransactionCmd represents the transaction command
var TransactionCmd = &cobra.Command{
	Use:   "transaction",
	Short: "Manage transactions",
	Long:  "Manage transactions: view details, delete, or modify transaction status.",
}

func init() {
	TransactionCmd.AddCommand(showCmd)
	TransactionCmd.AddCommand(deleteCmd)
	TransactionCmd.AddCommand(clearCmd)
	TransactionCmd.AddCommand(editCmd)
}

// SetDependencies allows root command to inject dependencies
func SetDependencies(l *accounting.AccountingLogic) {
	logic = l
}
