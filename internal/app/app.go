package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
)
type App struct {
	Service *service.Service
}

// NewApp initialize config, database and core logic, then return App entity
func NewApp(cfg *config.Config, migrationFS fs.FS) (*App, func(), error) {
	dbPathRaw := cfg.Database.Path

	if dbPathRaw == "" {
		appDir, err := GetAppDataDir()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to determine data directory: %w", err)
		}
		dbPathRaw = filepath.Join(appDir, "kea.db")
	}

	dbStore, err := store.NewStore(dbPathRaw, migrationFS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	svc := service.NewService(dbStore, dbStore, dbStore, cfg)

	cleanup := func() {
		if err := dbStore.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing DB: %v\n", err)
		}
	}

	return &App{
		Service: svc,
	}, cleanup, nil
}

func GetAppDataDir() (string, error) {
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
