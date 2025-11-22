/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/hance08/kea/internal/store"
	"github.com/spf13/cobra"
)

var (
	listType        string
	listShowBalance bool
	listShowHidden  bool
	listTree        bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all accounts",
	Long:  `List all accounts in the database with optional filters`,
	RunE: func(cmd *cobra.Command, agrs []string) error {
		var accounts []*store.Account
		var err error

		if listType != "" {
			accounts, err = logic.GetAccountsByType(listType)
		} else {
			accounts, err = logic.GetAllAccounts()
		}

		if err != nil {
			return err
		}

		if !listShowHidden {
			accounts = filterHiddenAccounts(accounts)
		}

		if listTree {
			// TODO: displayAccountsTree(accounts, listShowBalance)
		} else {
			displayAccountsList(accounts, listShowBalance)
		}

		return nil
	},
}

func init() {
	accountCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by account type (A, L, C, R, E)")
	listCmd.Flags().BoolVarP(&listShowBalance, "balance", "b", true, "Show account balances")
	listCmd.Flags().BoolVar(&listShowHidden, "show-hidden", false, "Show hidden accounts")
	listCmd.Flags().BoolVar(&listTree, "tree", false, "Display as tree structure (not yet implemented)")
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

func displayAccountsList(accounts []*store.Account, showBalance bool) {
	fmt.Println("Account List")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("%-30s [%s] %s\n", "Name", "Type", "Currency")
	fmt.Println("--------------------------------------------------")

	for _, acc := range accounts {
		fmt.Printf("%-30s [%s] %+6s\n", acc.Name, acc.Type, acc.Currency)

		if showBalance {
			balance, _ := logic.GetAccountBalanceFormatted(acc.ID)
			fmt.Printf("  Balance: %s\n", balance)
		}
		fmt.Println("--------------------------------------------------")
	}

	fmt.Printf("Total: %d accounts\n", len(accounts))
}
