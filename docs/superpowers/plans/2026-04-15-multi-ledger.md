# Multi-Ledger Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persistent named-ledger system so users can manage multiple independent SQLite databases from a single KEA installation.

**Architecture:** A new `internal/ledger` package owns the `Registry` type and its `ledgers.yaml` file. `cmd/root.go` loads the registry on every startup, resolves the active DB path, and injects it into `app.NewApp`. A new `cmd/ledger` command group provides `add`, `list`, `switch`, and `remove` subcommands. Downstream layers (`service`, `store`) are unchanged — they receive a plain DB path string as before.

**Tech Stack:** Go, `gopkg.in/yaml.v3` (already in `go.sum` as indirect dep), `github.com/stretchr/testify`, `github.com/pterm/pterm`, `github.com/spf13/cobra`

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Create | `internal/ledger/registry.go` | Registry type, Load, Save, Add, Switch, Active, Remove, helpers |
| Create | `internal/ledger/registry_test.go` | Unit tests for all registry methods |
| Create | `cmd/ledger/ledger.go` | `kea ledger` group command + wires subcommands |
| Create | `cmd/ledger/list.go` | `kea ledger list` runner |
| Create | `cmd/ledger/list_test.go` | Tests for list runner |
| Create | `cmd/ledger/add.go` | `kea ledger add` runner |
| Create | `cmd/ledger/add_test.go` | Tests for add runner |
| Create | `cmd/ledger/switch.go` | `kea ledger switch` runner |
| Create | `cmd/ledger/switch_test.go` | Tests for switch runner |
| Create | `cmd/ledger/remove.go` | `kea ledger remove` runner |
| Create | `cmd/ledger/remove_test.go` | Tests for remove runner |
| Modify | `internal/config/config.go` | Add `ActiveLedger string` computed field |
| Modify | `cmd/root.go` | Load registry, resolve active path, wire `kea ledger` command |
| Modify | `ui/views/system_info.go` | Add `ActiveLedger` field to `SystemInfo` and `Render` |
| Modify | `ui/views/json_types.go` | Add `active_ledger` to `JSONSystemInfo` |
| Modify | `cmd/info.go` | Pass `cfg.ActiveLedger` into `SystemInfo` |

---

## Task 1: `internal/ledger` — Registry struct + Load + Save

**Files:**
- Create: `internal/ledger/registry.go`
- Create: `internal/ledger/registry_test.go`

- [ ] **Step 1: Add `gopkg.in/yaml.v3` as a direct dependency**

```bash
cd /path/to/kea && go get gopkg.in/yaml.v3
```
Expected: `go.mod` now lists `gopkg.in/yaml.v3` as a direct `require`.

- [ ] **Step 2: Write failing tests for Load**

Create `internal/ledger/registry_test.go`:

```go
package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FreshInstall(t *testing.T) {
	dir := t.TempDir()

	r, err := Load(dir)

	require.NoError(t, err)
	assert.Empty(t, r.Ledgers)
	assert.Empty(t, r.ActiveLedger)
	_, statErr := os.Stat(filepath.Join(dir, "ledgers.yaml"))
	assert.NoError(t, statErr, "ledgers.yaml should be created on fresh install")
}

func TestLoad_AutoMigratesLegacyDB(t *testing.T) {
	dir := t.TempDir()
	legacyDB := filepath.Join(dir, "kea.db")
	require.NoError(t, os.WriteFile(legacyDB, []byte(""), 0644))

	r, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r.Ledgers, "default")
	assert.Equal(t, legacyDB, r.Ledgers["default"].Path)
	assert.Equal(t, "default", r.ActiveLedger)
}

func TestLoad_NormalLoad(t *testing.T) {
	dir := t.TempDir()
	r1, err := Load(dir)
	require.NoError(t, err)
	r1.Ledgers["work"] = Entry{Path: "/tmp/work.db"}
	r1.ActiveLedger = "work"
	require.NoError(t, r1.Save())

	r2, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r2.Ledgers, "work")
	assert.Equal(t, "/tmp/work.db", r2.Ledgers["work"].Path)
	assert.Equal(t, "work", r2.ActiveLedger)
}
```

- [ ] **Step 3: Run to verify tests fail**

```bash
go test ./internal/ledger/... -v -run "TestLoad"
```
Expected: FAIL with `cannot find package` or `undefined: Load`.

