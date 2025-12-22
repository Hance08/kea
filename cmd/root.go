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
	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cfg     *config.Config
)

func Execute(migrations fs.FS) {
	pterm.Error.Prefix = pterm.Prefix{
		Text:  " ERROR ",
		Style: pterm.NewStyle(pterm.BgLightRed, pterm.FgBlack),
	}

	if err := initConfig(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	application, cleanup, err := app.NewApp(cfg, migrations)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	defer cleanup()

	if err := initSysAcc(application.Service); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:           "kea",
		Short:         "kea is a CLI/TUI based personal accounting tool",
		Long:          `kea is a CLI/TUI based personal accounting tool`,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "set the config file path")

	rootCmd.AddCommand(account.NewAccountCmd(application.Service))
	rootCmd.AddCommand(transaction.NewTransactionCmd(application.Service))

	rootCmd.AddCommand(NewAddCmd(application.Service))
	rootCmd.AddCommand(NewInfoCmd(application.Service))
	rootCmd.AddCommand(NewAccListCmd(application.Service))
	rootCmd.AddCommand(NewTxListCmd(application.Service))
	rootCmd.AddCommand(NewReportCmd())

	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		errMsg := err.Error()
		displayMsg := capitalize(errMsg)

		pterm.Error.Println(displayMsg)
		os.Exit(1)
	}
}

func initSysAcc(svc *service.Service) error {
	sysAccName := constants.SystemAccountOpeningBalance

	_, err := svc.Account.GetAccountByName(sysAccName)
	if err == nil {
		return nil
	}

	currency := viper.GetString("defaults.currency")

	if currency == "" {
		currency, err = initWizard()
		if err != nil {
			return err
		}
		cfg.Defaults.Currency = currency
	}

	_, err = svc.Account.CreateAccount(
		sysAccName,
		constants.TypeEquity,
		currency,
		"Opening Balances (System Account)",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed create system account: %w", err)
	}

	return nil
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		appDir, err := getAppDataDir()
		if err != nil {
			return fmt.Errorf("error getting app dir: %w", err)
		}

		viper.AddConfigPath(appDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := createDefaultConfig(); err != nil {
		return fmt.Errorf("failed to ensure config file: %w", err)
	}

	viper.SetEnvPrefix("KEA")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // allow using environment variables to override

	if err := viper.ReadInConfig(); err != nil {

		if cfgFile != "" {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		if !errors.As(err, &viper.ConfigFileNotFoundError{}) {
			return fmt.Errorf("config file error: %w", err)
		}
	}

	cfg = config.NewDefault()
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("unable to decode into struct, %v", err)
	}

	cfg.ConfigPath = viper.ConfigFileUsed()

	return nil
}

func initWizard() (string, error) {
	currentDefault := viper.GetString("defaults.currency")
	if currentDefault == "" {
		currentDefault = "USD"
	}

	currency, err := prompts.PromptInitCurrency(currentDefault)
	if err != nil {
		return "", err
	}

	viper.Set("defaults.currency", currency)

	if err := viper.WriteConfig(); err != nil {
		return "", fmt.Errorf("failed to save config to file: %w", err)
	}

	pterm.Success.Printf("Configuration saved. Default currency set to: %s\n", currency)

	return currency, nil
}

func getAppDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to determine user home directory: %w", err)
		}
		return filepath.Join(home, ".kea"), nil
	}

	return filepath.Join(configDir, "kea"), nil
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
	}
	return path, nil
}

func createDefaultConfig() error {
	appDir, err := getAppDataDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(appDir, "config.yaml")

	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	if err := viper.WriteConfigAs(configPath); err != nil {
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
