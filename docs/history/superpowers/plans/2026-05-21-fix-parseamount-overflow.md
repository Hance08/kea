# Fix ParseAmount Overflow (Issue #128) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `ParseAmount` from silently overflowing on large dollar values by adding overflow guards before the multiplication and addition on line 127.

**Architecture:** Add a sentinel error `ErrAmountOverflow` to the `utils` package. Before the `(dollars+dollarCarry)*CentsPerUnit + cents` computation, check that `dollars+dollarCarry` does not exceed `math.MaxInt64/100` (positive) or fall below `math.MinInt64/100` (negative). This catches overflow at the earliest point and returns a clear error. TDD: write failing tests first for boundary values.

**Tech Stack:** Go, `math` stdlib

---

### Task 1: Add overflow tests

**Files:**
- Modify: `internal/utils/amount_test.go`

- [ ] **Step 1: Add overflow test cases to `TestParseAmount_Invalid`**

Add these cases to the `tests` slice in `TestParseAmount_Invalid` (after the existing `{"just -", "-"}` entry):

```go
{"overflow positive dollars", "92233720368547759"},
{"overflow negative dollars", "-92233720368547759"},
{"overflow at exact boundary", "92233720368547758"},
{"overflow from dollarCarry", "92233720368547757.999"},
```

The first two overflow when multiplied by 100. The third is exactly `math.MaxInt64/100` — but `92233720368547758 * 100 = 9223372036854775800`, and adding even 0 cents is fine, however the value itself is right at the boundary. The fourth triggers a dollar carry from rounding `.999` → `+1`, pushing `92233720368547757 + 1 = 92233720368547758` which then overflows when multiplied by 100 only if cents push it over — actually `92233720368547758 * 100 = 9223372036854775800` which fits. Let me recalculate.

`math.MaxInt64 = 9223372036854775807`. `math.MaxInt64 / 100 = 92233720368547758` (integer division). `92233720368547758 * 100 = 9223372036854775800`. That fits. `92233720368547759 * 100 = 9223372036854775900` which overflows.

So the boundary is: dollars ≤ `92233720368547758` is safe (the product fits), dollars = `92233720368547759` overflows.

But we also need to check the addition of cents: `92233720368547758 * 100 + cents`. The product is `9223372036854775800`, and `math.MaxInt64 = 9223372036854775807`, so cents up to 7 are safe, cents ≥ 8 overflow.

Corrected test cases:

```go
{"overflow positive dollars", "92233720368547759"},
{"overflow negative dollars", "-92233720368547759"},
{"overflow from cents addition", "92233720368547758.08"},
{"overflow from dollarCarry", "92233720368547758.995"},
```

- [ ] **Step 2: Add a valid boundary test to `TestParseAmount_Valid`**

Add this case to the `tests` slice in `TestParseAmount_Valid`:

```go
{"max safe positive", "92233720368547758.07", 9223372036854775807},
{"max safe negative", "-92233720368547758.07", -9223372036854775807},
```

`92233720368547758 * 100 + 7 = 9223372036854775807 = math.MaxInt64`.

- [ ] **Step 3: Run tests to verify the new overflow cases fail (no guard exists yet)**

Run: `go test ./internal/utils/ -v -run "TestParseAmount_Invalid|TestParseAmount_Valid"`

Expected: The four new invalid cases PASS incorrectly (no error returned — they silently overflow). The two new valid cases may produce wrong values due to overflow. This confirms the bug exists.

- [ ] **Step 4: Commit**

```bash
git add internal/utils/amount_test.go
git commit -m "test: add failing tests for ParseAmount overflow (issue #128)"
```

---

### Task 2: Add overflow guards to `ParseAmount`

**Files:**
- Modify: `internal/utils/amount.go`

- [ ] **Step 1: Add the `ErrAmountOverflow` sentinel and a `maxDollars` constant**

Add these after the imports block (before `FormatAmount`):

```go
var ErrAmountOverflow = fmt.Errorf("amount too large: exceeds int64 range")

const maxDollars = math.MaxInt64 / int64(model.CentsPerUnit)
```

`maxDollars` = `92233720368547758`.

- [ ] **Step 2: Add overflow checks before the multiplication on line 127**

Replace this block in `ParseAmount`:

```go
	total := (dollars+dollarCarry)*int64(model.CentsPerUnit) + cents

	if isNegative {
		total = -total
	}

	return total, nil
```

With:

```go
	dollars += dollarCarry
	if dollars > maxDollars {
		return 0, ErrAmountOverflow
	}

	total := dollars*int64(model.CentsPerUnit) + cents
	if total < 0 {
		return 0, ErrAmountOverflow
	}

	if isNegative {
		total = -total
	}

	return total, nil
```

The logic:
1. First check: `dollars > maxDollars` catches the multiplication overflow (since `(maxDollars+1)*100` wraps).
2. Second check: `total < 0` catches the addition overflow — when dollars equals exactly `maxDollars`, the product is `9223372036854775800`, and adding cents > 7 wraps negative.
3. Negative amounts: since we strip the sign early and re-apply at the end, overflow detection only needs to work on positive values. The max negative result is `-9223372036854775807` which is safely above `math.MinInt64` (`-9223372036854775808`).

- [ ] **Step 3: Run all tests**

Run: `go test ./internal/utils/ -v`

Expected: All tests pass — both existing tests and the new overflow boundary tests from Task 1.

- [ ] **Step 4: Run full test suite to check for regressions**

Run: `go test ./...`

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/utils/amount.go
git commit -m "fix: guard ParseAmount against int64 overflow on large amounts (#128)"
```
