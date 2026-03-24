package cmd

import (
	"os"
	"path/filepath"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type SystemInfoView interface {
	Render(data views.SystemInfo) error
}

type infoRunner struct {
	svc     *service.Service
	view    SystemInfoView
	jsonOut bool
}

func NewInfoCmd(svc *service.Service) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &infoRunner{svc: svc, view: views.NewSystemInfoView(), jsonOut: jsonOut}
			return runner.Run()
		},
	}
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output as JSON")
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
		DBPath:          expandedDBPath,
		DBExists:        dbExists,
		DefaultCurrency: r.svc.Config().Defaults.Currency,
		AppDataDir:      getAppDataDirOrPanic(),
	}

	if r.jsonOut {
		return views.WriteJSON(views.ToJSONSystemInfo(info))
	}
	return r.view.Render(info)
}

func getAppDataDirOrPanic() string {
	dir, err := getAppDataDir()
	if err != nil {
		return "Unknown"
	}
	return dir
}
