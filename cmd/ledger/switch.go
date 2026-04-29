// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package ledger

import (
	"fmt"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)

type switchRunner struct {
	registry *internalled.Registry
	name     string
}

func NewSwitchCmd(registry *internalled.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "Set the active ledger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &switchRunner{registry: registry, name: args[0]}
			return runner.Run()
		},
	}
}

func (r *switchRunner) Run() error {
	if err := r.registry.Switch(r.name); err != nil {
		return err
	}
	fmt.Printf("Switched to ledger %q\n", r.name)
	return nil
}
