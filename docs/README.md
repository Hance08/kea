# kea Docs

> **Read this when:** you are new to (or returning to) the codebase and need to find the right doc.
>
> **Related:** [../AGENTS.md](../AGENTS.md) · [../SKILL.md](../SKILL.md)

## Start here
1. [architecture.md](architecture.md) — layers, request flow, package map, startup.
2. [domain.md](domain.md) — the accounting rules every change must respect.
3. [development.md](development.md) — setup, running, testing, deploying.

## If you want to…

| I want to… | Read |
|---|---|
| Understand how a command or request reaches the DB | [architecture.md](architecture.md) |
| Know what a transaction type / reconcile / opening balance is | [domain.md](domain.md) |
| Add or change a database column or table | [recipes/add-migration.md](recipes/add-migration.md) |
| Add business logic or a repository method | [recipes/add-service-method.md](recipes/add-service-method.md) |
| Add or change an HTTP endpoint | [recipes/add-api-endpoint.md](recipes/add-api-endpoint.md) · [http-api.md](http-api.md) |
| Add a CLI command or flag | [recipes/add-cli-command.md](recipes/add-cli-command.md) |
| Add a web UI page | [recipes/add-spa-page.md](recipes/add-spa-page.md) · [../spa/README.md](../spa/README.md) (SPA routes, commands) |
| Know why something was built this way | [decisions.md](decisions.md) |
| Read the original design of a past feature | [history/](history/) |
| Operate the kea CLI as an agent | [../SKILL.md](../SKILL.md) |

## Layout
- `docs/*.md` — current reference; kept in sync with code.
- `docs/recipes/` — step-by-step guides built from real past commits.
- `docs/history/` — archived specs and plans. Point-in-time records; they may be out of date. `docs/history/` has three layouts from different periods: root files (June 2026 tweaks and dashboard), `web-layer/`, and `superpowers/`. New specs and plans go to `docs/history/superpowers/specs/` and `docs/history/superpowers/plans/`.

## Keeping docs current
Run `scripts/check-docs.sh` after editing docs. When a change alters a pattern, endpoint, or domain rule, update the matching doc in the same commit.