- [ ] **Step 4: Implement Registry struct + Load + Save**

Create `internal/ledger/registry.go`:

```go
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
	ActiveLedger string           `yaml:"active"`
	Ledgers      map[string]Entry `yaml:"ledgers"`
	filePath     string
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
		if err := r.Save(); err != nil {
			return nil, fmt.Errorf("auto-migrate: %w", err)
		}
		fmt.Println(`Migrated existing database as ledger "default".`)
		return r, nil
	}

	if err := r.Save(); err != nil {
		return nil, fmt.Errorf("init ledgers.yaml: %w", err)
	}
	return r, nil
}

// Save writes the current registry state to ledgers.yaml.
func (r *Registry) Save() error {
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
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/ledger/... -v -run "TestLoad"
```
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/ledger/registry.go internal/ledger/registry_test.go go.mod go.sum
git commit -m "feat(ledger): add Registry type with Load/Save"
```

---

## Task 2: Registry — Add + Switch

**Files:**
- Modify: `internal/ledger/registry.go`
- Modify: `internal/ledger/registry_test.go`

- [ ] **Step 1: Write failing tests for Add and Switch**

Append to `internal/ledger/registry_test.go`:

```go
func TestAdd_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Add("personal", "/tmp/personal.db")

	require.NoError(t, err)
	assert.Equal(t, "/tmp/personal.db", r.Ledgers["personal"].Path)
}

func TestAdd_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	err = r.Add("personal", "/tmp/other.db")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLedgerExists)
}

func TestAdd_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.Contains(t, r2.Ledgers, "work")
}

func TestSwitch_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	err = r.Switch("work")

	require.NoError(t, err)
	assert.Equal(t, "work", r.ActiveLedger)
}

func TestSwitch_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Switch("nonexistent")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLedgerNotFound)
}

func TestSwitch_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "work", r2.ActiveLedger)
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./internal/ledger/... -v -run "TestAdd|TestSwitch"
```
Expected: FAIL with `undefined: Add` and `undefined: Switch`.

- [ ] **Step 3: Implement Add and Switch**

Append to `internal/ledger/registry.go` (before the closing brace of the file, after `EntryFor`):

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/ledger/... -v -run "TestAdd|TestSwitch"
```
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ledger/registry.go internal/ledger/registry_test.go
git commit -m "feat(ledger): add Registry.Add and Registry.Switch"
```

---

## Task 3: Registry — Active, ActiveName, Remove

**Files:**
- Modify: `internal/ledger/registry.go`
- Modify: `internal/ledger/registry_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/ledger/registry_test.go`:

```go
func TestActive_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	path, err := r.Active()

	require.NoError(t, err)
	assert.Equal(t, "/tmp/work.db", path)
}

func TestActive_NoActiveLedger(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	_, err = r.Active()

	assert.ErrorIs(t, err, ErrNoActiveLedger)
}

func TestActive_EnvVarOverride(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))
	require.NoError(t, r.Switch("work"))
	t.Setenv("KEA_LEDGER", "personal")

	path, err := r.Active()

	require.NoError(t, err)
	assert.Equal(t, "/tmp/personal.db", path)
}

func TestActive_UnregisteredActiveName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	r.ActiveLedger = "ghost"

	_, err = r.Active()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestRemove_UnregisterOnly(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))
	require.NoError(t, r.Switch("work"))

	err = r.Remove("personal", false)

	require.NoError(t, err)
	assert.NotContains(t, r.Ledgers, "personal")
	_, statErr := os.Stat("/tmp/personal.db")
	assert.NoError(t, statErr, "file should NOT be deleted without --delete-file")
}

func TestRemove_RefusesActiveLedger(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	err = r.Remove("work", false)

	assert.ErrorIs(t, err, ErrRemoveActive)
}

func TestRemove_DeleteFile(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	dbFile := filepath.Join(dir, "personal.db")
	require.NoError(t, os.WriteFile(dbFile, []byte(""), 0644))
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", dbFile))
	require.NoError(t, r.Switch("work"))

	err = r.Remove("personal", true)

	require.NoError(t, err)
	_, statErr := os.Stat(dbFile)
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "database file should be deleted")
}

func TestRemove_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Remove("nonexistent", false)

	assert.ErrorIs(t, err, ErrLedgerNotFound)
}

