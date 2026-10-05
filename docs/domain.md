# Domain Model

> **Read this when:** you touch money, accounts, transactions, reconciliation, or reports — or need to know which invariant a change might break.
>
> **Related:** [architecture.md](architecture.md) · [decisions.md](decisions.md)

## Double-entry in one paragraph

Every event that moves money is a **transaction** made of two or more **splits**. Each split posts a signed amount to exactly one account. Money is never created or destroyed inside a transaction: whatever leaves one account must arrive in another, so the split amounts of every transaction sum to zero. A positive amount is a debit (asset or expense goes up), a negative amount is a credit (asset goes down, liability or revenue goes up). An account's balance is simply the sum of all its splits.

Worked example — buying lunch for 12.50 with cash:

| Account | Amount (cents) | Sign | Meaning |
| --- | --- | --- | --- |
| `Expenses:Food:Dining` | 1250 | + | spending recorded |
| `Assets:Wallet` | -1250 | - | cash left the wallet |
| **Sum** | **0** | | balanced |

`kea add --from Assets:Wallet --to Expenses:Food:Dining --amount 12.50` produces exactly these two splits: the "to" account gets `+amount`, the "from" account gets `-amount` (`internal/service/transaction_ops.go`, `TransactionService.CreateSimpleTransaction`).

Code lives in: `internal/model/transaction.go` (`Transaction`, `Split`), `internal/service/transaction_validation.go` (`ValidateSplitsBalance`), `internal/service/transaction_ops.go` (`CreateSimpleTransaction`, `CreateTransaction`).

## Amounts

- Every amount is an `int64` count of cents (`model.CentsPerUnit` = 100). There are no floats in storage or business logic; `splits.amount` is an `INTEGER` column.
- Splits store the raw signed value. The normal (healthy) sign depends on the account type:

| Type | Normal sign of balance | Example |
| --- | --- | --- |
| Asset (A) | positive | bank balance 1,000 → `100000` |
| Liability (L) | negative | credit-card debt 300 → `-30000` |
| Equity (C) | negative (mirror of opening assets) | `Equity:OpeningBalances_USD` → `-100000` |
| Revenue (R) | negative | salary earned → `-850000` |
| Expense (E) | positive | groceries → `4500` |

- Presentation flips signs where humans expect positives: the balance sheet shows liabilities as `-balance`, income/expense reports use absolute values, and the monthly balance history flips liabilities. Never flip signs when writing splits.
- `utils.FormatAmount` renders cents for display: thousands separators, trailing zeros trimmed (`100` → `"1"`, `1250` → `"12.5"`, `1205` → `"12.05"`, `123456789` → `"1,234,567.89"`). It panics on `math.MinInt64`.
- `utils.ParseAmount` parses user input into cents: optional leading `-`, digits, optional `.` and fraction. One fractional digit is padded (`"150.5"` → `15050`); more than two digits round half-up on the third digit only (`"1.005"` → `101`). Commas and other non-digits are rejected. Values beyond the `int64` range return `utils.ErrAmountOverflow`.

Code lives in: `internal/utils/amount.go` (`FormatAmount`, `ParseAmount`), `internal/utils/math.go` (`AbsInt64`), `internal/model/types.go` (`CentsPerUnit`).

## Accounts

| Code | `model.AccountType` constant | Root name | Normal sign |
| --- | --- | --- | --- |
| `A` | `AccountTypeAsset` | `Assets` | + |
| `L` | `AccountTypeLiability` | `Liabilities` | - |
| `C` | `AccountTypeEquity` | `Equity` | - |
| `R` | `AccountTypeRevenue` | `Revenue` | - |
| `E` | `AccountTypeExpense` | `Expenses` | + |

