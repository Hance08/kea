package views

import (
	"bytes"
	"os"
	"strings"
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

	assert.True(t, strings.Contains(output, "-50"),
		"negative asset balance should show minus sign; got:\n%s", output)
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

	assert.True(t, strings.Contains(output, "-20"),
		"negative expense balance should show minus sign; got:\n%s", output)
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

	assert.True(t, strings.Contains(output, "100"),
		"liability balance should strip minus sign; got:\n%s", output)
	assert.False(t, strings.Contains(output, "-100"),
		"liability balance must not show minus sign; got:\n%s", output)
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

	assert.True(t, strings.Contains(output, "200"),
		"positive asset balance should be unchanged; got:\n%s", output)
}
