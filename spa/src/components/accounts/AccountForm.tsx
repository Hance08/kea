import { OpeningBalanceField } from '@/components/accounts/OpeningBalanceField';
import { ParentAccountPicker } from '@/components/accounts/ParentAccountPicker';
import { TypeSelect } from '@/components/accounts/TypeSelect';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useServerConfig } from '@/lib/server-config';
import type { Account, AccountType, CreateAccountInput, UpdateAccountInput } from '@/lib/types';
import { useState } from 'react';

interface CreateProps {
  mode: 'create';
  onSubmit: (input: CreateAccountInput) => void;
  pending?: boolean;
  formError?: string;
  fieldErrors?: Record<string, string | undefined>;
}

interface EditProps {
  mode: 'edit';
  account: Account;
  parentName?: string;
  systemReadOnlyName?: boolean;
  onSubmit: (input: UpdateAccountInput) => void;
  pending?: boolean;
  formError?: string;
  fieldErrors?: Record<string, string | undefined>;
}

type Props = CreateProps | EditProps;

// parseAmount: convert a user-entered decimal string to integer cents.
// Returns undefined when the input is empty or not a finite number, so callers can
// distinguish "no opening balance" from "0 cents". Matches the conventions used in
// TransactionForm / SplitsEditor (Math.round(n * 100)).
function parseAmount(s: string): number | undefined {
  const trimmed = s.trim();
  if (trimmed === '') return undefined;
  const n = Number(trimmed);
  if (!Number.isFinite(n)) return undefined;
  return Math.round(n * 100);
}

export function AccountForm(props: Props) {
  return props.mode === 'create' ? <CreateForm {...props} /> : <EditForm {...props} />;
}

