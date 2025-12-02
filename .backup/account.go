/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/hance08/kea/internal/store"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	accountListType       string
	accountListShowHidden bool
)

// accountCmd represents the account command
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "It can create, edit, delete account and show the list of all accounts.",
	Long:  `It can create, edit, delete account and show the list of all accounts.`,
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all accounts with their balances",
	Long: `List all accounts in the system with their current balances.

You can filter by account type or show hidden accounts.`,
	Example: `  # List all accounts
  kea account list

  # List only asset accounts
  kea account list -t A

  # Show hidden accounts
  kea account list --show-hidden`,
	RunE: runAccountList,
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.AddCommand(accountListCmd)

	accountListCmd.Flags().StringVarP(&accountListType, "type", "t", "", "Filter accounts by type (A, L, C, R, E)")
	accountListCmd.Flags().BoolVar(&accountListShowHidden, "show-hidden", false, "Show hidden accounts")
}

func runAccountList(cmd *cobra.Command, args []string) error {
	var accounts []*store.Account
	var err error

	if accountListType != "" {
		accounts, err = logic.GetAccountsByType(accountListType)
	} else {
		accounts, err = logic.GetAllAccounts()
	}

	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	if !accountListShowHidden {
		accounts = filterHiddenAccounts(accounts)
	}

	displayAccountsList(accounts)

	return nil
}

func filterHiddenAccounts(accounts []*store.Account) []*store.Account {
	var filtered []*store.Account
	for _, acc := range accounts {
		if !acc.IsHidden {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}

func displayAccountsList(accounts []*store.Account) {
	headers := []string{"Name", "Type", "Balance"}

	tableData := pterm.TableData{headers}

	for _, acc := range accounts {
		balance, _ := logic.GetAccountBalanceFormatted(acc.ID)
		balanceWithCurrency := fmt.Sprintf("%s %s", balance, acc.Currency)

		// Apply color based on account type
		var coloredAccount, coloredType, coloredBalance string
		switch acc.Type {
		case "A", "R": // Assets, Revenue - Green (positive)
			coloredType = pterm.Green(acc.Type)
			coloredBalance = pterm.Green(balanceWithCurrency)
			coloredAccount = pterm.Green(acc.Name)
		case "L", "E": // Liabilities, Expenses - Red (caution)
			coloredType = pterm.Red(acc.Type)
			coloredBalance = pterm.Red(balanceWithCurrency)
			coloredAccount = pterm.Red(acc.Name)
		case "C": // Capital/Equity - Gray (system)
			coloredType = pterm.Gray(acc.Type)
			coloredBalance = pterm.Gray(balanceWithCurrency)
			coloredAccount = pterm.Gray(acc.Name)

		default:
			coloredType = acc.Type
			coloredBalance = balance
		}
		row := []string{coloredAccount, coloredType, coloredBalance}
		tableData = append(tableData, row)
	}

	pterm.DefaultSection.Printf("Account List")
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	pterm.Info.Printf("Total: %d accounts\n", len(accounts))
}