func TestRemove_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))
	require.NoError(t, r.Switch("work"))
	require.NoError(t, r.Remove("personal", false))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.NotContains(t, r2.Ledgers, "personal")
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./internal/ledger/... -v -run "TestActive|TestRemove"
```
Expected: FAIL with `undefined: Active`, `undefined: Remove`.

- [ ] **Step 3: Implement Active, ActiveName, Remove**

Append to `internal/ledger/registry.go`:

```go
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
```

- [ ] **Step 4: Run all ledger tests**

```bash
go test ./internal/ledger/... -v
```
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ledger/registry.go internal/ledger/registry_test.go
git commit -m "feat(ledger): add Registry.Active, ActiveName, Remove"
```

---

## Task 4: `cmd/ledger` — Group command + list

**Files:**
- Create: `cmd/ledger/ledger.go`
- Create: `cmd/ledger/list.go`
- Create: `cmd/ledger/list_test.go`

- [ ] **Step 1: Write failing test for list runner**

Create `cmd/ledger/list_test.go`:

```go
package ledger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRunner_NoLedgers(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	var buf bytes.Buffer
	runner := &listRunner{registry: r, out: &buf}
	err = runner.Run()

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No ledgers registered")
}

func TestListRunner_ShowsLedgersWithActiveMarker(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", filepath.Join(dir, "personal.db")))
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Switch("work"))

	var buf bytes.Buffer
	runner := &listRunner{registry: r, out: &buf}
	err = runner.Run()

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "* work")
	assert.Contains(t, output, "  personal")
}

func TestListRunner_EnvVarActiveMarker(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", filepath.Join(dir, "personal.db")))
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Switch("personal"))
	t.Setenv("KEA_LEDGER", "work")

	var buf bytes.Buffer
	runner := &listRunner{registry: r, out: &buf}
	err = runner.Run()

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "* work")
	assert.Contains(t, output, "  personal")

	_ = os.Unsetenv("KEA_LEDGER")
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./cmd/ledger/... -v -run "TestListRunner"
```
Expected: FAIL with `cannot find package`.

- [ ] **Step 3: Implement group command and list runner**

Create `cmd/ledger/ledger.go`:

```go
package ledger

import (
	"io/fs"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)

func NewLedgerCmd(registry *internalled.Registry, migrations fs.FS, appDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Manage ledgers (independent databases)",
		Long:  `Create, list, switch between, and remove named ledgers.`,
	}
	cmd.AddCommand(NewListCmd(registry))
	cmd.AddCommand(NewAddCmd(registry, migrations, appDir))
	cmd.AddCommand(NewSwitchCmd(registry))
	cmd.AddCommand(NewRemoveCmd(registry))
	return cmd
}
```

Create `cmd/ledger/list.go`:

```go
package ledger

import (
	"fmt"
	"io"
	"os"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)

type listRunner struct {
	registry *internalled.Registry
	out      io.Writer
}

func NewListCmd(registry *internalled.Registry) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List all registered ledgers",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &listRunner{registry: registry, out: os.Stdout}
			return runner.Run()
		},
	}
}

func (r *listRunner) Run() error {
	names := r.registry.Names()
	if len(names) == 0 {
		fmt.Fprintln(r.out, "No ledgers registered. Run: kea ledger add <name>")
		return nil
	}
	activeName := r.registry.ActiveName()
	fmt.Fprintf(r.out, "  %-20s  %s\n", "NAME", "PATH")
	for _, name := range names {
		entry, _ := r.registry.EntryFor(name)
		marker := "  "
		if name == activeName {
			marker = "* "
		}
		fmt.Fprintf(r.out, "%s%-20s  %s\n", marker, name, entry.Path)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./cmd/ledger/... -v -run "TestListRunner"
```
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/ledger/ledger.go cmd/ledger/list.go cmd/ledger/list_test.go
git commit -m "feat(ledger): add kea ledger group command and list subcommand"
```

---

## Task 5: `cmd/ledger/add.go`

**Files:**
- Create: `cmd/ledger/add.go`
- Create: `cmd/ledger/add_test.go`

- [ ] **Step 1: Write failing tests**

Create `cmd/ledger/add_test.go`:

```go
package ledger

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopDBInit is a test double that creates an empty file (simulating DB init).
func noopDBInit(path string, _ fs.FS) error {
	return os.WriteFile(path, []byte(""), 0644)
}