- **Names are full colon paths** stored in `accounts.name` (unique), e.g. `Assets:Bank:Checking`. The first segment must be one of the five root names (case-insensitive, `model.ReservedNames`) and must agree with the account type (`model.AccountTypeFromRootName`). Root names may not appear as deeper segments. No segment may be empty, contain `:`, or have leading/trailing spaces; the full name and each segment are limited to `model.AccountNameMaxLength` (100) characters.
- **Hierarchy:** `parent_id` points to the parent. A child's name must be exactly `<parent name>:<one segment>`, and its type must equal the parent's type. The parent chain is walked to reject cycles and chains deeper than 100 (`ErrCircularParent`).
- **Only leaf accounts hold splits.** An account that already has transactions cannot become a parent, and `TransactionService.checkAccountSelectable` rejects parent accounts when a split is created or moved to a different account. Parents only aggregate in views.
- **Hidden accounts** (`is_hidden`) are archived: they keep their history and still appear in the balance sheet, but cannot receive new splits and are omitted from lists unless requested. Toggle with `AccountService.UpdateAccountMetadata`.
- **Currency** is per account, a 3-letter uppercase code (`AccountService.ValidateCurrency`). An empty currency means "use `config.Defaults.Currency`"; splits copy the account's currency at creation time.
- **Rename** changes only the last segment (`AccountService.RenameAccount`); the store cascades the new prefix to all descendants (`internal/store/sqlite_account.go`, `Store.RenameAccount`).
- **Delete** is refused for system accounts (`ErrNotEditable`), accounts with children, and accounts with transactions (`AccountService.DeleteAccountByName`). The DB also enforces `ON DELETE RESTRICT` from splits to accounts.
- Migration `migrations/0007_add_account_type_check.up.sql` adds `CHECK(type IN ('A','L','C','R','E'))`.

Code lives in: `internal/model/account.go` (`Account`), `internal/model/input.go` (`CreateAccountInput`), `internal/model/types.go`, `internal/service/account_validation.go` (`ValidateAccountName`, `ValidateFullAccountName`, `ValidateCurrency`), `internal/service/account_ops.go` (`CreateAccount`, `validateAccountFields`, `validateParentChain`, `validateParentType`), `internal/service/account_service.go`.

Metadata updates go through `AccountService.UpdateAccountMetadata` (description and hidden flag as plain arguments; there is no metadata input struct).

## Transactions and splits

`model.Transaction` fields: `ID`, `Timestamp` (Unix seconds), `Description` (required, max 500 chars), `Status`, `Type`, `Regular`, `ExternalID`. `model.Split` fields: `TransactionID`, `AccountID`, `Amount` (cents), `Currency`, `Memo` (max 200 chars). Splits are deleted with their transaction (`ON DELETE CASCADE`).

Invariants enforced on create (`TransactionService.CreateTransaction`) and full update (`TransactionService.UpdateTransactionComplete`):

- At least `model.MinSplitsCount` (2) splits.
- Splits sum to zero **and all use the same currency** — a single transaction never mixes currencies (`TransactionService.ValidateSplitsBalance`, `TransactionService.ValidateSplitDetailsBalance`).
- Every split account exists. On create, every split account must also be a leaf and not hidden; on update, that check applies only to new splits and splits moved to a different account, so existing splits on a since-hidden account stay valid.
- The splits match the declared type (`TransactionService.ValidateSplitsMatchType`, see below).
- The `Regular` invariant holds (see Regular attribute).
- Both operations run inside `TransactionManager.ExecTx`, so the transaction row and its splits are written atomically.

**Status** (`model.TransactionStatus`, stored as an integer):

| Value | Constant | Set by |
| --- | --- | --- |
| 0 | `StatusPending` | user on create/update |
| 1 | `StatusCleared` | user on create/update (`kea transaction clear`); opening balances |
| 2 | `StatusReconciled` | only the reconcile workflow |

Create and update accept only Pending or Cleared; `TransactionService.UpdateTransactionStatus` likewise refuses to set Reconciled.

**external_id** (`migrations/0002_add_external_id.up.sql`) is an optional unique key for de-duplicating imported transactions. A duplicate insert fails with `store.ErrTransactionExists`, surfaced as `service.ErrAlreadyExists`. No current service path sets it.

