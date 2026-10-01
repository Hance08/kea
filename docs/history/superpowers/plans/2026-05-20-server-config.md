# Server Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ServerConfig` (host, port, CORS origins) to the Config struct with Viper defaults and env var support (#117).

**Architecture:** Add a `ServerConfig` sub-struct to `Config`, set Viper defaults in `initConfig`, and update `NewDefault()` to include the server defaults. Env vars `KEA_SERVER_HOST`, `KEA_SERVER_PORT`, `KEA_SERVER_CORS_ORIGINS` work automatically via the existing `KEA` prefix + `AutomaticEnv()`.

**Tech Stack:** Go, Viper, mapstructure

---

### Task 1: Add ServerConfig struct and update NewDefault

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestNewDefault_ServerConfig(t *testing.T) {
	cfg := NewDefault()

	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host %q, got %q", "localhost", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port %d, got %d", 8080, cfg.Server.Port)
	}
	if len(cfg.Server.CORSOrigins) != 1 || cfg.Server.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("expected default CORS origins [http://localhost:5173], got %v", cfg.Server.CORSOrigins)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestNewDefault_ServerConfig -v`
Expected: FAIL — `cfg.Server` does not exist yet.

- [ ] **Step 3: Add ServerConfig struct and wire into Config**

In `internal/config/config.go`, add the `ServerConfig` struct and the `Server` field on `Config`:

```go
type ServerConfig struct {
	Host        string   `mapstructure:"host"`
	Port        int      `mapstructure:"port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	Defaults     DefaultsConfig `mapstructure:"defaults"`
	Server       ServerConfig   `mapstructure:"server"`
	ConfigPath   string         `mapstructure:"-"`
	ActiveLedger string         `mapstructure:"-"`
}
```

Update `NewDefault`:

```go
func NewDefault() *Config {
	return &Config{
		Database: DatabaseConfig{Path: ""},
		Defaults: DefaultsConfig{},
		Server: ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: []string{"http://localhost:5173"},
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestNewDefault_ServerConfig -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add ServerConfig struct with host, port, CORS origins (#117)"
```

---

### Task 2: Set Viper defaults for server fields

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/root_server_config_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfig_ServerDefaults(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")

	setServerDefaults(v)

	if v.GetString("server.host") != "localhost" {
		t.Errorf("expected server.host %q, got %q", "localhost", v.GetString("server.host"))
	}
	if v.GetInt("server.port") != 8080 {
		t.Errorf("expected server.port %d, got %d", 8080, v.GetInt("server.port"))
	}
	origins := v.GetStringSlice("server.cors_origins")
	if len(origins) != 1 || origins[0] != "http://localhost:5173" {
		t.Errorf("expected server.cors_origins [http://localhost:5173], got %v", origins)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestInitConfig_ServerDefaults -v`
Expected: FAIL — `setServerDefaults` does not exist.

- [ ] **Step 3: Add setServerDefaults and call it from initConfig**

In `cmd/root.go`, add the helper function:

```go
func setServerDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.cors_origins", []string{"http://localhost:5173"})
}
```

In `initConfig`, call `setServerDefaults(viper.GetViper())` right before `viper.SetEnvPrefix("KEA")` (line 287):

```go
	setServerDefaults(viper.GetViper())

	viper.SetEnvPrefix("KEA")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestInitConfig_ServerDefaults -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: All tests pass — no regressions.

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/root_server_config_test.go
git commit -m "feat(cmd): set Viper defaults for server config fields (#117)"
```

---

### Task 3: Update default config template

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Update the defaultConfigTemplate**

In `cmd/root.go`, add the server section to `defaultConfigTemplate`:

```go
const defaultConfigTemplate = `# kea configuration file

database:
  # Path to the SQLite database file.
  path: ""

defaults:
  # Default currency code (ISO 4217), e.g. USD, TWD, JPY, EUR
  currency: ""

server:
  # Host to bind the web server to
  host: "localhost"
  # Port to listen on
  port: 8080
  # Allowed CORS origins
  cors_origins:
    - "http://localhost:5173"
`
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "docs(config): add server section to default config template (#117)"
```