func TestAddRunner_AutoPath(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	runner := &addRunner{
		registry:   r,
		migrations: nil,
		appDir:     dir,
		name:       "personal",
		customPath: "",
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.NoError(t, err)
	expectedPath := filepath.Join(dir, "ledgers", "personal.db")
	entry, ok := r.Ledgers["personal"]
	require.True(t, ok, "ledger should be registered")
	assert.Equal(t, expectedPath, entry.Path)
	_, statErr := os.Stat(expectedPath)
	assert.NoError(t, statErr, "DB file should exist")
}

func TestAddRunner_CustomPath(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	customPath := filepath.Join(dir, "custom.db")

	runner := &addRunner{
		registry:   r,
		migrations: nil,
		appDir:     dir,
		name:       "custom",
		customPath: customPath,
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.NoError(t, err)
	entry, ok := r.Ledgers["custom"]
	require.True(t, ok)
	assert.Equal(t, customPath, entry.Path)
}

func TestAddRunner_CustomPath_DirNotExist(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	runner := &addRunner{
		registry:   r,
		migrations: nil,
		appDir:     dir,
		name:       "bad",
		customPath: "/nonexistent/dir/bad.db",
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory does not exist")
}

func TestAddRunner_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	runner := &addRunner{
		registry:   r,
		appDir:     dir,
		name:       "personal",
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.Error(t, err)
	assert.True(t, errors.Is(err, internalled.ErrLedgerExists))
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./cmd/ledger/... -v -run "TestAddRunner"
```
Expected: FAIL with `undefined: addRunner`.

- [ ] **Step 3: Implement add runner**

Create `cmd/ledger/add.go`:

```go
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
	dbPath := r.customPath
	if dbPath == "" {
		dbPath = filepath.Join(r.appDir, "ledgers", r.name+".db")
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./cmd/ledger/... -v -run "TestAddRunner"
```
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/ledger/add.go cmd/ledger/add_test.go
git commit -m "feat(ledger): add kea ledger add subcommand"
```

---

## Task 6: `cmd/ledger/switch.go`

**Files:**
- Create: `cmd/ledger/switch.go`
- Create: `cmd/ledger/switch_test.go`

- [ ] **Step 1: Write failing tests**

Create `cmd/ledger/switch_test.go`:

```go
package ledger

import (
	"errors"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchRunner_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))

	runner := &switchRunner{registry: r, name: "work"}
	err = runner.Run()

	require.NoError(t, err)
	assert.Equal(t, "work", r.ActiveLedger)
}

func TestSwitchRunner_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	runner := &switchRunner{registry: r, name: "ghost"}
	err = runner.Run()

	require.Error(t, err)
	assert.True(t, errors.Is(err, internalled.ErrLedgerNotFound))
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./cmd/ledger/... -v -run "TestSwitchRunner"
```
Expected: FAIL with `undefined: switchRunner`.

- [ ] **Step 3: Implement switch runner**

Create `cmd/ledger/switch.go`:

```go
package ledger

import (
	"fmt"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/spf13/cobra"
)

type switchRunner struct {
	registry *internalled.Registry
	name     string
}

func NewSwitchCmd(registry *internalled.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "Set the active ledger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &switchRunner{registry: registry, name: args[0]}
			return runner.Run()
		},
	}
}

func (r *switchRunner) Run() error {
	if err := r.registry.Switch(r.name); err != nil {
		return err
	}
	fmt.Printf("Switched to ledger %q\n", r.name)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./cmd/ledger/... -v -run "TestSwitchRunner"
```
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/ledger/switch.go cmd/ledger/switch_test.go
git commit -m "feat(ledger): add kea ledger switch subcommand"
```

---

## Task 7: `cmd/ledger/remove.go`

**Files:**
- Create: `cmd/ledger/remove.go`
- Create: `cmd/ledger/remove_test.go`

- [ ] **Step 1: Write failing tests**

Create `cmd/ledger/remove_test.go`:

```go
package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveRunner_UnregisterOnly(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Add("personal", filepath.Join(dir, "personal.db")))
	require.NoError(t, r.Switch("work"))

	runner := &removeRunner{registry: r, name: "personal", deleteFile: false, yes: true}
	err = runner.Run()

	require.NoError(t, err)
	assert.NotContains(t, r.Ledgers, "personal")
}

func TestRemoveRunner_DeleteFile(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	dbFile := filepath.Join(dir, "personal.db")
	require.NoError(t, os.WriteFile(dbFile, []byte(""), 0644))
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Add("personal", dbFile))
	require.NoError(t, r.Switch("work"))

	runner := &removeRunner{registry: r, name: "personal", deleteFile: true, yes: true}
	err = runner.Run()

	require.NoError(t, err)
	_, statErr := os.Stat(dbFile)
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestRemoveRunner_RefusesActiveLedger(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Switch("work"))

	runner := &removeRunner{registry: r, name: "work", deleteFile: false, yes: true}
	err = runner.Run()

	require.Error(t, err)
	assert.True(t, errors.Is(err, internalled.ErrRemoveActive))
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./cmd/ledger/... -v -run "TestRemoveRunner"
```
Expected: FAIL with `undefined: removeRunner`.

- [ ] **Step 3: Implement remove runner**

Create `cmd/ledger/remove.go`:

```go
package ledger

import (
	"fmt"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/ui/prompts"
	"github.com/spf13/cobra"
)

type removeFlags struct {
	DeleteFile bool
	Yes        bool
}

type removeRunner struct {
	registry   *internalled.Registry
	name       string
	deleteFile bool
	yes        bool
}

func NewRemoveCmd(registry *internalled.Registry) *cobra.Command {
	flags := &removeFlags{}
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Unregister a ledger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &removeRunner{
				registry:   registry,
				name:       args[0],
				deleteFile: flags.DeleteFile,
				yes:        flags.Yes,
			}
			return runner.Run()
		},
	}
	cmd.Flags().BoolVar(&flags.DeleteFile, "delete-file", false, "also delete the database file from disk")
	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func (r *removeRunner) Run() error {
	// Capture entry before removal so we can report the path in the success message.
	entry, entryOk := r.registry.EntryFor(r.name)

	if r.deleteFile && !r.yes {
		if !entryOk {
			return fmt.Errorf("%w: %q", internalled.ErrLedgerNotFound, r.name)
		}
		confirmed, err := prompts.PromptConfirm(
			fmt.Sprintf("Delete %s?", entry.Path),
			false,
		)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := r.registry.Remove(r.name, r.deleteFile); err != nil {
		return err
	}

	if r.deleteFile {
		fmt.Printf("Ledger %q removed and database file deleted.\n", r.name)
	} else {
		fmt.Printf("Ledger %q unregistered. Database file remains at: %s\n", r.name, entry.Path)
	}
	return nil
}
```

- [ ] **Step 4: Run all cmd/ledger tests**

```bash
go test ./cmd/ledger/... -v
```
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ledger/remove.go cmd/ledger/remove_test.go
git commit -m "feat(ledger): add kea ledger remove subcommand"
```

---

## Task 8: Wire registry into `cmd/root.go` + extend `internal/config/config.go`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Add `ActiveLedger` field to Config**

In `internal/config/config.go`, add the `ActiveLedger` field (it is set at runtime, not from YAML):

```go
type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	Defaults     DefaultsConfig `mapstructure:"defaults"`
	ConfigPath   string         `mapstructure:"-"`
	ActiveLedger string         `mapstructure:"-"` // name of the active ledger, set at startup
}
```

- [ ] **Step 2: Modify `cmd/root.go` — add imports**

In `cmd/root.go`, add these imports:

```go
import (
    // existing imports ...
    ledgercmd "github.com/hance08/kea/cmd/ledger"
    "github.com/hance08/kea/internal/ledger"
)
```

- [ ] **Step 3: Modify `Execute` in `cmd/root.go` — load registry and wire ledger commands**

Replace the body of `Execute` from after the `initConfig` call through to the `os.Exit(exitCode)` with this restructured version:

```go
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

	exitCode := func() int {
		// Ledger management commands are always available.
		rootCmd.AddCommand(ledgercmd.NewLedgerCmd(registry, migrations, appDir))

		activePath, err := registry.Active()
		if err != nil {
			// No active ledger — only ledger commands are useful.
			pterm.Warning.Println("No ledger configured. Run: kea ledger add <name>")
			if err := rootCmd.Execute(); err != nil {
				pterm.Error.Println(capitalize(err.Error()))
				return 1
			}
			return 0
		}

		// Inject the resolved DB path so app.NewApp and kea info both see it.
		cfg.Database.Path = activePath
		cfg.ActiveLedger = registry.ActiveName()

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
```

- [ ] **Step 4: Run all tests to verify nothing is broken**

```bash
go test ./...
```
Expected: All existing tests PASS. No new failures.

- [ ] **Step 5: Build and do a quick smoke test**

```bash
make build
./kea_test ledger list
./kea_test ledger add mytest
./kea_test ledger list
./kea_test ledger switch mytest
./kea_test ledger list
```
Expected: Commands run without error. `ledger list` shows `* mytest` after switch.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go cmd/root.go
git commit -m "feat(ledger): wire registry into app startup"
```

---

## Task 9: Extend `kea info` with active ledger

**Files:**
- Modify: `ui/views/system_info.go`
- Modify: `ui/views/json_types.go`
- Modify: `cmd/info.go`

- [ ] **Step 1: Add `ActiveLedger` to `SystemInfo` and update `Render`**

In `ui/views/system_info.go`, add the field and update the table:

```go
type SystemInfo struct {
	ConfigPath      string
	DBPath          string
	DBExists        bool
	DefaultCurrency string
	AppDataDir      string
	ActiveLedger    string
}

func (v *SystemInfoView) Render(info SystemInfo) error {
	dbStatus := pterm.Green("Found")
	if !info.DBExists {
		dbStatus = pterm.Red("Not Found (Will be created)")
	}

	tableData := pterm.TableData{
		{"Configuration File", info.ConfigPath},
		{"Active Ledger", info.ActiveLedger},
		{"Database Path", info.DBPath},
		{"Database Status", dbStatus},
		{"Default Currency", info.DefaultCurrency},
		{"AppData Directory", info.AppDataDir},
	}

	return pterm.DefaultTable.WithData(tableData).Render()
}
```

- [ ] **Step 2: Add `ActiveLedger` to `JSONSystemInfo` and its converter**

In `ui/views/json_types.go`, update `JSONSystemInfo` and `ToJSONSystemInfo`:

```go
type JSONSystemInfo struct {
	ConfigPath      string `json:"config_path"`
	ActiveLedger    string `json:"active_ledger"`
	DBPath          string `json:"db_path"`
	DBExists        bool   `json:"db_exists"`
	DefaultCurrency string `json:"default_currency"`
	AppDataDir      string `json:"app_data_dir"`
}

func ToJSONSystemInfo(info SystemInfo) JSONSystemInfo {
	return JSONSystemInfo{
		ConfigPath:      info.ConfigPath,
		ActiveLedger:    info.ActiveLedger,
		DBPath:          info.DBPath,
		DBExists:        info.DBExists,
		DefaultCurrency: info.DefaultCurrency,
		AppDataDir:      info.AppDataDir,
	}
}
```

- [ ] **Step 3: Pass `ActiveLedger` from config in `cmd/info.go`**

In `cmd/info.go`, update the `Run` method to include `ActiveLedger`:

```go
func (r *infoRunner) Run() error {
	configPath := r.svc.Config().ConfigPath
	if configPath == "" {
		configPath = "(None, using defaults)"
	}

	rawDBPath := r.svc.Config().Database.Path
	if rawDBPath == "" {
		appDir := getAppDataDirOrPanic()
		rawDBPath = filepath.Join(appDir, "kea.db")
	}
	expandedDBPath, _ := expandPath(rawDBPath)

	dbExists := false
	if _, err := os.Stat(expandedDBPath); err == nil {
		dbExists = true
	}

	info := views.SystemInfo{
		ConfigPath:      configPath,
		ActiveLedger:    r.svc.Config().ActiveLedger,
		DBPath:          expandedDBPath,
		DBExists:        dbExists,
		DefaultCurrency: r.svc.Config().Defaults.Currency,
		AppDataDir:      getAppDataDirOrPanic(),
	}

	if r.json {
		return views.WriteJSON(views.ToJSONSystemInfo(info))
	}
	return r.view.Render(info)
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```
Expected: All tests PASS.

- [ ] **Step 5: Build and verify `kea info` output**

```bash
make build
./kea_test info
```
Expected: Table includes an `Active Ledger` row showing the current ledger name.

- [ ] **Step 6: Commit**

```bash
git add ui/views/system_info.go ui/views/json_types.go cmd/info.go
git commit -m "feat(ledger): show active ledger name in kea info"
```
