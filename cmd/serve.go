// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"io/fs"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/hance08/kea/internal/api"
	"github.com/hance08/kea/internal/app"
)

func NewServeCmd(application *app.App, migrationFS fs.FS, appDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the local web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
			srv := api.NewServer(
				application.Config(),
				application.Service,
				application.Registry,
				migrationFS,
				appDir,
				application.SwitchLedger,
				logger,
			)
			return srv.Run(cmd.Context())
		},
	}
}
