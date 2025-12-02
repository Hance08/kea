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
	"github.com/hance08/kea/internal/logic/accounting"
	"github.com/hance08/kea/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile         string
	dbStore         *store.Store
	logic           *accounting.AccountingLogic
	defaultCurrency string
	migrationsFS    fs.FS
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kea",
	Short: "kea is a CLI/TUI based personal accounting tool",
	Long:  `kea is a CLI/TUI based personal accounting tool`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 1. read the config file
		if err := initConfig(); err != nil {
			return err
		}

		// 2. get config value
		dbPath := viper.GetString("database.path")
		if dbPath == "" {
			return fmt.Errorf("error : can't find 'database.path' setting")
		}

		// expand the ~ symbol (if there has)
		if strings.HasPrefix(dbPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("can't get the home directory : %w", err)
			}
			dbPath = filepath.Join(home, dbPath[2:])
		}

		// fmt.Printf("Database path : %s\n", dbPath)

		defaultCurrency = viper.GetString("defaults.currency")
		if defaultCurrency == "" {
			return fmt.Errorf("error : can't find 'defaults.currency' setting")
		}

		// 3. initialize Store
		var err error
		dbStore, err = store.NewStore(dbPath, migrationsFS)
		if err != nil {
			return fmt.Errorf("failed to initialize database : %w", err)
		}

		// 4. initialize logic and input Store
		logic = accounting.NewLogic(dbStore)

		// 5. inject dependencies into subcommands
		transaction.SetDependencies(logic)
		account.SetDependencies(logic)

		if err := ensureSystemAccounts(); err != nil {
			return fmt.Errorf("failed to initialize system account : %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbStore != nil {
			return dbStore.Close()
		}
		return nil
	},
}

func Execute(migrations fs.FS) {
	migrationsFS = migrations
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "set the config file path")
	home, _ := os.UserHomeDir()
	viper.SetDefault("database.path", filepath.Join(home, ".kea", "kea.db"))

	viper.SetDefault("defaults.currency", "TWD")

	// Register subcommands
	rootCmd.AddCommand(transaction.TransactionCmd)
	rootCmd.AddCommand(account.AccountCmd)
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(filepath.Join(home, ".config", "kea")) // Linux or Mac: ~/.config/kea
		viper.AddConfigPath(filepath.Join(home, ".kea"))           // Fallback: ~/.kea
		viper.SetConfigName("config")                              // config.yaml
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("KEA")
	viper.AutomaticEnv() // allow using environment varibles to override

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// use default settings
		} else {
			// config file has been found, but failed to interpret
			return err
		}
	}
	return nil
}

func ensureSystemAccounts() error {
	// ensure Equity:OpeningBalance existed
	_, err := logic.GetAccountByName("Equity:OpeningBalances")
	if err != nil {
		// account doesn't exist, create it
		_, err = logic.CreateAccount(
			"Equity:OpeningBalances",
			"C",
			defaultCurrency,
			"Opening Balances (System Account)",
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed create 'Equity:OpeningBalances' account : %w", err)
		}
		// fmt.Printf("automatic create system account -> Equity:OpeningBalances")
	}

	return nil
}
