package cmd

import (
	"os"

	"github.com/hance08/kea/internal/ui/views"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		Run: func(cmd *cobra.Command, args []string) {
			runInfo()
		},
	}
}

func runInfo() {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = "(None, using defaults)"
	}

	rawDBPath := viper.GetString("database.path")
	expandedDBPath, _ := expandPath(rawDBPath)

	dbExists := false
	if _, err := os.Stat(expandedDBPath); os.IsNotExist(err) {
		dbExists = true
	}

	items := views.SystemInfoItem{
		ConfigPath:      configPath,
		DBPath:          expandedDBPath,
		DBExists:        dbExists,
		DefaultCurrency: viper.GetString("defaults.currency"),
		AppDataDir:      getAppDataDirOrPanic(),
	}

	views.RenderSystemInfo(items)
}

func getAppDataDirOrPanic() string {
	dir, err := getAppDataDir()
	if err != nil {
		return "Unknown"
	}
	return dir
}