Code lives in: `internal/model/transaction.go`, `internal/model/input.go`, `internal/service/transaction_ops.go`, `internal/service/transaction_validation.go`, `internal/store/sqlite_transaction.go`.

## Transaction types

The type is **stored** in `transactions.type` (`migrations/0005_add_transaction_type.up.sql`) and read back as-is; it is not recomputed when listing. It is chosen once — by the caller, or by `TransactionService.DetermineType` when `CreateSimpleTransaction` gets no type — and only changes when a user edits it.

| Constant | Value | Typical shape |
| --- | --- | --- |
| `TxTypeExpense` | `Expense` | Expense account + Asset/Liability |
| `TxTypeIncome` | `Income` | Revenue account + Asset/Liability |
| `TxTypeTransfer` | `Transfer` | only Asset/Liability accounts |
| `TxTypeOpening` | `Opening` | account + `Equity:OpeningBalances_<CCY>`, memo `Opening Balance` |
| `TxTypeDeposit` | `Deposit` | Equity + Asset/Liability, asset increases |
| `TxTypeWithdrawal` | `Withdrawal` | Equity + Asset/Liability, no asset increase |
| `TxTypeInvestment` | `Investment` | an `Assets:Investments:*` account + another Asset/Liability (the cash side) |
| `TxTypeOther` | `Other` | anything else |

`DetermineType` (`internal/service/transaction_classifier.go`) applies these rules in order; the first match wins:

1. Any split memo equals `model.OpeningAccountMemo` → Opening.
2. An `Assets:Investments:*` split (`model.IsInvestmentAccount`) plus at least one other Asset/Liability split → Investment.
3. Expense and Revenue both present → Income if total revenue >= total expense, else Expense.
4. Expense plus two or more Asset/Liability splits → Transfer if the positive Asset/Liability total exceeds the expense total (e.g. a transfer with a fee), else Expense.
5. Expense plus exactly one Asset/Liability → Expense.
6. Revenue plus at least one Asset/Liability → Income.
7. Two or more Asset/Liability → Transfer.
8. Equity plus Asset/Liability → Deposit if an asset split is positive, else Withdrawal.
9. Otherwise (including no splits) → Other.

`ValidateSplitsMatchType` checks a declared type against the splits: Expense needs an Expense and an Asset/Liability account; Income needs a Revenue and an Asset/Liability account; Transfer allows only Asset/Liability accounts; Investment needs an `Assets:Investments:*` account and another Asset/Liability account. Opening, Deposit, Withdrawal and Other are not checked. `TransactionService.GetTransactionRule` gives the allowed source/destination account types for the interactive wizard.

Backfill migrations for rows created before the column existed:

- `migrations/0006_backfill_transaction_type.up.sql` — SQL port of an older classifier (Opening, Income, Expense, Transfer, Other). It has no Deposit/Withdrawal/Investment rules and does not implement rule 4, so old rows can differ from what `DetermineType` would return today.
- `migrations/0009_backfill_investment_type.up.sql` and `migrations/0010_backfill_investment_type_expense.up.sql` — re-label rows with the Investment shape (0010 catches buys with fee splits that 0006 had stamped Expense).

Code lives in: `internal/model/types.go` (`TransactionType`, `ParseTransactionType`, `IsInvestmentAccount`, `InvestmentAccountPrefix`), `internal/service/transaction_classifier.go`, `internal/service/transaction_service.go` (`GetTransactionRule`).

## Regular attribute

`Regular` marks an Income or Expense transaction as habitual (rent, salary, groceries) versus one-off (a gift, a vacation). It is a label for filtering and report subtotals, not a schedule.

- Applies **only** to `Income` and `Expense`. For those it is `true` or `false`; for every other type it is absent — `nil` in Go (`*bool`), `NULL` in SQLite, omitted in JSON.
- New Income/Expense transactions default to `true` when the caller leaves it unset (`CreateTransaction`, `CreateSimpleTransaction`). On update, an unset value defaults to `true` for Income/Expense and is forced to `nil` for other types (`UpdateTransactionComplete`).
- `ValidateRegular` (`internal/service/transaction_validation.go`) enforces the invariant and returns `ErrRegularRequired` (Income/Expense with `nil`) or `ErrRegularNotApplicable` (other type with a value).
- The database repeats the rule as a `CHECK` on `transactions` in `migrations/0011_add_transaction_regular.up.sql`; that migration backfilled existing Income/Expense rows with `1` and everything else with `NULL`.

