-- Reverse: same best-effort reverse-classifier shape as 0009.down.sql.
-- Investment rows containing a Revenue split → Income;
-- Investment rows containing an Expense split → Expense;
-- remaining Investment rows → Transfer.
-- (Lossy by design; perfect round-trip is not required.)
UPDATE transactions SET type = 'Income'
WHERE type = 'Investment'
  AND id IN (
    SELECT s.transaction_id
    FROM splits s
    JOIN accounts a ON s.account_id = a.id
    WHERE a.type = 'R'
  );

UPDATE transactions SET type = 'Expense'
WHERE type = 'Investment'
  AND id IN (
    SELECT s.transaction_id
    FROM splits s
    JOIN accounts a ON s.account_id = a.id
    WHERE a.type = 'E'
  );

UPDATE transactions SET type = 'Transfer'
WHERE type = 'Investment';
