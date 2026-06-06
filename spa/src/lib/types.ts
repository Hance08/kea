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
