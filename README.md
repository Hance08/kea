# kea

kea is a personal double-entry accounting tool. Use it from the terminal (CLI and TUI) or run `kea serve` for a web UI. Each ledger is a local SQLite database.

## Quick start

```bash
make spa-install      # once, on a fresh clone (needs Node)
make build-all        # build the SPA and the kea binary (needs Go and Node)
./kea --help
./kea serve           # web UI at http://localhost:8080
```

A plain `make build` skips the SPA, so `kea serve` then shows only a placeholder page.

Or with Docker for development: `docker compose up -d` (API on :8080, SPA dev server on :5173).

## Documentation

- [docs/README.md](docs/README.md) — start here: architecture, domain rules, development, recipes, decisions.
- [AGENTS.md](AGENTS.md) — hard rules and a task-to-doc index (for AI agents and contributors).
- [SKILL.md](SKILL.md) — how to operate the kea CLI as an agent.

## License

See [LICENSE](LICENSE).
