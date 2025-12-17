package config

type Config struct {
	Database   DatabaseConfig `mapstructure:"database"`
	Defaults   DefaultsConfig `mapstructure:"defaults"`
	ConfigPath string         `mapstructure:"-"`
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
		Defaults: DefaultsConfig{Currency: "USD"},
	}
}
