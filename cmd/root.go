// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hance08/kea/cmd/account"
	ledgercmd "github.com/hance08/kea/cmd/ledger"
	"github.com/hance08/kea/cmd/transaction"
	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
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

	rootCmd := &cobra.Command{
		Use:           "kea",
		Short:         "kea is a CLI/TUI based personal accounting tool",
		Long:          `kea is a CLI/TUI based personal accounting tool`,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "set the config file path")
	rootCmd.PersistentFlags().BoolVar(new(bool), "no-color", false, "disable colored output (machine-friendly)")

	_ = rootCmd.ParseFlags(os.Args[1:])

	noColor, _ := rootCmd.PersistentFlags().GetBool("no-color")
	configureOutput(noColor)

	cfg, err := initConfig(cfgFile)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	appDir, err := app.GetAppDataDir()
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	registry, err := ledger.Load(appDir)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	if registry.MigratedLegacy {
		pterm.Info.Println(`Migrated existing database as ledger "default".`)
	}

	exitCode := func() int {
		// Ledger management commands are always available.
		rootCmd.AddCommand(ledgercmd.NewLedgerCmd(registry, migrations, appDir))

		if isLedgerCommand(os.Args[1:]) {
			if err := rootCmd.ExecuteContext(context.Background()); err != nil {
				pterm.Error.Println(capitalize(err.Error()))
				return 1
			}
			return 0
		}

		activePath, err := registry.Active()
		if err != nil {
			// No active ledger — only ledger commands are useful.
			pterm.Warning.Println("No ledger configured. Run: kea ledger add <name>")
			if err := rootCmd.ExecuteContext(context.Background()); err != nil {
				pterm.Error.Println(capitalize(err.Error()))
				return 1
			}
			return 0
		}

		// Inject the resolved DB path so app.NewApp and kea info both see it.
		cfg.Database.Path = activePath
		cfg.ActiveLedger = registry.ActiveName()

		application, cleanup, err := app.NewApp(cfg, registry, migrations)
		if err != nil {
			pterm.Error.Println(err)
			return 1
		}
		defer cleanup()

		if err := ensureCurrency(cfg); err != nil {
			pterm.Error.Println(err)
			return 1
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		if err := initSysAcc(ctx, application.Service, cfg); err != nil {
			pterm.Error.Println(err)
			return 1
		}

		rootCmd.AddCommand(account.NewAccountCmd(application.Service))
		rootCmd.AddCommand(transaction.NewTransactionCmd(application.Service))
		rootCmd.AddCommand(NewAddCmd(application.Service))
		rootCmd.AddCommand(NewInfoCmd(application.Service))
		rootCmd.AddCommand(NewReportCmd(application.Service))
		rootCmd.AddCommand(NewReconcileCmd(application.Service))

		if err := rootCmd.ExecuteContext(ctx); err != nil {
			pterm.Error.Println(capitalize(err.Error()))
			return 1
		}
		return 0
	}()
	os.Exit(exitCode)
}

func initSysAcc(ctx context.Context, svc *service.Service, cfg *config.Config) error {
	if err := migrateLegacySysAcc(ctx, svc, cfg); err != nil {
		return err
	}

	targetName := model.OpeningBalancesAccountName(cfg.Defaults.Currency)
	_, err := svc.Account().GetAccountByName(ctx, targetName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, service.ErrNotFound) {
		return fmt.Errorf("failed to check system account: %w", err)
	}

	_, err = svc.Account().CreateAccount(
		ctx,
		model.CreateAccountInput{
			Name:        targetName,
			Type:        model.AccountTypeEquity,
			Currency:    cfg.Defaults.Currency,
			Description: "Opening Balances (System Account)",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create system account: %w", err)
	}

	return nil
}

type accountMigrator interface {
	GetAccountByName(ctx context.Context, name string) (*model.Account, error)
	RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error)
	DeleteAccountByName(ctx context.Context, name string) error
}

// migrateLegacySysAcc renames the legacy "Equity:OpeningBalances" account to
// "Equity:OpeningBalances_<currency>" the first time a user upgrades.
// It is a no-op when the legacy account does not exist.
func migrateLegacySysAcc(ctx context.Context, svc *service.Service, cfg *config.Config) error {
	return migrateLegacySysAccWith(ctx, svc.Account(), cfg)
}

func migrateLegacySysAccWith(ctx context.Context, acc accountMigrator, cfg *config.Config) error {
	fullTargetName := model.OpeningBalancesAccountName(cfg.Defaults.Currency)

	// Nothing to migrate if legacy account is already gone.
	_, err := acc.GetAccountByName(ctx, model.LegacyOpeningBalancesName)
	if errors.Is(err, service.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check legacy system account: %w", err)
	}

	// Both accounts exist: the target was created independently. Remove the
	// empty legacy account, or error if it still holds transactions.
	_, err = acc.GetAccountByName(ctx, fullTargetName)
	if err == nil {
		if err := acc.DeleteAccountByName(ctx, model.LegacyOpeningBalancesName); err != nil {
			return fmt.Errorf(
				"legacy system account %q and target %q both exist; "+
					"remove or reconcile the legacy account manually: %w",
				model.LegacyOpeningBalancesName, fullTargetName, err,
			)
		}
		return nil
	}
	if !errors.Is(err, service.ErrNotFound) {
		return fmt.Errorf("failed to check target system account: %w", err)
	}

	if _, err := acc.RenameAccount(ctx, model.LegacyOpeningBalancesName, fullTargetName); err != nil {
		return fmt.Errorf("failed to migrate legacy system account: %w", err)
	}

	return nil
}

func ensureCurrency(cfg *config.Config) error {
	return ensureCurrencyWith(cfg, isInteractive())
}

func ensureCurrencyWith(cfg *config.Config, interactive bool) error {
	if cfg.Defaults.Currency != "" {
		return nil
	}

	if interactive {
		return initWizard(cfg)
	}

	if err := setCurrency(cfg, "USD"); err != nil {
		return err
	}

	pterm.Warning.Println("No default currency configured; defaulting to USD.")

	return nil
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func setCurrency(cfg *config.Config, currency string) error {
	cfg.Defaults.Currency = currency
	viper.Set("defaults.currency", currency)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config to file: %w", err)
	}

	return nil
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

	setServerDefaults(viper.GetViper())

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

	if err := setCurrency(cfg, currency); err != nil {
		return err
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

func setServerDefaults(v *viper.Viper) {
	def := config.NewDefault()
	v.SetDefault("server.host", def.Server.Host)
	v.SetDefault("server.port", def.Server.Port)
	v.SetDefault("server.cors_origins", def.Server.CORSOrigins)
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

// isLedgerCommand reports whether args represent a ledger management command.
// Only -c/--config consumes the next token as its value.
// If new value-consuming flags are added to rootCmd, update this list.
func isLedgerCommand(args []string) bool {
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "-c" || a == "--config" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "ledger"
	}
	return false
}

func configureOutput(noColorFlag bool) {
	if noColorFlag || os.Getenv("NO_COLOR") != "" {
		pterm.DisableStyling()
		return
	}

	pterm.EnableStyling()
}
