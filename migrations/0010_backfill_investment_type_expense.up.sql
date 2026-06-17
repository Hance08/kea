-- 0009 missed transactions that the legacy 0006 SQL backfill stamped as
-- 'Expense' (e.g., a buy with a non-zero fee split). The Go classifier
-- would have returned 'Transfer' for the same shape, but the SQL backfill
-- in 0006 short-circuits to 'Expense' as soon as it sees an Expense split
-- plus any A/L split. This catches those rows.
--
-- Re-classify ANY row that is not already 'Investment' or 'Opening' and
-- whose splits match the Investment shape (≥1 Assets:Investments:* split
-- AND ≥1 other A/L split).
UPDATE transactions SET type = 'Investment'
WHERE type NOT IN ('Investment', 'Opening')
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
