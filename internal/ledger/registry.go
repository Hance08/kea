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

// Add registers a new ledger. Returns ErrLedgerExists if the name is already taken.
func (r *Registry) Add(name, dbPath string) error {
	if _, exists := r.Ledgers[name]; exists {
		return fmt.Errorf("%w: %q", ErrLedgerExists, name)
	}
	r.Ledgers[name] = Entry{Path: dbPath}
	return r.Save()
}

// Switch sets the active ledger by name. Returns ErrLedgerNotFound if unknown.
func (r *Registry) Switch(name string) error {
	if _, exists := r.Ledgers[name]; !exists {
		return fmt.Errorf("%w: %q — run: kea ledger list", ErrLedgerNotFound, name)
	}
	r.ActiveLedger = name
	return r.Save()
}

// Active returns the resolved DB path for the active ledger.
// KEA_LEDGER env var overrides the registry's active field.
func (r *Registry) Active() (string, error) {
	name := r.ActiveName()
	if name == "" {
		return "", ErrNoActiveLedger
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return "", fmt.Errorf("active ledger %q is not registered — run: kea ledger list", name)
	}
	return entry.Path, nil
}

// ActiveName returns the name of the active ledger.
// KEA_LEDGER env var takes precedence over the registry's active field.
func (r *Registry) ActiveName() string {
	if env := os.Getenv("KEA_LEDGER"); env != "" {
		return env
	}
	return r.ActiveLedger
}

// Remove unregisters a ledger. If deleteFile is true and the ledger's DB file
// exists, it is deleted after unregistering. Returns ErrRemoveActive if the
// target is currently the active ledger.
func (r *Registry) Remove(name string, deleteFile bool) error {
	if name == r.ActiveName() {
		return ErrRemoveActive
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return fmt.Errorf("%w: %q", ErrLedgerNotFound, name)
	}
	delete(r.Ledgers, name)
	if err := r.Save(); err != nil {
		return err
	}
	if deleteFile {
		if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete database file: %w", err)
		}
	}
	return nil
}