Code lives in: `internal/model/transaction.go` (`Transaction.Regular`), `internal/service/transaction_validation.go` (`ValidateRegular`), `internal/service/transaction_ops.go`, `internal/service/errors.go`.

## Opening balances

- An opening balance is an `Opening` transaction against a per-currency system equity account named `Equity:OpeningBalances_<CCY>`. Build the name with `model.OpeningBalancesAccountName` (uppercases the code) and detect one with `model.IsOpeningBalancesAccount` (prefix match).
- `AccountService.CreateAccountWithBalance` creates the account and its opening transaction in one `ExecTx`. Only Asset and Liability accounts may have an opening balance. The transaction is `Cleared`, its description and both split memos are `model.OpeningAccountMemo` (`Opening Balance`), and the equity account for the account's currency is created on demand (`createOpeningBalanceInRepo`).
- Split direction (amount entered as a positive number):

| New account | Account split | Equity split |
| --- | --- | --- |
| Asset | `+amount` | `-amount` |
| Liability | `-amount` | `+amount` |

- At startup `initSysAcc` (`cmd/root.go`) first runs `migrateLegacySysAcc`, which renames the legacy single account `Equity:OpeningBalances` (`model.LegacyOpeningBalancesName`) to the per-currency name for `config.Defaults.Currency`. If both names exist it deletes the legacy one, failing if it still has transactions. Then it creates the default-currency system account if missing.
- System accounts cannot be deleted, renamed, or have their metadata edited: `DeleteAccountByName`, `RenameAccount` and `UpdateAccountMetadata` return `ErrNotEditable`.

Code lives in: `internal/model/types.go`, `internal/service/account_ops.go` (`CreateAccountWithBalance`, `createOpeningBalanceInRepo`), `cmd/root.go` (`initSysAcc`, `migrateLegacySysAcc`, `migrateLegacySysAccWith`).

## Reconciliation

Reconciliation matches one account's splits against an external statement (bank, card).

```
 split with reconciled = 0   reconcile(account, statementBalance, txIDs)
 (this account)         ───────────────────────────────────────────────►  splits.reconciled = 1 (this account only)
                                                                          transactions.status = Reconciled
                                                                          last_reconciled_balance += selected amounts
```

- **Per-split flag.** `splits.reconciled` (`migrations/0003_add_split_reconciled.up.sql`) tracks reconciliation per account, so a transfer between two banks can be reconciled separately on each side. The candidate list is "splits on this account with `reconciled = 0`" (`Store.GetUnreconciledTransactionsByAccount`), not "transactions with status != Reconciled".
- **Transaction status.** `Store.MarkSplitsReconciledByAccount` sets the whole transaction to `StatusReconciled` as soon as any one of its splits is reconciled (expense/revenue sides are never reconciled, so waiting for all splits would never finish). The transaction then becomes immutable even if another account has not reconciled its side yet.
- **Last reconciled balance.** `account_reconcile_state.last_reconciled_balance` (`migrations/0004_add_account_reconcile_state.up.sql`) stores a running total per account (0 if never reconciled). Each reconcile adds the selected split amounts and persists the new total, whether or not it matches the statement.
- **Difference.** `statementBalance - (lastReconciledBalance + sum of selected amounts)`. `TransactionService.PreviewReconcile` computes it without writing; `TransactionService.ReconcileTransactions` validates the IDs (non-empty, no duplicates, all in the unreconciled set), marks splits, updates the balance atomically in `ExecTx`, and returns the difference.
- **Balance-mismatch gate.** The service always commits. The gate lives in the callers: non-interactive `kea reconcile --balance --ids` previews first and refuses a non-zero difference unless `--force` (`cmd/reconcile_actions.go`); the HTTP commit handler does the same unless `allow_mismatch` is true (`internal/api/reconcile.go`, `balanceMismatchError`). The interactive TUI (`ui/reconcile/model.go`, `Model.Update`) asks for a y/n confirmation when the difference is non-zero, and `cmd/reconcile_actions.go` calls `ReconcileTransactions` only after the user confirms.
- **Immutability.** A Reconciled transaction cannot be updated, have its status changed, or be deleted: `UpdateTransactionComplete`, `UpdateTransactionStatus` and `DeleteTransaction` return an error wrapping `ErrReconciled`. `TransactionService.IsEditable` exposes the same check to UIs. There is no un-reconcile operation.
- **What the CLI user sees.**
  - `kea transaction delete` surfaces the service error (`failed to delete transaction: ...`); `cmd.Execute` prints it with `pterm.Error` and exits 1.
  - `kea transaction edit` checks `IsEditable` up front, prints a `pterm.Warning` ("cannot be edited (Reconciled Transaction)") and exits 0 without entering the edit menu.