function CreateForm(props: CreateProps) {
  const { defaults } = useServerConfig();
  const [parentName, setParentName] = useState('');
  const [parentAcc, setParentAcc] = useState<Account | undefined>(undefined);
  const [type, setType] = useState<AccountType | ''>('');
  const [name, setName] = useState('');
  const [currency, setCurrency] = useState(defaults.currency);
  const [description, setDescription] = useState('');
  const [balanceEnabled, setBalanceEnabled] = useState(false);
  const [balanceText, setBalanceText] = useState('');

  const effectiveType: AccountType | '' = parentAcc?.type ?? type;
  const effectiveCurrency = parentAcc?.currency ?? currency;

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!effectiveType) return;
    if (!name) return;
    const fullName = parentName ? `${parentName}:${name}` : name;
    const balanceCents = balanceEnabled ? parseAmount(balanceText) : undefined;
    props.onSubmit({
      name: fullName,
      type: effectiveType,
      parent_id: parentAcc?.id,
      currency: effectiveCurrency,
      description: description || undefined,
      balance: balanceCents,
    });
  };

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-md border bg-card p-4">
      <h1 className="text-xl font-semibold">New account</h1>

      {props.formError && (
        <Alert variant="destructive">
          <AlertDescription>{props.formError}</AlertDescription>
        </Alert>
      )}

      <div className="space-y-1">
        <Label>Parent account (optional)</Label>
        <ParentAccountPicker
          value={parentName}
          onChange={(n, acc) => {
            setParentName(n);
            setParentAcc(acc);
          }}
        />
        {props.fieldErrors?.parent_id && (
          <p className="text-xs text-destructive">{props.fieldErrors.parent_id}</p>
        )}
      </div>

      <div className="space-y-1">
        <Label>Type</Label>
        <TypeSelect value={effectiveType} onChange={setType} disabled={Boolean(parentAcc)} />
        {parentAcc && (
          <p className="text-xs text-muted-foreground">Inherited from parent ({parentAcc.type}).</p>
        )}
        {props.fieldErrors?.type && (
          <p className="text-xs text-destructive">{props.fieldErrors.type}</p>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label htmlFor="acc-name">Name</Label>
          <Input
            id="acc-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={parentAcc ? 'Checking' : 'Assets'}
            aria-invalid={props.fieldErrors?.name ? true : undefined}
          />
          {name.includes(':') && (
            <p className="text-xs text-destructive">
              Name should not contain ':'. Use the Parent picker to nest.
            </p>
          )}
          {props.fieldErrors?.name && (
            <p className="text-xs text-destructive">{props.fieldErrors.name}</p>
          )}
        </div>
        <div className="space-y-1">
          <Label htmlFor="acc-currency">Currency</Label>
          <Input
            id="acc-currency"
            value={effectiveCurrency}
            onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            disabled={Boolean(parentAcc)}
          />
          {parentAcc && <p className="text-xs text-muted-foreground">Inherited from parent.</p>}
          {props.fieldErrors?.currency && (
            <p className="text-xs text-destructive">{props.fieldErrors.currency}</p>
          )}
        </div>
      </div>

      <div className="space-y-1">
        <Label htmlFor="acc-desc">Description</Label>
        <Input id="acc-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
        {props.fieldErrors?.description && (
          <p className="text-xs text-destructive">{props.fieldErrors.description}</p>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Hidden is set after creation — use the edit form to hide this account from default views.
      </p>

      <OpeningBalanceField
        enabled={balanceEnabled}
        onEnabledChange={setBalanceEnabled}
        amount={balanceText}
        onAmountChange={setBalanceText}
        currency={effectiveCurrency}
        error={props.fieldErrors?.balance}
      />

      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={() => window.history.back()}>
          Cancel
        </Button>
        <Button
          type="submit"
          disabled={props.pending || !effectiveType || !name || name.includes(':')}
        >
          {props.pending ? 'Creating…' : 'Create'}
        </Button>
      </div>
    </form>
  );
}

function EditForm(props: EditProps) {
  const [name, setName] = useState(props.account.name.split(':').pop() ?? props.account.name);
  const [description, setDescription] = useState(props.account.description);
  const [isHidden, setIsHidden] = useState(props.account.is_hidden);

  const TYPE_LABEL: Record<AccountType, string> = {
    A: 'Asset',
    L: 'Liability',
    C: 'Equity',
    R: 'Revenue',
    E: 'Expense',
  };

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const patch: UpdateAccountInput = {};
    const segments = props.account.name.split(':');
    segments[segments.length - 1] = name;
    const newFullName = segments.join(':');
    if (newFullName !== props.account.name) patch.name = newFullName;
    if (description !== props.account.description) patch.description = description;
    if (isHidden !== props.account.is_hidden) patch.is_hidden = isHidden;
    props.onSubmit(patch);
  };

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-md border bg-card p-4">
      <h1 className="text-xl font-semibold">Edit account</h1>

      {props.formError && (
        <Alert variant="destructive">
          <AlertDescription>{props.formError}</AlertDescription>
        </Alert>
      )}

      <dl className="grid grid-cols-[8rem_1fr] gap-1 text-sm">
        <dt className="text-muted-foreground">Type</dt>
        <dd>
          {TYPE_LABEL[props.account.type]}{' '}
          <span className="text-xs text-muted-foreground">(cannot change)</span>
        </dd>
        <dt className="text-muted-foreground">Parent</dt>
        <dd>
          {props.parentName ?? '(top-level)'}{' '}
          <span className="text-xs text-muted-foreground">(cannot change)</span>
        </dd>
        <dt className="text-muted-foreground">Currency</dt>
        <dd>
          {props.account.currency}{' '}
          <span className="text-xs text-muted-foreground">(cannot change)</span>
        </dd>
      </dl>
      <p className="text-xs text-muted-foreground">
        Type, parent, and currency cannot be changed. Create a new account and move transactions
        instead.
      </p>

      <div className="space-y-1">
        <Label htmlFor="acc-name">Name</Label>
        <Input
          id="acc-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={props.systemReadOnlyName}
          aria-invalid={props.fieldErrors?.name ? true : undefined}
        />
        {props.fieldErrors?.name && (
          <p className="text-xs text-destructive">{props.fieldErrors.name}</p>
        )}
      </div>

      <div className="space-y-1">
        <Label htmlFor="acc-desc">Description</Label>
        <Input id="acc-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
      </div>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={isHidden} onChange={(e) => setIsHidden(e.target.checked)} />
        Hidden
      </label>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={() => window.history.back()}>
          Cancel
        </Button>
        <Button type="submit" disabled={props.pending}>
          {props.pending ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </form>
  );
}
