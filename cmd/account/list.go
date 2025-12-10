/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package account

import (
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type listFlags struct {
	Type       string
	ShowHidden bool
}

type ListCommandRunner struct {
	svc   *service.AccountingService
	flags *listFlags
}

func NewListCmd(svc *service.AccountingService) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all accounts with their balances",
		Long: `List all accounts in the system with their current balances.
You can filter by account type or show hidden accounts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &ListCommandRunner{
				svc:   svc,
				flags: flags,
			}
			return runner.Run()
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Filter accounts by type (A, L, C, R, E)")
	cmd.Flags().BoolVar(&flags.ShowHidden, "show-hidden", false, "Show hidden accounts")

	return cmd
}

func (r *ListCommandRunner) Run() error {

	var accounts []*store.Account
	var err error

	if r.flags.Type != "" {
		accounts, err = r.svc.GetAccountsByType(r.flags.Type)
	} else {
		accounts, err = r.svc.GetAllAccounts()
	}

	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	if !r.flags.ShowHidden {
		accounts = filterHiddenAccounts(accounts)
	}

	r.displayAccountsList(accounts)

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

func (r *ListCommandRunner) displayAccountsList(accounts []*store.Account) {
	headers := []string{"Name", "Type", "Balance"}
	tableData := pterm.TableData{headers}

	for _, acc := range accounts {
		balance, _ := r.svc.GetAccountBalanceFormatted(acc.ID)
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