Code lives in: `internal/service/reconcile_ops.go`, `internal/store/sqlite_reconcile.go`, `internal/model/transaction.go` (`ReconcileEntry`), `ui/reconcile/model.go`.

## Reports

All reports are methods on `TransactionService` and are built from stored splits and the stored transaction type.

- **Balance sheet** (`GenerateBalanceSheet`, `model.BalanceSheetResult`) — balance of every Asset, Liability and Equity account as of a timestamp. Liabilities are shown sign-flipped (positive = owed). Net worth = assets - liabilities.
- **Income statement** (`GenerateIncomeStatement`, `GenerateFullIncomeStatement`, `model.ReportResult`) — for a period, Revenue splits of `Income` transactions and Expense splits of `Expense` transactions, as absolute amounts, grouped by account + offset account + currency. Net = income - expense. The "full" variant adds net worth at period end, at the end of the previous equal-length period, and growth percentage (`GetNetWorthAt`).
- **Income / expense breakdown** (`GenerateIncomeBreakdown`, `GenerateExpenseBreakdown`, and their `Full` variants) — the same rows for one side only, sorted by amount descending.
- **Regular/irregular subtotals** — `TotalIncomeRegular`, `TotalIncomeIrregular`, `TotalExpenseRegular`, `TotalExpenseIrregular` split each total by the transaction's `Regular` flag (`buildReportMaps`).
- **Classification by type, not by account.** Only transactions typed `Income`/`Expense` feed the income statement; an Expense split inside a `Transfer` or `Investment` transaction (e.g. a fee) is not counted.
- **Net worth series** (`GetDailyNetWorthSeries`, `model.CurrencyDailySeries`) — daily Asset + Liability totals per currency from the first transaction day to today (UTC), front-filled over quiet days; all-zero series are dropped.
- **Monthly balance history** (`GetMonthlyBalanceHistory`, `model.AccountMonthlySeries`) — end-of-month balance per Asset/Liability account, liabilities sign-flipped.
- **Date ranges** — `ResolveDateRange` takes `DateRangeParams`: `Month` (`YYYY-MM`) wins, else `From`/`To` (`YYYY-MM-DD`, inclusive, local time), else the current month.
- **Currencies are never summed together.** Every total is a `map[string]int64` keyed by currency code, and rows carry their own `Currency`. There is no exchange-rate conversion anywhere; a ledger with USD and TWD gets two totals.

Code lives in: `internal/service/report_service.go`, `internal/service/transaction_service.go` (`GetMonthlyBalanceHistory`), `internal/model/report.go`, `internal/model/net_worth_series.go`, `internal/model/balance_history.go`.

## Budgets

A budget is a monthly spending limit on an `Expense` (`E`) account, leaf or parent. Budgets live in the `budgets` table (`migrations/0012_create_budgets.up.sql`); each row is a version of one account's budget.

