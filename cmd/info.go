package cmd

import (
	"os"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/spf13/cobra"
)

type InfoCommandRunner struct {
	svc *service.Service
}

func NewInfoCmd(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &InfoCommandRunner{
				svc: svc,
			}

			return runner.Run()
		},
	}
}

func (r *InfoCommandRunner) Run() error {
	configPath := r.svc.Config.ConfigPath
	if configPath == "" {
		configPath = "(None, using defaults)"
	}

	rawDBPath := r.svc.Config.Database.Path
	expandedDBPath, _ := expandPath(rawDBPath)

	dbExists := false
	if _, err := os.Stat(expandedDBPath); os.IsNotExist(err) {
		dbExists = true
	}

	items := views.SystemInfoItem{
		ConfigPath:      configPath,
		DBPath:          expandedDBPath,
		DBExists:        dbExists,
		DefaultCurrency: r.svc.Config.Defaults.Currency,
		AppDataDir:      getAppDataDirOrPanic(),
	}

	views.RenderSystemInfo(items)
	return nil
}

func getAppDataDirOrPanic() string {
	dir, err := getAppDataDir()
	if err != nil {
		return "Unknown"
	}
	return dir
}
