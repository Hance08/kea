package account

import (
	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

func NewAccountCmd(svc *service.Service) *cobra.Command {
	accountCmd := &cobra.Command{
		Use:     "account",
		Aliases: []string{"acc", "a"},
		Short:   "It can create, edit, delete account and show the list of all accounts.",
		Long:    `It can create, edit, delete account and show the list of all accounts.`,
	}

	accountCmd.AddCommand(NewCreateCmd(svc))
	accountCmd.AddCommand(NewListCmd(svc))

	return accountCmd
}
