-- Reverse: reclassify Investment rows back using the legacy classifier output.
-- Investment rows containing a Revenue split → Income; otherwise → Transfer.
-- (Lossy by design; perfect round-trip is not required.)
UPDATE transactions SET type = 'Income'
WHERE type = 'Investment'
  AND id IN (
    SELECT s.transaction_id
    FROM splits s
    JOIN accounts a ON s.account_id = a.id
    WHERE a.type = 'R'
  );

UPDATE transactions SET type = 'Transfer'
WHERE type = 'Investment';
