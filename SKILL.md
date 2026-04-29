# SKILL.md — KEA Agent Skill Reference

This document teaches an AI agent how to operate KEA, a CLI personal double-entry accounting tool. Use this as the primary reference when invoking `kea` commands or reasoning about financial data.

---

## What KEA Is

KEA manages personal finances using **double-entry bookkeeping**. Every transaction has two or more "splits" that sum to zero — money always moves *from* somewhere *to* somewhere. Data is stored in a local SQLite database.

---

## Core Concepts

### Account Types
| Code | Name        | Description                          | Root name     |
|------|-------------|--------------------------------------|---------------|
| A    | Asset       | Things you own (bank accounts, cash) | Assets        |
| L    | Liability   | Things you owe (loans, credit cards) | Liabilities   |
| C    | Equity      | Net worth / opening balances         | Equity        |
| R    | Revenue     | Income sources                       | Revenue       |
| E    | Expense     | Spending categories                  | Expenses      |

### Account Names
Accounts use colon-separated paths: `Assets:Bank:Checking`, `Expenses:Food:Dining`.
- **Leaf accounts** hold transactions; parent accounts aggregate only.
- System accounts named `Equity:OpeningBalances_<CCY>` (e.g. `Equity:OpeningBalances_USD`, `Equity:OpeningBalances_TWD`) are created automatically per currency and must not be deleted. The legacy unsuffixed name `Equity:OpeningBalances` is auto-migrated to the per-currency form on first run.

### Amounts
- All amounts are stored as **integer cents** internally.
- Display format: `"150.50"` for $150.50, `"1"` for $1.00 (trailing zeros trimmed).
- Use the same string format for CLI flags.

### Transaction Status
| Value | Name       | Editable? |
|-------|------------|-----------|
| 0     | Pending    | Yes       |
| 1     | Cleared    | Yes       |
| 2     | Reconciled | No (locked) |

### Protected Records
- **Transaction ID 1** (opening balance transaction) is immutable.
- **Reconciled transactions** (status 2) cannot be edited or deleted.

### JSON Output
Most commands accept `--json` / `-j` for machine-readable output: `account create`, `account list`, `account edit`, `add`, `transaction list`, `transaction show`, `report`, `reconcile`, `info`. Combine with the global `--no-color` flag when capturing output for further processing.

---

## CLI Reference

All commands follow: `kea [global-flags] <command> [subcommand] [flags]`

**Global flags:**
- `--config, -c <path>` — Override config file path
- `--no-color` — Disable color output (use for scripting/piping)

---

### Account Commands

#### Create an account
```bash
kea account create \
  --name <leaf-name> \
  --type <A|L|C|R|E> \
  --parent <parent-full-name> \
  --balance <amount> \
  --currency <ISO-code> \
  --description <text>
```

- `--parent` is required for sub-accounts; omit for root accounts.
- `--balance` sets the opening balance (only valid for A and L accounts).
- `--type` is required when creating a root account; inferred from parent for sub-accounts.

Examples:
```bash
# Create a root asset account
kea account create --name "Assets" --type A --currency TWD

# Create a bank sub-account with an opening balance
kea account create --name "TaiShin" --parent "Assets:Bank" --balance 50000 --currency TWD --description "TaiShin checking"

# Create an expense category
kea account create --name "Dining" --parent "Expenses:Food" --currency TWD
```

#### List accounts
```bash
kea account list
kea account list --type A          # Filter by type
kea account list --show-hidden     # Include archived accounts
```

#### Edit an account
```bash
kea account edit <full-account-name> [--name <new-leaf>] [--desc <text>] [--hidden | --no-hidden]
```

- `--name` replaces only the **last segment** of the path; the parent prefix is preserved. Renaming a parent cascades to all descendants.
- `--hidden` and `--no-hidden` are mutually exclusive.

Examples:
```bash
kea account edit "Assets:Bank" --name "Banking"
kea account edit "Assets:Bank:TaiShin" --desc "Main checking account"
kea account edit "Expenses:Food:Dining" --hidden
```

#### Delete an account
```bash
kea account delete <full-account-name>
kea account delete "Expenses:Food:Dining" --yes   # Skip confirmation
```

Deletion fails if: account has child accounts, has any transactions, or is a system account.

---

### Quick Transaction Entry

Two-leg form (most common):
```bash
kea add \
  --from <source-account> \
  --to <destination-account> \
  --amount <amount> \
  --desc <description> \
  --date <YYYY-MM-DD> \
  --status <pending|cleared> \
  --type <expense|income|transfer> \
  [--json]
```

