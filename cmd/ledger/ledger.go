package ledger

import (
	"io/fs"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)

func NewLedgerCmd(registry *internalled.Registry, migrations fs.FS, appDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Manage ledgers (independent databases)",
		Long:  `Create, list, switch between, and remove named ledgers.`,
	}
	cmd.AddCommand(NewListCmd(registry))
	// add, switch, remove subcommands added in Tasks 5-7
	return cmd
}
