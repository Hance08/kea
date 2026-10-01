# Recipe: Add a CLI command or flag

> **Read this when:** you need a new `kea` subcommand, or a new flag (with or without an interactive prompt) on an existing one.
>
> **Related:** [architecture.md](../architecture.md), [development.md](../development.md), [domain.md](../domain.md), [add-service-method.md](add-service-method.md), [add-api-endpoint.md](add-api-endpoint.md), [SKILL.md](../../SKILL.md)

## When to use
- A user-facing CLI/TUI operation or option is missing and the service method already exists (or you add it first).
- Do NOT use this for business logic; see [add-service-method.md](add-service-method.md).
- Do NOT use this for HTTP or SPA changes; see [add-api-endpoint.md](add-api-endpoint.md). The CLI request flow is in [architecture.md](../architecture.md).

## Steps
1. Pick where the command lives.
   - Subcommand of a group: `cmd/account/` (see `NewAccountCmd` in `cmd/account/account.go`) or `cmd/transaction/`. Register with `AddCommand` in the group constructor.
   - Top-level command: a `NewXxxCmd` in `cmd/`, registered in `Execute` in `cmd/root.go`.
   - Commands that need the database go after `app.NewApp` in `Execute`. Only `ledger` commands (`cmd/ledger/`) are registered before it, so they work with no active ledger.
2. Define the narrow provider interface in the command file, naming only the service methods the runner calls.
   - Examples: `AccountSearchProvider` in `cmd/account/search.go`, `InfoProvider` in `cmd/info.go`. The constructor passes `svc.Account()` or `svc.Transaction()`.
   - Add a view interface too when the runner renders through `ui/views` (`ShowView` in `cmd/transaction/show.go`), so tests can capture output.
3. Write the runner struct and `Run` method; keep cobra out of `Run` where possible. Choose the flag pattern below.
4. Write `NewXxxCmd`: `Use` with `<arg>` placeholders, `Short`, `Long`, `Args` validator, flags, and a `RunE` that builds the runner and calls `Run(cmd.Context(), ...)`.
   - Add `--json` / `-j` for any command that prints data or a result.
5. Render output.
   - Human output: a view in `ui/views` (pterm tables, detail views). Interactive input: a huh prompt in `ui/prompts`.
   - JSON output: add a `JSONXxx` type and `ToJSONXxx` converter in `ui/views/json_types.go`, then call `views.WriteJSON` (`ui/views/json.go`).
6. Large commands split into `x.go` (cobra wiring), `x_types.go` (provider/view interfaces, flags, input structs) and `x_actions.go` (runner logic).
   - Used by `add`, `report`, `reconcile`, `account create`, `account edit` and `transaction edit`. Small commands (`list`, `delete`, `search`, `show`, `clear`, `info`) stay in one file.
7. Write the tests, update docs, and run the checklist.

## Flag-handling patterns
Pick one per command and never mix them. The 3 vs 1-2 flag split is intentional, not inconsistency.
- Pattern A: store the whole flags struct in the runner (`runner{svc, flags *xxxFlags}`). Use for 3+ flags read in several places in `Run`.
  - Examples: `listRunner` in `cmd/account/list.go`, `listRunner` in `cmd/transaction/list.go`, `searchRunner` in `cmd/account/search.go`.
- Pattern B: copy individual fields into the runner (`runner{svc, json bool, yes bool}`). Use for 1-2 flags.
  - Examples: `deleteRunner` in `cmd/account/delete.go`, `showRunner` in `cmd/transaction/show.go`, `clearRunner` in `cmd/transaction/clear.go`, `infoRunner` in `cmd/info.go`.
  - Flags still bind to a local `xxxFlags` struct in the constructor; only the values are copied.
- Pattern C: pass flags straight to `Run(ctx, flags, cmd)`. Use when `Run` needs `cmd.Flags().Changed()` to choose interactive vs flag mode.
  - Examples: `addRunner` in `cmd/add.go`, `createRunner` in `cmd/account/create.go`.
- A tri-state bool (unset / true / false) needs `Changed`: `RunE` sets `RegularSet` and `Regular` on the flags struct before calling `Run`. See `--regular` in `cmd/add.go` and `cmd/transaction/list.go`.

## Worked example
Two commits cover a new command; two more cover a new flag.
- `970195a` added `kea account search` as one file, `cmd/account/search.go`, plus one `AddCommand` line in `cmd/account/account.go`.
  - Provider interface, `searchFlags` (Pattern A), `--json` through `views.ToJSONAccount` and `views.WriteJSON`, table through `views.NewAccountListView`.
- `36b7e8b` removed the `-c` shorthand from `--currency`. Lesson: the root command defines persistent `-c/--config` (`cmd/root.go`), so a shorthand must not collide with it or with the command's own flags.
- `43f1077` added `--regular` to `kea add`.
  - `cmd/add.go`: declare the flag, set `RegularSet`/`Regular` via `Changed`. `cmd/add_types.go`: new fields on `addFlags` and a `Regular *bool` on `addTransactionInput`.
  - `cmd/add_actions.go`: honor it in `runFromFlags` and `runFromSplitFlags`, warn on stderr when the type is not Income/Expense, and ask in `runInteractive`.
  - `ui/prompts/transaction.go`: new `PromptRegular`. Both modes must work: flags for agents and scripts, prompts for people.
- `e8f2331` added a `--regular` filter to `kea transaction list`.
  - New flag fields, a filter passed to the service, `Regular` on `views.JSONTransaction` in `ui/views/json_types.go`, and a `Reg` column in `ui/views/transaction_list.go`.

## Conventions
- Interactive vs non-interactive: if a command prompts when flags are absent, give it a complete flag mode that never prompts. Agents and scripts rely on it; see [SKILL.md](../../SKILL.md).
  - Destructive commands take `--yes`/`-y`; `account delete` treats `--json` as implying `--yes`.
- Errors: return wrapped errors (`fmt.Errorf("...: %w", err)`); never print and exit inside `Run`. `Execute` prints them with pterm and exits 1.
- Amounts: parse input with `utils.ParseAmount`, print with `utils.FormatAmount`; stored values are cents. See [domain.md](../domain.md).
- JSON: emit one document through `views.WriteJSON`, with nothing else on stdout. Human-only messages go through pterm or stderr.
- Help text: write `Long` with examples for non-obvious flags; flag descriptions start with a capital letter or a verb and name the allowed values.
- Tests: `package cmd` or the subpackage, with fakes for the provider and view interfaces; test `Run` directly. See `cmd/info_test.go` and the cmd tests section in [development.md](../development.md).

## Checklist
- [ ] Runner tests with fake providers (success, invalid input, `--json` path) next to the command
- [ ] No flag shorthand collides with root `-c` or another flag on the command
- [ ] Update SKILL.md if agents should know about the command/flag
- [ ] Update the matching doc in the same commit
- [ ] `go test ./...` passes
