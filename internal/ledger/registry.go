package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

var (
	ErrNoActiveLedger = errors.New("no ledger configured — run: kea ledger add <name>")
	ErrLedgerExists   = errors.New("ledger already exists")
	ErrLedgerNotFound = errors.New("ledger not found")
	ErrRemoveActive   = errors.New("cannot remove active ledger — switch to another ledger first")
)

// Entry holds metadata for a single registered ledger.
type Entry struct {
	Path string `yaml:"path"`
}

// Registry manages the set of known ledgers and the active ledger name.
type Registry struct {
	ActiveLedger   string           `yaml:"active"`
	Ledgers        map[string]Entry `yaml:"ledgers"`
	MigratedLegacy bool             `yaml:"-"`
	filePath       string
}

// Load reads or initialises the ledger registry from appDir/ledgers.yaml.
// If ledgers.yaml is absent and appDir/kea.db exists, it auto-migrates
// by registering kea.db as the "default" ledger.
func Load(appDir string) (*Registry, error) {
	registryPath := filepath.Join(appDir, "ledgers.yaml")

	data, err := os.ReadFile(registryPath)
	if err == nil {
		r := &Registry{filePath: registryPath}
		if err := yaml.Unmarshal(data, r); err != nil {
			return nil, fmt.Errorf("parse ledgers.yaml: %w", err)
		}
		if r.Ledgers == nil {
			r.Ledgers = make(map[string]Entry)
		}
		return r, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read ledgers.yaml: %w", err)
	}

	r := &Registry{
		Ledgers:  make(map[string]Entry),
		filePath: registryPath,
	}

	legacyDB := filepath.Join(appDir, "kea.db")
	if _, err := os.Stat(legacyDB); err == nil {
		r.Ledgers["default"] = Entry{Path: legacyDB}
		r.ActiveLedger = "default"
		r.MigratedLegacy = true
		if err := r.Save(); err != nil {
			return nil, fmt.Errorf("auto-migrate: %w", err)
		}
		return r, nil
	}

	if err := r.Save(); err != nil {
		return nil, fmt.Errorf("init ledgers.yaml: %w", err)
	}
	return r, nil
}

// Save writes the current registry state to ledgers.yaml.
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(r.filePath, data, 0644)
}

// Names returns all registered ledger names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Ledgers))
	for name := range r.Ledgers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EntryFor returns the Entry for a given name and whether it was found.
func (r *Registry) EntryFor(name string) (Entry, bool) {
	e, ok := r.Ledgers[name]
	return e, ok
}
