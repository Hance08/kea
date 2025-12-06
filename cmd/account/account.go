/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package account

import (
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/validation"
	"github.com/spf13/cobra"
)

var (
	logic     *service.AccountingService
	validator *validation.AccountValidator
)

// AccountCmd represents the account command
var AccountCmd = &cobra.Command{
	Use:   "account",
	Short: "It can create, edit, delete account and show the list of all accounts.",
	Long:  `It can create, edit, delete account and show the list of all accounts.`,
}

func init() {
	AccountCmd.AddCommand(listCmd)
	AccountCmd.AddCommand(createCmd)
}

// SetDependencies allows root command to inject dependencies
func SetDependencies(l *service.AccountingService) {
	logic = l
	validator = validation.NewAccountValidator(l)
}
