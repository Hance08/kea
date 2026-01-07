package views

import "github.com/pterm/pterm"

type SystemInfo struct {
	ConfigPath      string
	DBPath          string
	DBExists        bool // true = Found, false = Not Found
	DefaultCurrency string
	AppDataDir      string
}

type SystemInfoView struct{}

func NewSystemInfoView() *SystemInfoView {
	return &SystemInfoView{}
}

func (v *SystemInfoView) Render(info SystemInfo) error {
	dbStatus := pterm.Green("Found")
	if !info.DBExists {
		dbStatus = pterm.Red("Not Found (Will be created)")
	}

	tableData := pterm.TableData{
		{"Configuration File", info.ConfigPath},
		{"Database Path", info.DBPath},
		{"Database Status", dbStatus},
		{"Default Currency", info.DefaultCurrency},
		{"AppData Directory", info.AppDataDir},
	}

	return pterm.DefaultTable.WithData(tableData).Render()
}
