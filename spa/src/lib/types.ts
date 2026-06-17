export type AccountType = 'A' | 'L' | 'C' | 'R' | 'E';

export interface AccountBalance {
  account_id: number;
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  amount: number; // int64 cents
  is_hidden: boolean;
}

export interface ListResult<T> {
  items: T[];
  total_count: number;
  limit: number;
  offset: number;
}

export interface ServerConfig {
  defaults: {
    currency: string;
  };
  display: {
    hide_decimals: boolean;
  };
}

export interface LedgerInfo {
  name: string;
  path: string;
  active: boolean;
}

export interface LedgerListResponse {
  active: string;
  items: LedgerInfo[];
}

export type TransactionType =
  | 'Expense'
  | 'Income'
  | 'Transfer'
  | 'Opening'
  | 'Deposit'
  | 'Withdrawal'
  | 'Other'
  | 'Investment';

export type TransactionStatus = 'Pending' | 'Cleared' | 'Reconciled';

export interface SplitDetail {
  id: number;
  account_id: number;
  account_name: string;
  account_type: AccountType;
  amount: number; // int64 cents, signed
  currency: string;
  memo: string;
}

export interface TransactionDetail {
  id: number;
  timestamp: number; // Unix seconds
  description: string;
  status: TransactionStatus;
  type: TransactionType;
  splits: SplitDetail[];
}

export interface SplitInput {
  id?: number;
  account_name: string;
  amount: number;
  currency: string;
  memo?: string;
}

export interface CreateTransactionInput {
  splits: SplitInput[];
  description: string;
  timestamp: number;
  status: TransactionStatus;
  type?: TransactionType;
}

export interface UpdateTransactionInput {
  id: number;
  description: string;
  timestamp: number;
  status: TransactionStatus;
  type?: TransactionType;
  splits: SplitInput[];
}

export interface TransactionFilter {
  account_id?: number;
  type?: TransactionType;
  status?: TransactionStatus;
  start_time?: number;
  end_time?: number;
  description?: string;
}

export interface Account {
  id: number;
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  description: string;
  is_hidden: boolean;
}

export interface AccountNode {
  account: Account;
  children: AccountNode[];
}

export interface CreateAccountInput {
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  description?: string;
  balance?: number; // optional opening balance, in cents
}

export interface UpdateAccountInput {
  name?: string;
  description?: string;
  is_hidden?: boolean;
}

export interface BalanceResponse {
  account_id: number;
  amount: number;
  currency: string;
}

export interface BalanceHistoryPoint {
  month: string; // "YYYY-MM"
  balance: number; // cents, natural-amount
}

export interface AccountBalanceHistory {
  account_id: number;
  currency: string;
  points: BalanceHistoryPoint[];
}

export interface BalanceHistoryResponse {
  items: AccountBalanceHistory[];
}

// Report shapes (mirror Go JSON from internal/model/report.go
// and internal/api/reports.go).

export interface ReportRow {
  account_name: string;
  offset_account: string;
  amount: number;
  currency: string;
  tx_count: number;
}

export interface ReportResult {
  period: string;
  total_income: Record<string, number>;
  total_expense: Record<string, number>;
  net_amount: Record<string, number>;
  net_worth: Record<string, number>;
  previous_net_worth: Record<string, number>;
  net_worth_growth_pct: Record<string, number>;
  income_rows: ReportRow[];
  expense_rows: ReportRow[];
}

export interface BalanceSheetResult {
  assets: ReportRow[];
  liabilities: ReportRow[];
  equity: ReportRow[];
  total_assets: Record<string, number>;
  total_liabilities: Record<string, number>;
  total_equity: Record<string, number>;
  net_worth: Record<string, number>;
  as_of: number;
}

export interface NetWorthResponse {
  at: number;
  net_worth: Record<string, number>;
}
