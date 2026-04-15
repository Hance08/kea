package ledger

import (
	"fmt"
	"io"
	"os"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)

type listRunner struct {
	registry *internalled.Registry
	out      io.Writer
}

func NewListCmd(registry *internalled.Registry) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List all registered ledgers",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &listRunner{registry: registry, out: os.Stdout}
			return runner.Run()
		},
	}
}

func (r *listRunner) Run() error {
	names := r.registry.Names()
	if len(names) == 0 {
		fmt.Fprintln(r.out, "No ledgers registered. Run: kea ledger add <name>")
		return nil
	}
	activeName := r.registry.ActiveName()
	fmt.Fprintf(r.out, "  %-20s  %s\n", "NAME", "PATH")
	for _, name := range names {
		entry, _ := r.registry.EntryFor(name)
		marker := "  "
		if name == activeName {
			marker = "* "
		}
		fmt.Fprintf(r.out, "%s%-20s  %s\n", marker, name, entry.Path)
	}
	return nil
}
