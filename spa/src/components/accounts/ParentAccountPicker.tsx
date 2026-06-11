import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import type { Account, AccountType } from '@/lib/types';

interface Props {
  value: string;
  onChange: (name: string, account?: Account) => void;
  // When set, only accounts of this type may be picked as parent (child must match parent type).
  restrictToType?: AccountType;
}

export function ParentAccountPicker({ value, onChange, restrictToType }: Props) {
  return (
    <AccountCombobox
      value={value}
      onChange={onChange}
      placeholder="No parent (top-level)"
      allowedTypes={restrictToType ? [restrictToType] : undefined}
    />
  );
}
