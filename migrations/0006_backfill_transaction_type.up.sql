UPDATE transactions SET type = (
  SELECT
    CASE
      WHEN memo_check.has_opening = 1 THEN 'Opening'
      WHEN agg.has_exp = 1 AND agg.has_rev = 1
        THEN CASE WHEN agg.rev_sum >= agg.exp_sum THEN 'Income' ELSE 'Expense' END
      WHEN agg.has_exp = 1 AND agg.al_cnt >= 1 THEN 'Expense'
      WHEN agg.has_rev = 1 AND agg.al_cnt >= 1 THEN 'Income'
      WHEN agg.al_cnt >= 2                       THEN 'Transfer'
      ELSE 'Other'
    END
  FROM (
    SELECT
      MAX(CASE WHEN a.type = 'E' THEN 1 ELSE 0 END)             AS has_exp,
      MAX(CASE WHEN a.type = 'R' THEN 1 ELSE 0 END)             AS has_rev,
      SUM(CASE WHEN a.type IN ('A','L') THEN 1 ELSE 0 END)      AS al_cnt,
      SUM(CASE WHEN a.type = 'E' THEN ABS(s.amount) ELSE 0 END) AS exp_sum,
      SUM(CASE WHEN a.type = 'R' THEN ABS(s.amount) ELSE 0 END) AS rev_sum
    FROM splits s
    JOIN accounts a ON s.account_id = a.id
    WHERE s.transaction_id = transactions.id
  ) agg,
  (
    SELECT MAX(CASE WHEN s.memo = 'Opening Balance' THEN 1 ELSE 0 END) AS has_opening
    FROM splits s
    WHERE s.transaction_id = transactions.id
  ) memo_check
);
