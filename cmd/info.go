package cmd

import (
	"os"
	"path/filepath"

	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type InfoProvider interface {
	Config() *config.Config
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

func NewInfoCmd(svc *service.Service) *cobra.Command {
	flags := &infoFlags{}
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &infoRunner{svc: svc, view: views.NewSystemInfoView(), json: flags.JSON}
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

	rawDBPath := r.svc.Config().Database.Path
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
		ActiveLedger:    r.svc.Config().ActiveLedger,
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