- `--date` defaults to today.
- `--status` defaults to `cleared`.
- `--from` is where money leaves; `--to` is where it goes.
- `--json` / `-j` prints the created transaction as JSON (requires flag mode).

Multi-leg form (manual splits):
```bash
kea add --desc <description> \
  --split "<account>=<signed-amount>" \
  --split "<account>=<signed-amount>" \
  [--split "<account>=<signed-amount>" ...] \
  [--date <YYYY-MM-DD>] [--status ...] [--type ...] [--json]
```

- Each `--split` is repeatable. Amounts are signed: positive into the account, negative out. The sum of all split amounts must be zero (double-entry).
- `--split` is mutually exclusive with `--from`, `--to`, `--amount`.

Examples:
```bash
# Record an expense: money leaves bank, goes to food expense
kea add --from "Assets:Bank:TaiShin" --to "Expenses:Food:Dining" \
  --amount 350 --desc "Lunch with team" --date 2026-03-23

# Record income: money from revenue into bank
kea add --from "Revenue:Salary" --to "Assets:Bank:TaiShin" \
  --amount 85000 --desc "March salary"

# Transfer between accounts
kea add --from "Assets:Bank:TaiShin" --to "Assets:Wallet" \
  --amount 2000 --desc "ATM withdrawal" --type transfer

# Multi-leg split: pay rent partly from bank, partly from wallet
kea add --desc "April rent" \
  --split "Expenses:Housing:Rent=20000" \
  --split "Assets:Bank:TaiShin=-15000" \
  --split "Assets:Wallet=-5000"
```

---

### Transaction Commands

#### List transactions
```bash
kea transaction list
kea transaction list --account "Assets:Bank:TaiShin" --limit 50
```

#### Show a transaction
```bash
kea transaction show <id>
```
Displays all splits with account names, amounts, and currencies.

#### Delete a transaction
```bash
kea transaction delete <id>
```
Fails for transaction ID 1 and reconciled transactions.

#### Clear a transaction (mark as cleared)
```bash
kea transaction clear <id>
```

#### Edit a transaction (interactive)
```bash
kea transaction edit <id>
```
Launches interactive menu for editing description, date, status, and splits.

---

### Reports

```bash
kea report --type <is|ib|eb|bs> [--month YYYY-MM | --from YYYY-MM-DD --to YYYY-MM-DD] [--json]
```

| Type | Name               | Description                                     |
|------|--------------------|-------------------------------------------------|
| `is` | Income Statement   | Income vs. expenses for a period (default)      |
| `ib` | Income Breakdown   | Income sources ranked by amount                 |
| `eb` | Expense Breakdown  | Expense categories ranked by amount             |
| `bs` | Balance Sheet      | Current snapshot of assets, liabilities, equity |

Examples:
```bash
# Income statement for March 2026
kea report --type is --month 2026-03

# Expense breakdown with custom date range
kea report --type eb --from 2026-01-01 --to 2026-03-31

# Balance sheet as JSON (for further processing)
kea report --type bs --json

# Income statement for a specific period as JSON
kea report --type is --from 2026-01-01 --to 2026-03-31 --json
```

---

### Reconcile an Account

Reconciliation marks transactions as locked against an external statement (status 2 / Reconciled). Reconciled transactions cannot be edited or deleted afterward.

```bash
# Interactive (default): prompts for statement balance, lets you tick transactions
kea reconcile <account-name>

# Non-interactive / agent mode: all flags required (except --force)
kea reconcile <account-name> \
  --balance <statement-ending-balance> \
  --ids <id1,id2,id3,...> \
  [--force] [--json]
```

- `--balance` — the statement's ending balance, e.g. `2450.00`.
- `--ids` — comma-separated transaction IDs to mark as reconciled.
- `--force` — skip the balance-mismatch warning when the selected IDs don't reconcile to `--balance`.
- `--json` / `-j` — output the result as JSON.

Example:
```bash
kea reconcile "Assets:Bank:TaiShin" --balance 24500.00 --ids 12,15,18 --json
```

---

### Ledgers (multiple databases)

A "ledger" is an independent SQLite database. Each ledger has its own accounts, transactions, and currencies. Exactly one ledger is active at a time; all other commands operate on the active ledger.

```bash
# List all registered ledgers (the active one is marked)
kea ledger list

# Register a new ledger and initialise its database
kea ledger add <name>                       # default location: <appdata>/ledgers/<name>.db
kea ledger add <name> --path /custom/path.db

# Switch the active ledger
kea ledger switch <name>

# Unregister a ledger (does not delete the DB file)
kea ledger remove <name>
```

