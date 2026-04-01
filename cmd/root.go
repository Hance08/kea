package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hance08/kea/cmd/account"
	"github.com/hance08/kea/cmd/transaction"
	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

const defaultConfigTemplate = `# kea configuration file

database:
  # Path to the SQLite database file.
  path: ""

defaults:
  # Default currency code (ISO 4217), e.g. USD, TWD, JPY, EUR
  currency: ""
`

func Execute(migrations fs.FS) {
	pterm.Error.Prefix = pterm.Prefix{
		Text:  " ERROR ",
		Style: pterm.NewStyle(pterm.BgLightRed, pterm.FgBlack),
	}

	// rootCmd and the --config flag must be created before initConfig() so that
	// ParseFlags() can populate cfgFile prior to reading the configuration file.
	rootCmd := &cobra.Command{
		Use:           "kea",
		Short:         "kea is a CLI/TUI based personal accounting tool",
		Long:          `kea is a CLI/TUI based personal accounting tool`,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "set the config file path")
	rootCmd.PersistentFlags().BoolVar(new(bool), "no-color", false, "disable colored output (machine-friendly)")

	// Parse the persistent flags (--config / -c) before calling initConfig so
	// that a user-supplied config path is respected.
	_ = rootCmd.ParseFlags(os.Args[1:])

	noColor, _ := rootCmd.PersistentFlags().GetBool("no-color")
	configureOutput(noColor)

	cfg, err := initConfig(cfgFile)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	exitCode := func() int {
		application, cleanup, err := app.NewApp(cfg, migrations)
		if err != nil {
			pterm.Error.Println(err)
			return 1
		}
		defer cleanup()

		if err := ensureCurrency(cfg); err != nil {
			pterm.Error.Println(err)
			return 1
		}

		if err := initSysAcc(application.Service, cfg); err != nil {
			pterm.Error.Println(err)
			return 1
		}

		rootCmd.AddCommand(account.NewAccountCmd(application.Service))
		rootCmd.AddCommand(transaction.NewTransactionCmd(application.Service))

		rootCmd.AddCommand(NewAddCmd(application.Service))
		rootCmd.AddCommand(NewInfoCmd(application.Service))
		rootCmd.AddCommand(NewReportCmd(application.Service))

		if err := rootCmd.Execute(); err != nil {
			pterm.Error.Println(capitalize(err.Error()))
			return 1
		}
		return 0
	}()
	os.Exit(exitCode)
}

func initSysAcc(svc *service.Service, cfg *config.Config) error {
	_, err := svc.Account().GetAccountByName("Equity:OpeningBalances")
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrRecordNotFound) {
		return fmt.Errorf("failed to check system account: %w", err)
	}

	_, err = svc.Account().CreateAccount(
		"Equity:OpeningBalances",
		model.TypeEquity,
		cfg.Defaults.Currency,
		"Opening Balances (System Account)",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create system account: %w", err)
	}

	return nil
}

func ensureCurrency(cfg *config.Config) error {
	if cfg.Defaults.Currency != "" {
		return nil
	}

	return initWizard(cfg)
}

func initConfig(cfgFile string) (*config.Config, error) {
	var appDir string
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		var err error
		appDir, err = app.GetAppDataDir()
		if err != nil {
			return nil, fmt.Errorf("error getting app dir: %w", err)
		}

		viper.AddConfigPath(appDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")

		if err := createDefaultConfig(appDir); err != nil {
			return nil, fmt.Errorf("failed to ensure config file: %w", err)
		}
	}

	viper.SetEnvPrefix("KEA")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // allow using environment variables to override

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := config.NewDefault()
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	cfg.ConfigPath = viper.ConfigFileUsed()

	// Expand ~ in the database path so that paths like "~/mydata/kea.db"
	// are resolved before being handed to the SQLite layer.
	if cfg.Database.Path != "" {
		expanded, err := expandPath(cfg.Database.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid database path: %w", err)
		}
		cfg.Database.Path = expanded
	}

	return cfg, nil
}

func initWizard(cfg *config.Config) error {
	currency, err := prompts.PromptInitCurrency("USD")
	if err != nil {
		return err
	}

	cfg.Defaults.Currency = currency
	viper.Set("defaults.currency", currency)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config to file: %w", err)
	}

	pterm.Success.Printf("Configuration saved. Default currency set to: %s\n", currency)

	return nil
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
			return filepath.Join(home, path[2:]), nil
		}
		// ~username/... syntax is not supported
		return "", fmt.Errorf("unsupported path format %q: ~username syntax is not supported, use an absolute path or ~/", path)
	}
	return path, nil
}

func createDefaultConfig(appDir string) error {
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(appDir, "config.yaml")

	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(defaultConfigTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func configureOutput(noColorFlag bool) {
	if noColorFlag || os.Getenv("NO_COLOR") != "" {
		pterm.DisableStyling()
		return
	}

	pterm.EnableStyling()
}
