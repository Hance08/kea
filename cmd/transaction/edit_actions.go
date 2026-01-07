package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/constant"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/utils"
)

func (r *editRunner) actionEditBasicInfo(detail *service.TransactionDetail) error {
	// Description
	desc, err := r.view.AskDescription(detail.Description)
	if err != nil {
		return err
	}
	detail.Description = desc

	// Date
	timestamp, err := r.view.AskDate(detail.Timestamp)
	if err != nil {
		return err
	}
	detail.Timestamp = timestamp

	// Status
	status, err := r.view.AskStatus(detail.Status)
	if err != nil {
		return err
	}
	detail.Status = status

	r.view.ShowSuccess("Basic info updated")
	return nil
}

func (r *editRunner) actionQuickChangeAccount(detail *service.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		return fmt.Errorf("quick edit supports only 2 splits")
	}

	// Determine Type (Asset/Expense/etc)
	txType, err := r.txSvc.DetermineType(detail.Splits)
	if err != nil {
		return err
	}

	if txType == service.TxTypeOpening {
		r.view.ShowWarning("Cannot quick-edit Opening Balance transaction")
		return nil
	}

	// Select which split to change
	splitIndex, err := r.view.AskSplitSelection(detail.Splits)
	if err != nil {
		return err // User cancelled or error
	}
	if splitIndex == -1 {
		return nil // Cancelled
	}

	targetSplit := &detail.Splits[splitIndex]

	// Filter compatible accounts
	allAccounts, err := r.accSvc.GetAllAccounts()
	if err != nil {
		return err
	}

	currentAcc, err := r.accSvc.GetAccountByName(targetSplit.AccountName)
	if err != nil {
		return fmt.Errorf("failed to load account details for '%s': %w", targetSplit.AccountName, err)
	}

	compatibleAccounts := r.txSvc.GetAllowedAccounts(txType, currentAcc.Type, allAccounts)

	if len(compatibleAccounts) == 0 {
		r.view.ShowWarning("No other compatible accounts found")
		return nil
	}

	// Select new account
	newAccName, err := r.view.AskAccountFromList(compatibleAccounts, targetSplit.AccountName)
	if err != nil {
		return err
	}

	newAcc, _ := r.accSvc.GetAccountByName(newAccName)

	// Apply Change
	targetSplit.AccountID = newAcc.ID
	targetSplit.AccountName = newAcc.Name
	targetSplit.Currency = newAcc.Currency

	// Show current state
	r.view.RenderSplitsPreview(detail.Splits)
	r.view.ShowSuccess(fmt.Sprintf("Account changed to: %s", newAcc.Name))

	return nil
}

func (r *editRunner) actionQuickChangeAmount(detail *service.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		return fmt.Errorf("quick edit supports only 2 splits")
	}

	// UI: Show Current
	currentAbsAmount := utils.AbsInt64(detail.Splits[0].Amount)
	newAbsAmount, err := r.view.AskAmount("Enter new amount:", currentAbsAmount, false)
	if err != nil {
		return err
	}

	// Logic: Update Amount Preserving Balance
	if err := detail.UpdateAmountPreservingBalance(newAbsAmount); err != nil {
		return err
	}

	// Feedback
	r.view.RenderSplitsPreview(detail.Splits)
	r.view.ShowSuccess("Amount updated")
	return nil
}

func (r *editRunner) runSplitsMenu(detail *service.TransactionDetail) error {
	for {
		if err := r.view.RenderDetail(detail); err != nil {
			return err
		}

		action, err := r.view.AskSelection("Splits Editor:", []string{
			"Add Split", "Edit Split", "Delete Split", "Done",
		})
		if err != nil {
			return err
		}

		switch action {
		case "Add Split":
			if err := r.actionAddSplit(detail); err != nil {
				return err
			}
		case "Edit Split":
			if err := r.actionEditSplit(detail); err != nil {
				return err
			}
		case "Delete Split":
			if err := r.actionDeleteSplit(detail); err != nil {
				return err
			}
		case "Done":
			return nil
		}
	}
}

func (r *editRunner) actionAddSplit(detail *service.TransactionDetail) error {
	// Prepare Data
	accounts, err := r.accSvc.GetAllAccounts()
	if err != nil {
		return err
	}

	// Ask Questions
	accName, err := r.view.AskAccountFromList(accounts, "")
	if err != nil {
		return err
	}

	amount, err := r.view.AskAmount("Amount (negative for credit):", 0, false)
	if err != nil {
		return err
	}

	memo, err := r.view.AskInput("Memo (optional):", "")
	if err != nil {
		return err
	}

	// Update Model
	acc, _ := r.accSvc.GetAccountByName(accName)
	detail.Splits = append(detail.Splits, service.SplitDetail{
		AccountID: acc.ID, AccountName: acc.Name, Currency: acc.Currency,
		Amount: amount, Memo: memo,
	})
	return nil
}

func (r *editRunner) actionEditSplit(detail *service.TransactionDetail) error {
	// Select Split
	idx, err := r.view.AskSplitSelection(detail.Splits)
	if err != nil || idx == -1 {
		return err
	}
	split := &detail.Splits[idx]

	// Prepare Data
	accounts, err := r.accSvc.GetAllAccounts()
	if err != nil {
		return err
	}

	// Ask Questions
	newAccName, err := r.view.AskAccountFromList(accounts, split.AccountName)
	if err != nil {
		return err
	}

	newAmount, err := r.view.AskAmount("Amount:", split.Amount, true)
	if err != nil {
		return err
	}

	newMemo, err := r.view.AskInput("Memo:", split.Memo)
	if err != nil {
		return err
	}

	// Update Model
	acc, _ := r.accSvc.GetAccountByName(newAccName)
	split.AccountID = acc.ID
	split.AccountName = acc.Name
	split.Currency = acc.Currency
	split.Amount = newAmount
	split.Memo = newMemo

	return nil
}

func (r *editRunner) actionDeleteSplit(detail *service.TransactionDetail) error {
	if len(detail.Splits) <= constant.MinSplitsCount {
		r.view.ShowWarning(fmt.Sprintf("Transaction must have at least %d splits", constant.MinSplitsCount))
		return nil
	}

	idx, err := r.view.AskSplitSelection(detail.Splits)
	if err != nil || idx == -1 {
		return err
	}

	if r.view.AskConfirm("Delete this split?") {
		// Slice deletion
		detail.Splits = append(detail.Splits[:idx], detail.Splits[idx+1:]...)
	}
	return nil
}

func (r *editRunner) actionSave(detail *service.TransactionDetail) error {
	// Validate via Service
	splits := detail.ToSplitInputs()
	if err := r.txSvc.ValidateTransactionEdit(splits); err != nil {
		return err
	}

	// Execute Update
	if err := r.txSvc.UpdateTransactionComplete(
		r.txID, detail.Description, detail.Timestamp, detail.Status, splits,
	); err != nil {
		return err
	}

	r.view.ShowSuccess(fmt.Sprintf("Transaction (ID: %d) saved successfully", r.txID))

	return nil
}
