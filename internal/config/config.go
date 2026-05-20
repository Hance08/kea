// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package config

type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	Defaults     DefaultsConfig `mapstructure:"defaults"`
	Server       ServerConfig   `mapstructure:"server"`
	ConfigPath   string         `mapstructure:"-"`
	ActiveLedger string         `mapstructure:"-"` // name of the active ledger, set at startup
}

type ServerConfig struct {
	Host        string   `mapstructure:"host"`
	Port        int      `mapstructure:"port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
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
		Server: ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: []string{"http://localhost:5173"},
		},
	}
}
