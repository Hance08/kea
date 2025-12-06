/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package transaction

import (
	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

var (
	svc *service.AccountingService
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
func SetDependencies(s *service.AccountingService) {
	svc = s
}