Notes:
- On first run, an existing legacy `kea.db` is auto-registered as a ledger named `default`.
- `kea info` reports the active ledger name and resolved DB path.

---

### System Info

```bash
kea info [--json]
```
Shows config path, active ledger name, database path, and default currency.

---

## Common Workflows

### Set up a new account hierarchy

```bash
# 1. Create parent categories first
kea account create --name "Assets" --type A --currency TWD
kea account create --name "Bank" --parent "Assets" --currency TWD
kea account create --name "Expenses" --type E --currency TWD
kea account create --name "Food" --parent "Expenses" --currency TWD

# 2. Create leaf accounts (these hold transactions)
kea account create --name "TaiShin" --parent "Assets:Bank" --balance 100000 --currency TWD --description "TaiShin Bank Account"
kea account create --name "Dining" --parent "Expenses:Food" --currency TWD
```

### Record daily transactions

```bash
# Expense
kea add --from "Assets:Bank:TaiShin" --to "Expenses:Food:Dining" --amount 250 --desc "Breakfast"

# Income
kea add --from "Revenue:Salary" --to "Assets:Bank:TaiShin" --amount 85000 --desc "March salary" --date 2026-03-05

# Transfer
kea add --from "Assets:Bank:TaiShin" --to "Assets:Wallet" --amount 3000 --desc "ATM"
```

### Review finances

```bash
# See recent transactions
kea transaction list --limit 10

# Monthly income statement
kea report --type is --month 2026-03

# Check current net worth
kea report --type bs
```

---

## Business Rules to Follow

1. **Only use leaf accounts for transactions.** Do not use a parent account like `Assets:Bank` directly — use a leaf like `Assets:Bank:TaiShin`.

2. **Amount direction:**
   - For expenses: `--from` = Asset account, `--to` = Expense account
   - For income: `--from` = Revenue account, `--to` = Asset account
   - For transfers: `--from` = source Asset/Liability, `--to` = destination Asset/Liability
   - For liability payments: `--from` = Asset account, `--to` = Liability account (reduces debt)

3. **Opening balances** are handled automatically by `--balance` on account creation. Only Asset and Liability accounts support opening balances.

4. **Never delete or try to edit** transaction ID 1 or any reconciled transaction (status 2).

5. **Do not delete** any `Equity:OpeningBalances_<CCY>` system account (e.g. `Equity:OpeningBalances_USD`). One exists per currency you've used.

6. **Currency codes** must be exactly 3 uppercase letters (e.g., `TWD`, `USD`, `JPY`).

7. **Account names** must not contain `:`, cannot be reserved words (Assets, Liabilities, Equity, Revenue, Expenses), and must not exceed 100 characters.

---

## JSON Output for Programmatic Use

Add `--json` to report commands for machine-readable output. Combined with `--no-color`, this gives clean JSON:

```bash
kea --no-color report --type is --month 2026-03 --json
```

JSON report fields:
- `period` — Date range description
- `total_income` — Income in cents
- `total_expense` — Expense in cents
- `net_amount` — Income minus expense in cents
- `net_worth` — Assets minus liabilities in cents
- `currency` — Currency code
- `income_rows` / `expense_rows` — Array of `{ account_name, offset_account, amount, currency, tx_count }`

---

## Error Patterns

| Situation | Error message pattern | Action |
|-----------|----------------------|--------|
| Account not found | `"account not found"` | Check name spelling with `kea account list` |
| Using a parent account | `"account is a parent account"` | Use a leaf account instead |
| Deleting protected transaction | `ErrNotEditable` or `ErrReconciled` | Cannot delete; operation is forbidden |
| Account has transactions | Cannot delete | Use `kea transaction list --account <name>` to review |
| Account has children | Cannot delete | Delete children first |
| Splits don't balance | `"splits do not balance"` | Check that from/to amounts match |
| Invalid currency | `"currency code must be 3 characters"` | Use ISO 4217 code (e.g., TWD) |

---

## Tips for Agent Use

- Use `--no-color` when capturing output for further processing.
- Always verify an account exists with `kea account list` before referencing it.
- Use `kea transaction list --account <name>` to audit account history before deletion.
- When recording a batch of transactions, process them sequentially — each `kea add` is atomic.
- Use `kea report --type bs --json` to get a snapshot of current balances programmatically.
- Date format is always `YYYY-MM-DD`; month format for `--month` is `YYYY-MM`.
