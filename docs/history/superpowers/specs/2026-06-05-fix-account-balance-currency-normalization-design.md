# Fix: `handleAccountBalance` empty-currency normalization

**Status:** Design approved 2026-06-05.
**Triggered by:** Code-quality review of PR #182 (bulk balances). The new `GET /api/balances` endpoint normalizes empty `acc.Currency` to `config.Defaults.Currency`; the sibling `GET /api/accounts/{id}/balance` does not. This spec closes the drift.

## Problem

`internal/api/accounts.go::handleAccountBalance` builds its response with:

```go
return writeJSON(w, http.StatusOK, balanceResponse{
    AccountID: id,
    Amount:    amount,
    Currency:  acc.Currency,
})
```

`acc.Currency` can legitimately be `""` — `AccountService.ValidateCurrency` ([internal/service/account_validation.go:81](../../internal/service/account_validation.go)) explicitly returns `nil` for empty input with the comment `// empty is allowed, will use default`. An account created without specifying `Currency` then surfaces as `"currency": ""` from this endpoint, while every other balance-emitting code path normalizes to the config default:

- `internal/service/report_service.go:309-311` — `GenerateBalanceSheet`
- `internal/service/account_service.go:192-194` — `GetAccountBalancesBulk` (added in PR #182)
- `internal/service/account_ops.go:170-172` — `createOpeningBalanceInRepo`

A non-test audit (`grep "acc\.Currency\|account\.Currency"`) confirms `handleAccountBalance:72` is the **only** remaining site that reads `acc.Currency` without the normalization step.

## Decision

Normalize at the handler layer, mirroring how `handleListBalances` keeps inline composition logic (the `time.Now().Unix()` default) in the handler rather than the service.

Three options considered:

1. **Handler-level normalization** *(chosen)*. Four added lines in `handleAccountBalance`. Consistent with the immediate sibling endpoint.
2. Promote `AccountService.GetAccountBalance` to return `(int64, string, error)` (amount + resolved currency). Larger API surface change; every caller (`handleAccountBalance`, `GetAccountBalanceFormatted`, CLI paths) would need adjustment. Disproportionate for one drift site.
3. Refactor `handleAccountBalance` to call `GetAccountBalancesBulk` and filter to one ID. Wasteful — loads every account and every balance to return one row.

## Change

### `internal/api/accounts.go::handleAccountBalance`

Replace:

```go
return writeJSON(w, http.StatusOK, balanceResponse{
    AccountID: id,
    Amount:    amount,
    Currency:  acc.Currency,
})
```

with:

```go
currency := acc.Currency
if currency == "" {
    currency = s.svc.Config().Defaults.Currency
}
return writeJSON(w, http.StatusOK, balanceResponse{
    AccountID: id,
    Amount:    amount,
    Currency:  currency,
})
```

`s.svc.Config()` is the existing service-facade accessor (see [CLAUDE.md](../../CLAUDE.md) "Service facade").

### Test

New `TestHandleAccountBalance_EmptyCurrencyNormalized` in `internal/api/accounts_test.go`.

`seedAccount` ([internal/api/testhelper_test.go:30](../../internal/api/testhelper_test.go)) hardcodes `Currency: "USD"`, so it cannot be used to produce an empty-currency account. The test calls `svc.Account().CreateAccount(...)` directly with `Currency: ""` (no balance needed → `CreateAccount`, not `CreateAccountWithBalance`).

Shape:

```go
func TestHandleAccountBalance_EmptyCurrencyNormalized(t *testing.T) {
    ts, svc := newServerWithStore(t)
    acc, err := svc.Account().CreateAccount(t.Context(), model.CreateAccountInput{
        Name:     "Assets:NoCurrency",
        Type:     model.AccountTypeAsset,
        Currency: "",
    })
    if err != nil {
        t.Fatalf("create account: %v", err)
    }

    resp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
    if err != nil {
        t.Fatalf("GET: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("status: got %d, want 200", resp.StatusCode)
    }
    var got struct {
        AccountID int64  `json:"account_id"`
        Amount    int64  `json:"amount"`
        Currency  string `json:"currency"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
        t.Fatalf("decode: %v", err)
    }
    want := svc.Config().Defaults.Currency
    if got.Currency != want {
        t.Errorf("currency: got %q, want %q (config default)", got.Currency, want)
    }
}
```

Asserting against `svc.Config().Defaults.Currency` (not a hardcoded `"USD"`) pins "the normalization fires" without coupling the test to whatever string the default happens to be.

## Out of scope

- Extending `seedAccount` with a currency parameter — one test needs the empty case; a direct `CreateAccount` call is cleaner than helper proliferation.
- A spec audit of future endpoints — this PR sets the pattern; future drift gets caught in review.
- Changing `balanceResponse`'s shape or moving normalization into the service layer (Options 2 and 3 above).

## Verification

- `go test ./internal/api/... -run TestHandleAccountBalance` — green; the new test plus existing `TestHandleAccountBalance_*` cases all pass.
- `go test ./...` — green.
- `go build ./...` — clean.

## Commit shape

Single commit, scope `(api)`:

```
fix(api): normalize empty currency in /api/accounts/{id}/balance response

handleAccountBalance returned acc.Currency directly, so an account created
without a Currency (legal per ValidateCurrency) surfaced as "currency": "".
The bulk /api/balances endpoint and GenerateBalanceSheet already normalize
empty to config.Defaults.Currency; this commit applies the same pattern at
the handler layer, mirroring how handleListBalances composes the as_of
default inline.
```
