package prompts

import (
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
)

func makeAcc(id int64, name string, accType model.AccountType, currency string, parentID *int64, hidden bool) *model.Account {
	return &model.Account{ID: id, Name: name, Type: accType, Currency: currency, ParentID: parentID, IsHidden: hidden}
}

func int64Ptr(v int64) *int64 { return &v }

func TestFilterAccounts(t *testing.T) {
	asset := model.AccountTypeAsset
	expense := model.AccountTypeExpense

	t.Run("filters by type", func(t *testing.T) {
		accounts := []*model.Account{
			makeAcc(1, "Assets:Bank", asset, "USD", nil, false),
			makeAcc(2, "Expenses:Food", expense, "USD", nil, false),
		}
		got := filterAccounts(accounts, []string{"A"}, "")
		assert.Len(t, got, 1)
		assert.Equal(t, "Assets:Bank", got[0].Name)
	})

	t.Run("excludes hidden accounts", func(t *testing.T) {
		accounts := []*model.Account{
			makeAcc(1, "Assets:Bank", asset, "USD", nil, false),
			makeAcc(2, "Assets:Old", asset, "USD", nil, true),
		}
		got := filterAccounts(accounts, []string{"A"}, "")
		assert.Len(t, got, 1)
		assert.Equal(t, "Assets:Bank", got[0].Name)
	})

	t.Run("excludes parent (container) accounts", func(t *testing.T) {
		accounts := []*model.Account{
			makeAcc(1, "Assets", asset, "USD", nil, false),
			makeAcc(2, "Assets:Bank", asset, "USD", int64Ptr(1), false),
		}
		got := filterAccounts(accounts, []string{"A"}, "")
		assert.Len(t, got, 1)
		assert.Equal(t, "Assets:Bank", got[0].Name)
	})

	t.Run("empty allowedCurrency applies no currency filter", func(t *testing.T) {
		accounts := []*model.Account{
			makeAcc(1, "Assets:TWDBank", asset, "TWD", nil, false),
			makeAcc(2, "Assets:USDBank", asset, "USD", nil, false),
			makeAcc(3, "Assets:DefaultBank", asset, "", nil, false),
		}
		got := filterAccounts(accounts, []string{"A"}, "")
		assert.Len(t, got, 3)
	})

	t.Run("non-empty allowedCurrency filters to exact currency match", func(t *testing.T) {
		accounts := []*model.Account{
			makeAcc(1, "Assets:TWDBank", asset, "TWD", nil, false),
			makeAcc(2, "Assets:USDBank", asset, "USD", nil, false),
			makeAcc(3, "Assets:DefaultBank", asset, "", nil, false),
		}
		got := filterAccounts(accounts, []string{"A"}, "TWD")
		assert.Len(t, got, 1)
		assert.Equal(t, "Assets:TWDBank", got[0].Name)
	})

	t.Run("filters to USD accounts when allowedCurrency is USD", func(t *testing.T) {
		accounts := []*model.Account{
			makeAcc(1, "Assets:TWDBank", asset, "TWD", nil, false),
			makeAcc(2, "Assets:USDBank", asset, "USD", nil, false),
		}
		got := filterAccounts(accounts, []string{"A"}, "USD")
		assert.Len(t, got, 1)
		assert.Equal(t, "Assets:USDBank", got[0].Name)
	})
}
