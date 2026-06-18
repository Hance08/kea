// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"errors"
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

			// Start the registry watcher so external `kea ledger switch` calls
			// made while the server is running cause this server to swap stores.
			// The watcher exits when ctx is cancelled; the app's cleanup also
			// calls StopWatch defensively.
			go func() {
				if err := application.Registry.Watch(cmd.Context()); err != nil &&
					!errors.Is(err, context.Canceled) {
					logger.Error("registry watcher exited", "err", err)
				}
			}()

			srv := api.NewServer(
				application.Config(),
				application.Service,
				application.Registry,
				migrationFS,
				appDir,
				application.SwitchLedger,
				func() error {
					return saveDisplayHideDecimals(application.Config())
				},
				logger,
			)
			return srv.Run(cmd.Context())
		},
	}
}
