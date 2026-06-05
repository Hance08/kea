// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package ledger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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
	mu             sync.Mutex
	callbacks      []func(name string, path string)
	watcher        *fsnotify.Watcher
}

// EmptyRegistry returns a Registry with no ledgers and no active ledger.
// Intended for use in tests that need to exercise the "no ledgers" code path.
func EmptyRegistry() *Registry {
	return &Registry{Ledgers: make(map[string]Entry)}
}

// Load reads or initialises the ledger registry from appDir/ledgers.yaml.
// If ledgers.yaml is absent and appDir/kea.db exists, it auto-migrates
// by registering kea.db as the "default" ledger.
func Load(appDir string) (*Registry, error) {
	registryPath := filepath.Join(appDir, "ledgers.yaml")

	r := &Registry{
		Ledgers:  make(map[string]Entry),
		filePath: registryPath,
	}

	data, err := os.ReadFile(registryPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read ledgers.yaml: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, r); err != nil {
			return nil, fmt.Errorf("parse ledgers.yaml: %w", err)
		}
		if r.Ledgers == nil {
			r.Ledgers = make(map[string]Entry)
		}
		// If the file exists but has ledgers, return as-is.
		if len(r.Ledgers) > 0 {
			return r, nil
		}
	}

	// No ledgers registered yet (fresh install or empty file from old version).
	// Auto-migrate a pre-existing kea.db if present; otherwise bootstrap a default.
	legacyDB := filepath.Join(appDir, "kea.db")
	if _, err := os.Stat(legacyDB); err == nil {
		r.Ledgers["default"] = Entry{Path: legacyDB}
		r.ActiveLedger = "default"
		r.MigratedLegacy = true
	} else {
		r.Ledgers["default"] = Entry{Path: legacyDB}
		r.ActiveLedger = "default"
	}

	// Load runs during single-threaded init before any other goroutine
	// touches r; calling saveLocked() directly is safe and avoids the
	// uncontended lock cycle.
	if err := r.saveLocked(); err != nil {
		return nil, fmt.Errorf("init default ledger: %w", err)
	}
	return r, nil
}

// Save writes the current registry state to ledgers.yaml. Safe to call from
// external goroutines; takes r.mu internally.
func (r *Registry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked()
}

// registrySnapshot is a plain struct used as the YAML serialisation target.
// Marshalling r directly would expose r.mu to reflect while other goroutines
// hold or contend the same word, which the race detector correctly flags.
type registrySnapshot struct {
	ActiveLedger string           `yaml:"active"`
	Ledgers      map[string]Entry `yaml:"ledgers"`
}

// saveLocked persists the registry to disk. The caller MUST hold r.mu so
// that the snapshot sees a consistent view of r.ActiveLedger and r.Ledgers.
func (r *Registry) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	snap := registrySnapshot{
		ActiveLedger: r.ActiveLedger,
		Ledgers:      r.Ledgers,
	}
	data, err := yaml.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(r.filePath, data, 0644)
}

// Names returns all registered ledger names in sorted order. Safe to call
// from external goroutines; takes r.mu internally.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.Ledgers))
	for name := range r.Ledgers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EntryFor returns the Entry for a given name and whether it was found.
// Safe to call from external goroutines; takes r.mu internally.
func (r *Registry) EntryFor(name string) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.Ledgers[name]
	return e, ok
}

// Add registers a new ledger. Returns ErrLedgerExists if the name is already taken.
func (r *Registry) Add(name, dbPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Ledgers[name]; exists {
		return fmt.Errorf("%w: %q", ErrLedgerExists, name)
	}
	r.Ledgers[name] = Entry{Path: dbPath}
	return r.saveLocked()
}

// Switch sets the active ledger by name. Returns ErrLedgerNotFound if unknown.
func (r *Registry) Switch(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Ledgers[name]; !exists {
		return fmt.Errorf("%w: %q — run: kea ledger list", ErrLedgerNotFound, name)
	}
	r.ActiveLedger = name
	return r.saveLocked()
}

// Active returns the resolved DB path for the active ledger.
// KEA_LEDGER env var overrides the registry's active field. Safe to call
// from external goroutines; takes r.mu internally.
func (r *Registry) Active() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := r.activeNameLocked()
	if name == "" {
		return "", ErrNoActiveLedger
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return "", fmt.Errorf("active ledger %q is not registered — run: kea ledger list", name)
	}
	return entry.Path, nil
}

// ActiveName returns the name of the active ledger. KEA_LEDGER env var
// takes precedence over the registry's active field. Safe to call from
// external goroutines; takes r.mu internally.
func (r *Registry) ActiveName() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeNameLocked()
}

