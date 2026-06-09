// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"os"
	"path/filepath"

	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type InfoProvider interface {
	Config() *config.Config
	RuntimeState() app.RuntimeState
}

type SystemInfoView interface {
	Render(data views.SystemInfo) error
}

type infoFlags struct {
	JSON    bool
}

type infoRunner struct {
	svc  InfoProvider
	view SystemInfoView
	json bool
}

func NewInfoCmd(application *app.App) *cobra.Command {
	flags := &infoFlags{}
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &infoRunner{svc: application, view: views.NewSystemInfoView(), json: flags.JSON}
			return runner.Run()
		},
	}
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")
	return cmd
}

func (r *infoRunner) Run() error {
	configPath := r.svc.Config().ConfigPath
	if configPath == "" {
		configPath = "(None, using defaults)"
	}

	rt := r.svc.RuntimeState()
	rawDBPath := rt.DatabasePath
	if rawDBPath == "" {
		appDir := getAppDataDirOrPanic()
		rawDBPath = filepath.Join(appDir, "kea.db")
	}
	expandedDBPath, _ := expandPath(rawDBPath)

	dbExists := false
	if _, err := os.Stat(expandedDBPath); err == nil {
		dbExists = true
	}

	info := views.SystemInfo{
		ConfigPath:      configPath,
		ActiveLedger:    rt.ActiveLedger,
		DBPath:          expandedDBPath,
		DBExists:        dbExists,
		DefaultCurrency: r.svc.Config().Defaults.Currency,
		AppDataDir:      getAppDataDirOrPanic(),
	}

	if r.json {
		return views.WriteJSON(views.ToJSONSystemInfo(info))
	}
	return r.view.Render(info)
}

func getAppDataDirOrPanic() string {
	dir, err := app.GetAppDataDir()
	if err != nil {
		return "Unknown"
	}
	return dir
}
