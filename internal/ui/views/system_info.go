package views

import "github.com/pterm/pterm"

type SystemInfoItem struct {
	ConfigPath      string
	DBPath          string
	DBExists        bool // true = Found, false = Not Found
	DefaultCurrency string
	AppDataDir      string
}

func RenderSystemInfo(data SystemInfoItem) error {
	dbStatus := pterm.Green("Found")
	if !data.DBExists {
		dbStatus = pterm.Red("Not Found (Will be created)")
	}

	tableData := pterm.TableData{
		{"Configuration File", data.ConfigPath},
		{"Database Path", data.DBPath},
		{"Database Status", dbStatus},
		{"Default Currency", data.DefaultCurrency},
		{"AppData Directory", data.AppDataDir},
	}

	return pterm.DefaultTable.WithData(tableData).Render()
}
