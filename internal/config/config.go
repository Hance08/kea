package config

type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	Defaults     DefaultsConfig `mapstructure:"defaults"`
	ConfigPath   string         `mapstructure:"-"`
	ActiveLedger string         `mapstructure:"-"` // name of the active ledger, set at startup
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type DefaultsConfig struct {
	Currency string `mapstructure:"currency"`
}

func NewDefault() *Config {
	return &Config{
		Database: DatabaseConfig{Path: ""},
		Defaults: DefaultsConfig{},
	}
}
