package ledger

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/store"
	"github.com/spf13/cobra"
)

type addFlags struct {
	Path string
}

type addRunner struct {
	registry   *internalled.Registry
	migrations fs.FS
	appDir     string
	name       string
	customPath string
	dbInitFn   func(path string, migrations fs.FS) error
}

func NewAddCmd(registry *internalled.Registry, migrations fs.FS, appDir string) *cobra.Command {
	flags := &addFlags{}
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register and initialise a new ledger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &addRunner{
				registry:   registry,
				migrations: migrations,
				appDir:     appDir,
				name:       args[0],
				customPath: flags.Path,
				dbInitFn:   defaultDBInit,
			}
			return runner.Run()
		},
	}
	cmd.Flags().StringVarP(&flags.Path, "path", "p", "", "custom path for the database file")
	return cmd
}

func defaultDBInit(path string, migrations fs.FS) error {
	s, err := store.NewStore(path, migrations)
	if err != nil {
		return err
	}
	return s.Close()
}

func (r *addRunner) Run() error {
	// Fail early on duplicate name before creating any files.
	if _, exists := r.registry.Ledgers[r.name]; exists {
		return fmt.Errorf("%w: %q", internalled.ErrLedgerExists, r.name)
	}

	dbPath := r.customPath
	if dbPath == "" {
		dbPath = filepath.Join(r.appDir, "ledgers", r.name+".db")
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return fmt.Errorf("create ledger directory: %w", err)
		}
	} else {
		dir := filepath.Dir(dbPath)
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
	}

	if err := r.dbInitFn(dbPath, r.migrations); err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}

	if err := r.registry.Add(r.name, dbPath); err != nil {
		return err
	}

	fmt.Printf("Ledger %q created at %s\n", r.name, dbPath)
	fmt.Printf("To switch to it: kea ledger switch %s\n", r.name)
	return nil
}
