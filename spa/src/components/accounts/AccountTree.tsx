import { AccountTreeNode } from '@/components/accounts/AccountTreeNode';
import type { AccountBalance, AccountNode } from '@/lib/types';
import { useState } from 'react';

interface Props {
  nodes: AccountNode[];
  balances?: AccountBalance[];
}

export function AccountTree({ nodes, balances }: Props) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const balanceById = new Map<number, { amount: number; currency: string }>();
  if (balances) {
    for (const b of balances) {
      balanceById.set(b.account_id, { amount: b.amount, currency: b.currency });
    }
  }

  const toggle = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="rounded-md border bg-card">
      {nodes.map((node) => (
        <Branch
          key={node.account.id}
          node={node}
          depth={0}
          expanded={expanded}
          onToggle={toggle}
          balanceById={balanceById}
        />
      ))}
    </div>
  );
}

interface BranchProps {
  node: AccountNode;
  depth: number;
  expanded: Set<number>;
  onToggle: (id: number) => void;
  balanceById: Map<number, { amount: number; currency: string }>;
}

function Branch({ node, depth, expanded, onToggle, balanceById }: BranchProps) {
  const hasChildren = node.children.length > 0;
  const isOpen = expanded.has(node.account.id);
  return (
    <>
      <AccountTreeNode
        account={node.account}
        depth={depth}
        hasChildren={hasChildren}
        expanded={isOpen}
        onToggle={() => onToggle(node.account.id)}
        balance={balanceById.get(node.account.id)}
      />
      {isOpen &&
        node.children.map((child) => (
          <Branch
            key={child.account.id}
            node={child}
            depth={depth + 1}
            expanded={expanded}
            onToggle={onToggle}
            balanceById={balanceById}
          />
        ))}
    </>
  );
}