// activeNameLocked returns the active ledger name. Caller MUST hold r.mu.
// The env-var lookup itself is thread-safe and independent of registry
// state but is held inside the lock to keep the function shape simple.
func (r *Registry) activeNameLocked() string {
	if env := os.Getenv("KEA_LEDGER"); env != "" {
		return env
	}
	return r.ActiveLedger
}

// Remove unregisters a ledger. If deleteFile is true and the ledger's DB file
// exists, it is deleted after unregistering. Returns ErrRemoveActive if the
// target is currently the active ledger. The file deletion happens outside
// r.mu — the captured entry path was resolved while holding the lock, and
// blocking all registry reads on a disk-remove call would be needlessly
// pessimistic.
func (r *Registry) Remove(name string, deleteFile bool) error {
	entry, err := r.removeAndSave(name)
	if err != nil {
		return err
	}
	if !deleteFile {
		return nil
	}
	if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete database file: %w", err)
	}
	return nil
}

// removeAndSave deletes name from r.Ledgers, persists the registry, and
// returns the removed entry so the caller can act on it outside the lock.
// Takes r.mu internally.
func (r *Registry) removeAndSave(name string) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == r.activeNameLocked() {
		return Entry{}, ErrRemoveActive
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return Entry{}, fmt.Errorf("%w: %q", ErrLedgerNotFound, name)
	}
	delete(r.Ledgers, name)
	if err := r.saveLocked(); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// OnSwitch registers a callback that is called when the active ledger changes.
// Multiple callbacks may be registered; all are called on each change.
func (r *Registry) OnSwitch(fn func(name, path string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, fn)
}

const debounceDuration = 100 * time.Millisecond

// ErrWatcherAlreadyRunning is returned by Watch when a watcher is already active.
var ErrWatcherAlreadyRunning = errors.New("watcher already running")

// Watch monitors ledgers.yaml for changes and fires registered OnSwitch
// callbacks when the active ledger changes. It blocks until ctx is cancelled
// or the watcher is stopped via StopWatch. Includes a debounce to coalesce
// rapid writes into a single callback. Returns ErrWatcherAlreadyRunning if
// Watch has already been called without a subsequent StopWatch.
func (r *Registry) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	r.mu.Lock()
	if r.watcher != nil {
		r.mu.Unlock()
		_ = w.Close()
		return ErrWatcherAlreadyRunning
	}
	r.watcher = w
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.watcher = nil
		r.mu.Unlock()
		_ = w.Close()
	}()

	if err := w.Add(r.filePath); err != nil {
		return fmt.Errorf("watch %s: %w", r.filePath, err)
	}

	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			if debounce != nil {
				debounce.Stop()
			}
			return ctx.Err()
		case event, ok := <-w.Events:
			if !ok {
				if debounce != nil {
					debounce.Stop()
				}
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceDuration, func() {
				r.reload()
			})
		case err, ok := <-w.Errors:
			if !ok {
				if debounce != nil {
					debounce.Stop()
				}
				return nil
			}
			fmt.Fprintf(os.Stderr, "ledger watcher error: %v\n", err)
		}
	}
}

// StopWatch closes the underlying fsnotify watcher, causing Watch to return.
func (r *Registry) StopWatch() {
	r.mu.Lock()
	w := r.watcher
	r.watcher = nil
	r.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
}

// reload reads ledgers.yaml from disk and fires OnSwitch callbacks if the
// active ledger has changed since the last read.
func (r *Registry) reload() {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reload ledgers.yaml: %v\n", err)
		return
	}
	var fresh Registry
	if err := yaml.Unmarshal(data, &fresh); err != nil {
		fmt.Fprintf(os.Stderr, "parse ledgers.yaml: %v\n", err)
		return
	}
	if fresh.Ledgers == nil {
		fresh.Ledgers = make(map[string]Entry)
	}

	r.mu.Lock()
	prev := r.ActiveLedger
	if fresh.ActiveLedger == prev {
		r.mu.Unlock()
		return
	}
	r.ActiveLedger = fresh.ActiveLedger
	r.Ledgers = fresh.Ledgers
	entry, exists := r.Ledgers[r.ActiveLedger]
	cbs := make([]func(string, string), len(r.callbacks))
	copy(cbs, r.callbacks)
	r.mu.Unlock()

	if !exists {
		fmt.Fprintf(os.Stderr, "reloaded active ledger %q not found in registry\n", fresh.ActiveLedger)
		return
	}

	for _, fn := range cbs {
		fn(fresh.ActiveLedger, entry.Path)
	}
}
