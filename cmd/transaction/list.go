package transaction

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

//TODO: Efficiency optimize

type ListView interface {
	ShowWarning(format string, a ...interface{})
	Render(items []views.TransactionListItem, limit int) error
}

type ListProvider interface {
	GetTransactionHistory(accountName string, limit int) ([]*model.Transaction, error)
	GetRecentTransactions(limit int) ([]*model.Transaction, error)
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
	DetermineType(splits []model.SplitDetail) (model.TransactionType, error)
	GetDisplayAccount(splits []model.SplitDetail, txType string) (string, error)
	GetDisplayOffsetAccount(splits []model.SplitDetail, txType string, primaryAccount string) (string, error)
	GetDisplayAmount(splits []model.SplitDetail) (int64, string)
}

type listFlags struct {
	Account string
	Limit   int
	JSON    bool
}

type listRunner struct {
	svc   ListProvider
	view  ListView
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
				svc:   svc.Transaction(),
				view:  views.NewTransactionListView(),
				flags: flags,
			}
			return runner.Run()
		},
	}

	cmd.Flags().StringVarP(&flags.Account, "account", "a", "", "Filter transactions by account name")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "l", 20, "Maximum number of transactions to display")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")

	return cmd
}

func (r *listRunner) Run() error {
	// 1. Fetch Data
	transactions, err := r.fetchTransactions()
	if err != nil {
		return err
	}

	// 2. Transform Data (Model -> View Model)
	viewItems := r.buildViewItems(transactions)

	// 3. Render
	if r.flags.JSON {
		jsonItems := make([]views.JSONTxListItem, len(viewItems))
		for i, item := range viewItems {
			jsonItems[i] = views.ToJSONTxListItem(item)
		}
		return views.WriteJSON(jsonItems)
	}
	return r.view.Render(viewItems, r.flags.Limit)
}

func (r *listRunner) fetchTransactions() ([]*model.Transaction, error) {
	if r.flags.Account != "" {
		return r.svc.GetTransactionHistory(r.flags.Account, r.flags.Limit)
	}
	return r.svc.GetRecentTransactions(r.flags.Limit)
}

func (r *listRunner) buildViewItems(transactions []*model.Transaction) []views.TransactionListItem {
	var viewItems []views.TransactionListItem

	for _, tx := range transactions {
		detail, err := r.svc.GetTransactionByID(tx.ID)
		if err != nil {
			r.view.ShowWarning("Skipping transaction %d: %v\n", tx.ID, err)
			continue
		}

		viewItems = append(viewItems, r.convertToViewItem(tx, detail))
	}
	return viewItems
}

func (r *listRunner) convertToViewItem(tx *model.Transaction, detail *model.TransactionDetail) views.TransactionListItem {
	// 1. Determine Type
	txTypeEnum, err := r.svc.DetermineType(detail.Splits)
	txType := string(txTypeEnum)
	if err != nil {
		txType = "-"
	}

	accountName, err := r.svc.GetDisplayAccount(detail.Splits, txType)
	if err != nil {
		accountName = "-"
	}

	offsetAccount, err := r.svc.GetDisplayOffsetAccount(detail.Splits, txType, accountName)
	if err != nil {
		offsetAccount = "-"
	}

	amountCents, currency := r.svc.GetDisplayAmount(detail.Splits)
	amountFloat := float64(amountCents) / 100.0
	amountStr := fmt.Sprintf("%.2f", amountFloat)

	date := time.Unix(tx.Timestamp, 0).Format("2006-01-02")

	status := "Cleared"
	if tx.Status == model.StatusPending {
		status = "Pending"
	}

	return views.TransactionListItem{
		ID:          tx.ID,
		Date:        date,
		Type:        txType,
		Account:     accountName,
		Offset:      offsetAccount,
		Description: tx.Description,
		Amount:      amountStr,
		Currency:    currency,
		Status:      status,
	}
}
