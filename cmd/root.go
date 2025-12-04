/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hance08/kea/cmd/account"
	"github.com/hance08/kea/cmd/transaction"
	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/errhandler"
	"github.com/hance08/kea/internal/logic/accounting"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	dbStore      *store.Store
	logic        *accounting.AccountingLogic
	migrationsFS fs.FS
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:                "kea",
	Short:              "kea is a CLI/TUI based personal accounting tool",
	Long:               `kea is a CLI/TUI based personal accounting tool`,
	PersistentPreRunE:  setupApplication,
	PersistentPostRunE: cleanupApplication,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "set the config file path")

	appDir, err := getAppDataDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	viper.SetDefault("database.path", filepath.Join(appDir, "kea.db"))
	viper.SetDefault("defaults.currency", "USD")

	rootCmd.AddCommand(transaction.TransactionCmd)
	rootCmd.AddCommand(account.AccountCmd)
}

func setupApplication(cmd *cobra.Command, args []string) error {
	if isHelpCommand(cmd) {
		return nil
	}

	err := initApp()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	return nil
}

func cleanupApplication(cmd *cobra.Command, args []string) error {
	if dbStore != nil {
		return dbStore.Close()
	}
	return nil
}

func isHelpCommand(cmd *cobra.Command) bool {
	return cmd.Name() == "help" || cmd.Name() == "version" || cmd.Name() == "completion"
}

func initApp() error {
	if err := initConfig(); err != nil {
		return err
	}

	if err := initDB(); err != nil {
		return err
	}

	initLogicAndDependencies()

	if err := initSysAcc(); err != nil {
		return err
	}

	return nil
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		appDir, err := getAppDataDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting app dir:", err)
		}

		viper.AddConfigPath(appDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := createDefaultConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to ensure config file: %v\n", err)
	}

	viper.SetEnvPrefix("KEA")
	viper.AutomaticEnv() // allow using environment varibles to override

	if err := viper.ReadInConfig(); err != nil {

		if cfgFile != "" {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("config file error: %w", err)
		}
	}

	return nil
}

func initDB() error {
	dbPathRaw := viper.GetString("database.path")
	if dbPathRaw == "" {
		return fmt.Errorf("config 'database.path' is missing")
	}

	dbPath, err := expandPath(dbPathRaw)
	if err != nil {
		return fmt.Errorf("invalid database path: %w", err)
	}

	dbStore, err = store.NewStore(dbPath, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	return nil
}

func initLogicAndDependencies() {
	logic = accounting.NewLogic(dbStore)

	transaction.SetDependencies(logic)
	account.SetDependencies(logic)
}

func initSysAcc() error {
	sysAccName := constants.SystemAccountOpeningBalance

	_, err := logic.GetAccountByName(sysAccName)

	if err == nil {
		return nil
	}
	//TODO: Should be returned "Record Not Found" to avoid DB connection error

	currency, err := initWizard()
	if err != nil {
		return err
	}

	_, err = logic.CreateAccount(
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
		fmt.Fprintf(os.Stderr, "Warning: Failed to save config to file: %v\n", err)
	} else {
		fmt.Printf("✔ Configuration saved. Default currency set to: %s\n", currency)
	}

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

func Execute(migrations fs.FS) {
	migrationsFS = migrations
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err != nil {
		errhandler.HandleError(err)
		os.Exit(1)
	}
}
