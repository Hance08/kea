package views

import (
	"bytes"
	"os"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRender redirects stdout, calls Render, and returns the output string.
func captureRender(t *testing.T, accounts []*model.Account, getter func(int64) (string, error)) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	v := NewAccountListView()
	err = v.Render(accounts, getter)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

func TestAccountListView_NegativeAssetBalancePreservesSign(t *testing.T) {
	acc := &model.Account{
		ID:       1,
		Name:     "Assets:Checking",
		Type:     model.AccountTypeAsset,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "-50", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.Contains(t, output, "-50")
}

func TestAccountListView_NegativeExpenseBalancePreservesSign(t *testing.T) {
	acc := &model.Account{
		ID:       2,
		Name:     "Expenses:Refund",
		Type:     model.AccountTypeExpense,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "-20", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.Contains(t, output, "-20")
}

func TestAccountListView_NegativeLiabilityBalanceStripsSign(t *testing.T) {
	acc := &model.Account{
		ID:       3,
		Name:     "Liabilities:CreditCard",
		Type:     model.AccountTypeLiability,
		Currency: "TWD",
	}
	// Liabilities are stored negative-signed; the view must strip the sign.
	getter := func(int64) (string, error) { return "-100", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.Contains(t, output, "100")
	assert.NotContains(t, output, "-100")
}

func TestAccountListView_PositiveAssetBalanceUnchanged(t *testing.T) {
	acc := &model.Account{
		ID:       4,
		Name:     "Assets:Bank",
		Type:     model.AccountTypeAsset,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "200", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.Contains(t, output, "200")
}

func TestAccountListView_NegativeRevenueBalanceStripsSign(t *testing.T) {
	acc := &model.Account{
		ID:       5,
		Name:     "Revenue:Sales",
		Type:     model.AccountTypeRevenue,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "-75", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.Contains(t, output, "75")
	assert.NotContains(t, output, "-75")
}

func TestAccountListView_NegativeEquityBalanceStripsSign(t *testing.T) {
	acc := &model.Account{
		ID:       6,
		Name:     "Equity:OpeningBalances",
		Type:     model.AccountTypeEquity,
		Currency: "TWD",
	}
	getter := func(int64) (string, error) { return "-300", nil }

	output := captureRender(t, []*model.Account{acc}, getter)

	assert.Contains(t, output, "300")
	assert.NotContains(t, output, "-300")
}
