UPDATE transactions SET type = 'Investment'
WHERE type IN ('Income', 'Transfer', 'Other')
  AND id IN (
    SELECT s_inv.transaction_id
    FROM splits s_inv
    JOIN accounts a_inv ON s_inv.account_id = a_inv.id
    WHERE a_inv.name LIKE 'Assets:Investments:%'
    INTERSECT
    SELECT s_cash.transaction_id
    FROM splits s_cash
    JOIN accounts a_cash ON s_cash.account_id = a_cash.id
    WHERE a_cash.type IN ('A', 'L')
      AND a_cash.name NOT LIKE 'Assets:Investments:%'
  );
