// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package ledger

import (
	"fmt"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/ui/prompts"
	"github.com/spf13/cobra"
)

type removeFlags struct {
	DeleteFile bool
	Yes        bool
}

type removeRunner struct {
	registry   *internalled.Registry
	name       string
	deleteFile bool
	yes        bool
}

func NewRemoveCmd(registry *internalled.Registry) *cobra.Command {
	flags := &removeFlags{}
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Unregister a ledger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &removeRunner{
				registry:   registry,
				name:       args[0],
				deleteFile: flags.DeleteFile,
				yes:        flags.Yes,
			}
			return runner.Run()
		},
	}
	cmd.Flags().BoolVar(&flags.DeleteFile, "delete-file", false, "also delete the database file from disk")
	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func (r *removeRunner) Run() error {
	// Capture entry before removal so we can report the path in the success message.
	entry, entryOk := r.registry.EntryFor(r.name)

	if r.deleteFile && !r.yes {
		if !entryOk {
			return fmt.Errorf("%w: %q", internalled.ErrLedgerNotFound, r.name)
		}
		confirmed, err := prompts.PromptConfirm(
			fmt.Sprintf("Delete %s?", entry.Path),
			false,
		)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := r.registry.Remove(r.name, r.deleteFile); err != nil {
		return err
	}

	if r.deleteFile {
		fmt.Printf("Ledger %q removed and database file deleted.\n", r.name)
	} else {
		fmt.Printf("Ledger %q unregistered. Database file remains at: %s\n", r.name, entry.Path)
	}
	return nil
}
