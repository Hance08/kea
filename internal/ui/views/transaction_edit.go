package views

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/ui"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/utils"
	"github.com/pterm/pterm"
)

type TransactionEditView struct {
	splitsView *TransactionSplitsView
	detailView *TransactionDetailView
}

func NewTransactionEditView() *TransactionEditView {
	return &TransactionEditView{
		splitsView: NewTransactionSplitsView(),
		detailView: NewTransactionDetailView(),
	}
}

// ==========================================
// Message Output
// ==========================================

func (v *TransactionEditView) ShowError(msg string, err error) {
	if err != nil {
		pterm.Error.Printf("%s: %v\n", msg, err)
	} else {
		pterm.Error.Println(msg)
	}
}

func (v *TransactionEditView) ShowWarning(msg string) {
	pterm.Warning.Println(msg)
}

func (v *TransactionEditView) ShowSuccess(msg string) {
	pterm.Success.Println(msg)
}

func (v *TransactionEditView) ShowInfo(msg string) {
	pterm.Info.Println(msg)
}

// ==========================================
// Date Rendering
// ==========================================

func (v *TransactionEditView) RenderDetail(detail *model.TransactionDetail) error {
	return v.detailView.Render(detail, false)
}

func (v *TransactionEditView) RenderSplitsPreview(splits []model.SplitDetail) {
	ui.PrintL1Title("Current splitsView")
	v.splitsView.Render(splits)
}

// ==========================================
// Generic Prompts
// ==========================================

func (v *TransactionEditView) AskSelection(label string, options []string) (string, error) {
	return prompts.PromptSelect(label, options, "")
}

func (v *TransactionEditView) AskInput(label, defaultVal string) (string, error) {
	return prompts.PromptInput(label, defaultVal, nil)
}

func (v *TransactionEditView) AskConfirm(label string) bool {
	yes, _ := prompts.PromptConfirm(label, false)
	return yes
}

// ==========================================
// Specific Domain Prompts
// ==========================================

func (v *TransactionEditView) AskDescription(current string) (string, error) {
	return prompts.PromptInput("Description:", current, nil)
}

func (v *TransactionEditView) AskDate(currentTimestamp int64) (int64, error) {
	currentStr := time.Unix(currentTimestamp, 0).Format("2006-01-02")

	dateStr, err := prompts.PromptDate("Date (YYYY-MM-DD):", currentStr, "", prompts.ValidateDateFormat(true))
	if err != nil {
		return 0, err
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, fmt.Errorf("unexpected date format error: %w", err)
	}
	return t.Unix(), nil
}

func (v *TransactionEditView) AskStatus(current model.TransactionStatus) (model.TransactionStatus, error) {
	curStr := "Cleared"
	if current == model.StatusPending {
		curStr = "Pending"
	}

	statusStr, err := prompts.PromptTransactionStatus(curStr)
	if err != nil {
		return 0, err
	}

	if statusStr == "Pending" {
		return model.StatusPending, nil
	}
	return model.StatusCleared, nil
}

func (v *TransactionEditView) AskAccountFromList(accounts []*model.Account, defaultName string) (string, error) {
	var names []string
	for _, a := range accounts {
		names = append(names, a.Name)
	}
	return prompts.PromptSelect("Select Account:", names, defaultName)
}

func (v *TransactionEditView) AskAmount(label string, currentCents int64, allowEmpty bool) (int64, error) {
	defaultStr := ""
	if currentCents != 0 {
		defaultStr = utils.FormatAmount(currentCents)
	}

	amountStr, err := prompts.PromptAmount(label, defaultStr, prompts.ValidateAmountFormat(allowEmpty))
	if err != nil {
		return 0, err
	}

	return utils.ParseAmount(amountStr)
}

func (v *TransactionEditView) AskSplitSelection(splits []model.SplitDetail) (int, error) {
	var options []string
	for i, s := range splits {
		options = append(options, fmt.Sprintf("#%d %s (%s)", i+1, s.AccountName, utils.FormatAmount(s.Amount)))
	}
	options = append(options, "Cancel")

	choice, err := prompts.PromptSelect("Select Split:", options, "")
	if err != nil {
		return -1, err
	}
	if choice == "Cancel" {
		return -1, nil
	}

	var idx int
	_, err = fmt.Sscanf(choice, "#%d", &idx)
	if err != nil {
		return -1, fmt.Errorf("failed to parse selection: %w", err)
	}

	return idx - 1, nil
}
