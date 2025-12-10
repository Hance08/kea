package cmd

import (
	"os"

	"github.com/pterm/pterm"
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

	// 3. 檢查資料庫檔案是否存在
	dbStatus := pterm.Green("Found")
	if _, err := os.Stat(expandedDBPath); os.IsNotExist(err) {
		dbStatus = pterm.Red("Not Found (Will be created)")
	}

	// 4. 顯示表格
	data := pterm.TableData{
		{"Configuration File", configPath},
		{"Database Path", expandedDBPath},
		{"Database Status", dbStatus},
		{"Default Currency", viper.GetString("defaults.currency")},
		{"AppData Directory", getAppDataDirOrPanic()},
	}

	pterm.DefaultTable.WithData(data).Render()
}

func getAppDataDirOrPanic() string {
	dir, err := getAppDataDir()
	if err != nil {
		return "Unknown"
	}
	return dir
}
