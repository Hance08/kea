package transaction

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type listFlags struct {
	Account string
	Limit   int
}

type listRunner struct {
	svc   *service.Service
	flags *listFlags
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List recent transactions (alias: tls)",
		Long: `List recent transactions from your accounting records.

This command displays a table of transactions with their details including
date, type, account, description, amount, and status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &listRunner{
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

func (r *listRunner) Run() error {
	var transactions []*model.Transaction
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
		detail, err := r.svc.Transaction.GetTransactionByID(tx.ID)
		if err != nil {
			pterm.Warning.Printf("Skipping transaction %d: %v\n", tx.ID, err)
			continue
		}

		txTypeEnum, err := r.svc.Transaction.DetermineType(detail.Splits)
		txType := string(txTypeEnum)
		if err != nil {
			txType = "-"
		}

		accountName, err := r.svc.Transaction.GetDisplayAccount(detail.Splits, txType)
		if err != nil {
			accountName = "-"
		}

		amountCents, currency := r.svc.Transaction.GetDisplayAmount(detail.Splits)

		amountFloat := float64(amountCents) / 100.0
		amountStr := fmt.Sprintf("%.2f %s", amountFloat, currency)

		date := time.Unix(tx.Timestamp, 0).Format("2006-01-02")
		status := "Cleared"
		if tx.Status == 0 {
			status = "Pending"
		}

		viewItems = append(viewItems, views.TransactionListItem{
			ID:          tx.ID,
			Date:        date,
			Type:        txType,
			Account:     accountName,
			Description: tx.Description,
			Amount:      amountStr,
			Status:      status,
		})
	}

	if err := views.NewTransactionListView().Render(viewItems, r.flags.Limit); err != nil {
		return err
	}

	return nil
}
