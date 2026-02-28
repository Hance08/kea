package views

import (
	"fmt"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type AccountSummaryItem struct {
	FullName    string
	Type        model.AccountType
	Currency    string
	Balance     int64
	Description string
}

type AccountCreateView struct {
	*CommonView
}

func NewAccountCreateView() *AccountCreateView {
	return &AccountCreateView{
		CommonView: NewCommonView(),
	}
}

// ==========================================
// Data Rendering
// ==========================================

func (v *AccountCreateView) RenderSummary(data AccountSummaryItem) error {
	balanceStr := utils.FormatAmount(data.Balance)

	descStr := data.Description
	if descStr == "" {
		descStr = "-"
	}

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{})
	table.SetBorder(false)
	table.SetColumnSeparator(":")
	table.SetAutoWrapText(false)

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
	})

	headerStyle := pterm.NewStyle(pterm.FgCyan, pterm.Bold)
	rows := [][]string{
		{headerStyle.Sprint("Full Name"), data.FullName},
		{headerStyle.Sprint("Type"), string(data.Type)},
		{headerStyle.Sprint("Currency"), data.Currency},
		{headerStyle.Sprint("Balance"), balanceStr},
		{headerStyle.Sprint("Description"), descStr},
	}

	table.AppendBulk(rows)
	pterm.Println()
	table.Render()
	pterm.Println()

	return nil
}

// ==========================================
// Generic Prompts
// ==========================================

// ==========================================
// Specific Domain Prompts
// ==========================================

func (v *AccountCreateView) AskAccountType() (string, error) {
	return prompts.PromptAccountType()
}

func (v *AccountCreateView) AskIsSubAccount() (bool, error) {
	return prompts.PromptIsSubAccount()
}

func (v *AccountCreateView) AskParentAccount(allAccounts []*model.Account) (*model.Account, error) {
	_, selectedAccount, err := prompts.PromptParentAccount(allAccounts)
	if err != nil {
		return nil, err
	}

	if selectedAccount == nil {
		return nil, fmt.Errorf("no account selected")
	}

	return selectedAccount, nil
}
