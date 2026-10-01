# Use AccountType Constants in Transaction Classifier — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all raw string literals (`"E"`, `"R"`, `"A"`, `"L"`, `"C"`) in `transaction_classifier.go` with typed `model.AccountType*` constants, and tighten `filterAccountsByTypes` to use `[]model.AccountType` instead of `[]string`.

**Architecture:** Pure refactoring — no behavior changes. The `DetermineType` switch and `GetAllowedAccounts` callers use raw strings where `model.AccountType` constants exist. `filterAccountsByTypes` accepts `[]string` but should accept `[]model.AccountType` since all callers pass account type values. All existing tests must continue to pass unchanged.

**Tech Stack:** Go, existing `model.AccountType` constants from `internal/model/types.go`

---

### Task 1: Replace raw strings in `DetermineType` switch

**Files:**
- Modify: `internal/service/transaction_classifier.go:46-66`

- [ ] **Step 1: Run existing tests to confirm green baseline**

Run: `go test ./internal/service/ -v -run TestDetermineType`
Expected: All tests PASS

- [ ] **Step 2: Replace raw string cases in the `DetermineType` switch**

In `internal/service/transaction_classifier.go`, replace lines 46–66:

```go
		switch accType {
		case model.AccountTypeExpense:
			hasExpense = true
			totalExpenseAmount += utils.AbsInt64(split.Amount)
		case model.AccountTypeRevenue:
			hasRevenue = true
			totalRevenueAmount += utils.AbsInt64(split.Amount)
		case model.AccountTypeAsset:
			assetOrLiabCnt++
			if split.Amount > 0 {
				isAssetIncrease = true
				totalPositiveAssetLiabAmount += split.Amount
			}
		case model.AccountTypeLiability:
			assetOrLiabCnt++
			if split.Amount > 0 {
				totalPositiveAssetLiabAmount += split.Amount
			}
		case model.AccountTypeEquity:
			hasEquity = true
		}
```

- [ ] **Step 3: Run tests to verify nothing broke**

Run: `go test ./internal/service/ -v -run TestDetermineType`
Expected: All tests PASS (identical results to Step 1)

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_classifier.go
git commit -m "refactor: use AccountType constants in DetermineType switch (closes #66, part 1)"
```

---

### Task 2: Replace raw strings in `GetAllowedAccounts`

**Files:**
- Modify: `internal/service/transaction_classifier.go:257-277`

- [ ] **Step 1: Replace raw string comparisons in `GetAllowedAccounts`**

In `internal/service/transaction_classifier.go`, replace lines 260–272:

```go
	switch txType {
	case model.TxTypeExpense:
		if currentAccountType == model.AccountTypeExpense {
			return ts.filterAccountsByTypes(allAccounts, []string{"E"})
		}
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	case model.TxTypeIncome:
		if currentAccountType == model.AccountTypeRevenue {
			return ts.filterAccountsByTypes(allAccounts, []string{"R"})
		}
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	case model.TxTypeTransfer:
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	default:
		return allAccounts
	}
```

Note: The `filterAccountsByTypes` calls still use `[]string` — that gets fixed in Task 3.

- [ ] **Step 2: Run tests to verify nothing broke**

Run: `go test ./internal/service/ -v -run TestGetAllowedAccounts`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/service/transaction_classifier.go
git commit -m "refactor: use AccountType constants in GetAllowedAccounts (closes #66, part 2)"
```

---

### Task 3: Change `filterAccountsByTypes` to use `[]model.AccountType`

**Files:**
- Modify: `internal/service/transaction_classifier.go:257-277` (callers)
- Modify: `internal/service/transaction_classifier.go:355-369` (function signature + body)

- [ ] **Step 1: Update `filterAccountsByTypes` signature and body**

Replace the function at lines 355–369:

```go
func (ts *TransactionService) filterAccountsByTypes(accounts []*model.Account, allowedTypes []model.AccountType) []*model.Account {
	var filtered []*model.Account

	typeMap := make(map[model.AccountType]bool)
	for _, t := range allowedTypes {
		typeMap[t] = true
	}

	for _, acc := range accounts {
		if typeMap[acc.Type] {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}
```

- [ ] **Step 2: Update all `filterAccountsByTypes` callers in `GetAllowedAccounts`**

Replace the `filterAccountsByTypes` calls in `GetAllowedAccounts` (lines 261–272):

```go
	switch txType {
	case model.TxTypeExpense:
		if currentAccountType == model.AccountTypeExpense {
			return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeExpense})
		}
		return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability})

	case model.TxTypeIncome:
		if currentAccountType == model.AccountTypeRevenue {
			return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeRevenue})
		}
		return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability})

	case model.TxTypeTransfer:
		return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability})

	default:
		return allAccounts
	}
```

- [ ] **Step 3: Build to verify type correctness**

Run: `go build ./...`
Expected: No compile errors

- [ ] **Step 4: Run all tests**

Run: `go test ./internal/service/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_classifier.go
git commit -m "refactor: type filterAccountsByTypes with []model.AccountType (closes #66, part 3)"
```

---

### Task 4: Final verification

- [ ] **Step 1: Grep for any remaining raw account-type strings**

Run: `grep -n '"[EARCLC]"' internal/service/transaction_classifier.go`
Expected: No output (all raw strings replaced)

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`
Expected: All tests PASS

- [ ] **Step 3: Commit (if any stragglers found and fixed)**

Only if Step 1 found remaining raw strings. Otherwise skip.
