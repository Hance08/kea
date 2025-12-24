package transaction

import (
	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

func NewTransactionCmd(svc *service.Service) *cobra.Command {
	txCmd := &cobra.Command{
		Use:     "transaction",
		Short:   "Manage transactions",
		Long:    "Manage transactions: view details, delete, or modify transaction status.",
		Aliases: []string{"tx", "t"},
	}

	txCmd.AddCommand(NewListCmd(svc))
	txCmd.AddCommand(NewShowCmd(svc))
	txCmd.AddCommand(NewDeleteCmd(svc))
	txCmd.AddCommand(NewClearCmd(svc))
	txCmd.AddCommand(NewEditCmd(svc))

	return txCmd
}