- **Versioning.** A row applies from its `effective_month` (`YYYY-MM`, local time) until a later row for the same account. Setting a budget for an (account, month) that already has a row replaces it (upsert), so a month has at most one version per account.
- **Stopping.** `StopBudget` writes a row with `stopped=1` and `amount=0`; it ends the budget from that month on. It is rejected when the account has no active budget in that month. If a later version exists, that version re-activates the budget from its own month.
- **Zero is a budget.** `amount=0` with `stopped=0` is a real budget of zero; only `stopped=1` ends one.
- **Currency** is the account's currency (empty means `config.Defaults.Currency`). Splits of descendant accounts in another currency are excluded from the row and their account names are listed in `ExcludedAccounts`.
- **Actual** is the signed sum of `Expense` splits of `Expense`-typed transactions on the account and its descendants (`isSelfOrDescendant`: exact name or `name + ":"` prefix, so `Expenses:FoodTruck` is not under `Expenses:Food`). Refunds reduce it and it may be negative. It is split into `ActualRegular` and `ActualIrregular` by the transaction's `Regular` flag. This differs from the expense report, which sums absolute values.
- **Totals** are per currency and only count budgeted rows with no budgeted ancestor in the same currency (`hasBudgetedAncestorInCurrency`), so a parent and its same-currency budgeted child are not double counted, while a child in another currency still counts in its own currency.
- **No rollover.** Each month is compared on its own; unspent budget does not carry over.

Code lives in: `internal/service/budget_service.go` (`SetBudget`, `StopBudget`, `activeBudgets`), `internal/service/budget_report.go` (`GenerateBudgetReport`), `internal/model/budget.go`, `internal/store/sqlite_budget.go`. Endpoints are in [http-api.md](http-api.md); the CLI is `kea budget` (`cmd/budget/`, `ui/views/budget.go`).

## Errors you will meet

Service errors wrap sentinels with `%w`; always test with `errors.Is` (or `errors.As` for `*ValidationError`), never by string or `==`.

| Sentinel | Meaning | Where returned |
| --- | --- | --- |
| `service.ErrReconciled` | transaction is reconciled and immutable | `DeleteTransaction`, `UpdateTransactionStatus`, `UpdateTransactionComplete` |
| `service.ErrNotEditable` | protected record (system account) | `DeleteAccountByName`, `RenameAccount`, `UpdateAccountMetadata` |
| `service.ErrNotFound` | account or transaction does not exist | account lookups (`GetAccountByName`, `GetAccountByID`, `GetAccountBalance`, `DeleteAccountByName`), transaction lookups/updates/deletes, reconcile account check |
| `service.ErrAlreadyExists` | unique-key collision (account name, transaction `external_id`) | `CreateAccount`, `CreateAccountWithBalance`, `CreateTransaction` |
| `service.ErrCircularParent` | parent chain loops or exceeds 100 levels | `validateParentChain` via `validateAccountFields` (`CreateAccount`, `CreateAccountWithBalance`) |
| `service.ErrRegularRequired` | Income/Expense with no `Regular` value | `ValidateRegular` (normally pre-empted by the `true` default) |
| `service.ErrRegularNotApplicable` | `Regular` set on a non-Income/Expense type | `ValidateRegular` via `CreateTransaction` |
| `*service.ValidationError` | bad user input; carries `Field` and `Message` | all validators (`validationErrorf`, `validationWrap`) |
| `repository.ErrNotFound` | storage-level not found (wrapped by `store.ErrRecordNotFound`) | store methods; services translate it to `service.ErrNotFound` at most call sites |
| `repository.ErrAlreadyExists` | storage-level unique violation (wrapped by `store.ErrAccountExists`, `store.ErrTransactionExists`) | store inserts; services translate it to `service.ErrAlreadyExists` |

Callers above the service layer should match `service.*` sentinels; `repository.*` sentinels are for the service/store boundary. The HTTP status for each error is defined in `internal/api/errors.go` (`mapError`); see the table in [http-api.md](http-api.md#errors).

Code lives in: `internal/service/errors.go`, `internal/repository/errors.go`, `internal/store/errors.go`.
