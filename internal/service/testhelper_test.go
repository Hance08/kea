// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

// testhelper_test.go provides shared mock infrastructure for all service-layer tests.
// Uses package service (white-box) to directly construct unexported structs.

import (
	"context"
	"errors"
	"fmt"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

// ──────────────────────────────────────────────
// mockAccountRepo
// ──────────────────────────────────────────────

type mockAccountRepo struct {
	accountsByName map[string]*model.Account
	accountsByID   map[int64]*model.Account
	balances       map[int64]int64
	childMap       map[int64]bool // accountID → has children
	txExistsMap    map[int64]bool // accountID → has transactions
	nextID         int64

	// injectable errors
	createErr         error
	deleteErr         error
	getByNameErr      map[string]error
	getByIDErr        map[int64]error
	getBalanceErr     map[int64]error
	getAllBalancesErr error

	// call recorders
	renameCalls         []struct{ old, new string }
	updateMetadataCalls []struct {
		id          int64
		description string
		isHidden    bool
	}
	updateMetadataErr error
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		accountsByName: make(map[string]*model.Account),
		accountsByID:   make(map[int64]*model.Account),
		balances:       make(map[int64]int64),
		childMap:       make(map[int64]bool),
		txExistsMap:    make(map[int64]bool),
		getByNameErr:   make(map[string]error),
		getByIDErr:     make(map[int64]error),
		getBalanceErr:  make(map[int64]error),
		nextID:         1,
	}
}

// addAccount is a test helper that injects account data directly.
func (m *mockAccountRepo) addAccount(acc *model.Account) {
	m.accountsByName[acc.Name] = acc
	m.accountsByID[acc.ID] = acc
}

func (m *mockAccountRepo) CreateAccount(_ context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	if _, exists := m.accountsByName[name]; exists {
		return 0, fmt.Errorf("account %q already exists: %w", name, repository.ErrAlreadyExists)
	}
	id := m.nextID
	m.nextID++
	acc := &model.Account{
		ID:          id,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
	}
	m.accountsByName[name] = acc
	m.accountsByID[id] = acc
	return id, nil
}

func (m *mockAccountRepo) GetAllAccounts(_ context.Context) ([]*model.Account, error) {
	result := make([]*model.Account, 0, len(m.accountsByID))
	for _, acc := range m.accountsByID {
		result = append(result, acc)
	}
	return result, nil
}

func (m *mockAccountRepo) GetAccountByName(_ context.Context, name string) (*model.Account, error) {
	if err, ok := m.getByNameErr[name]; ok {
		return nil, err
	}
	acc, ok := m.accountsByName[name]
	if !ok {
		return nil, fmt.Errorf("account %q not found: %w", name, repository.ErrNotFound)
	}
	return acc, nil
}

func (m *mockAccountRepo) GetAccountByID(_ context.Context, id int64) (*model.Account, error) {
	if err, ok := m.getByIDErr[id]; ok {
		return nil, err
	}
	acc, ok := m.accountsByID[id]
	if !ok {
		return nil, fmt.Errorf("account ID %d not found: %w", id, repository.ErrNotFound)
	}
	return acc, nil
}

func (m *mockAccountRepo) AccountExists(_ context.Context, name string) (bool, error) {
	_, ok := m.accountsByName[name]
	return ok, nil
}

func (m *mockAccountRepo) GetAccountsByType(_ context.Context, accType model.AccountType) ([]*model.Account, error) {
	var result []*model.Account
	for _, acc := range m.accountsByID {
		if acc.Type == accType {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepo) GetAccountBalance(_ context.Context, accountID int64) (int64, error) {
	if err, ok := m.getBalanceErr[accountID]; ok {
		return 0, err
	}
	return m.balances[accountID], nil
}

func (m *mockAccountRepo) GetAllAccountBalances(_ context.Context, _ int64) (map[int64]int64, error) {
	if m.getAllBalancesErr != nil {
		return nil, m.getAllBalancesErr
	}
	result := make(map[int64]int64, len(m.balances))
	for id, bal := range m.balances {
		result[id] = bal
	}
	return result, nil
}

func (m *mockAccountRepo) HasChildAccounts(_ context.Context, accountID int64) (bool, error) {
	return m.childMap[accountID], nil
}

func (m *mockAccountRepo) AccountHasTransactions(_ context.Context, accountID int64) (bool, error) {
	return m.txExistsMap[accountID], nil
}

func (m *mockAccountRepo) DeleteAccount(_ context.Context, accountID int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	acc, ok := m.accountsByID[accountID]
	if !ok {
		return fmt.Errorf("account ID %d not found", accountID)
	}
	delete(m.accountsByName, acc.Name)
	delete(m.accountsByID, accountID)
	return nil
}

func (m *mockAccountRepo) RenameAccount(_ context.Context, oldName, newName string) error {
	m.renameCalls = append(m.renameCalls, struct{ old, new string }{oldName, newName})
	acc, ok := m.accountsByName[oldName]
	if !ok {
		return fmt.Errorf("account %q not found", oldName)
	}
	delete(m.accountsByName, oldName)
	acc.Name = newName
	m.accountsByName[newName] = acc
	return nil
}

func (m *mockAccountRepo) UpdateAccountMetadata(_ context.Context, accountID int64, description string, isHidden bool) error {
	if m.updateMetadataErr != nil {
		return m.updateMetadataErr
	}
	acc, ok := m.accountsByID[accountID]
	if !ok {
		return fmt.Errorf("account ID %d not found", accountID)
	}
	m.updateMetadataCalls = append(m.updateMetadataCalls, struct {
		id          int64
		description string
		isHidden    bool
	}{accountID, description, isHidden})
	acc.Description = description
	acc.IsHidden = isHidden
	return nil
}

// ──────────────────────────────────────────────
// mockTransactionRepo
// ──────────────────────────────────────────────

type mockTransactionRepo struct {
	transactions map[int64]*model.Transaction
	splits       map[int64][]*model.Split // txID → splits
	nextTxID     int64
	nextSplitID  int64

	// injectable errors
	createErr  error
	deleteErr  error
	updateErr  error
	getByIDErr map[int64]error

	// default return for GetSplitsWithAccountsByDateRange
	splitsWithAccts map[int64][]model.SplitDetail
	splitsRangeErr  error

	// default return for GetTransactionsByDateRange
	txsByDateRangeErr error

	// call recorders for interaction verification
	deleteSplitCalls []int64
	createSplitCalls []*model.Split
	updateSplitCalls []int64

	// reconciliation support
	unreconciledByAccount     map[int64][]*model.ReconcileEntry
	unreconciledByAccountErr  map[int64]error
	bulkUpdateErr             error
	bulkUpdateCalls           [][]int64
	markSplitsReconciledErr   error
	markSplitsReconciledCalls []struct {
		accountID int64
		txIDs     []int64
	}
	markSplitsRowsOverride *int64 // nil = return len(txIDs); non-nil = return *markSplitsRowsOverride

	// last reconciled balance support
	lastReconciledBalances    map[int64]int64
	getLastReconciledBalErr   map[int64]error
	setLastReconciledBalCalls []struct {
		accountID int64
		balance   int64
	}
	setLastReconciledBalErr map[int64]error
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{
		transactions:             make(map[int64]*model.Transaction),
		splits:                   make(map[int64][]*model.Split),
		getByIDErr:               make(map[int64]error),
		splitsWithAccts:          make(map[int64][]model.SplitDetail),
		unreconciledByAccount:    make(map[int64][]*model.ReconcileEntry),
		unreconciledByAccountErr: make(map[int64]error),
		lastReconciledBalances:   make(map[int64]int64),
		getLastReconciledBalErr:  make(map[int64]error),
		setLastReconciledBalErr:  make(map[int64]error),
		nextTxID:                 1,
		nextSplitID:              100,
	}
}

func (m *mockTransactionRepo) addTransaction(tx *model.Transaction, splits []*model.Split) {
	m.transactions[tx.ID] = tx
	m.splits[tx.ID] = splits
}

func (m *mockTransactionRepo) CreateTransactionWithSplits(_ context.Context, tx model.Transaction, splits []model.Split) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	id := m.nextTxID
	m.nextTxID++
	stored := tx
	stored.ID = id
	m.transactions[id] = &stored

	storedSplits := make([]*model.Split, len(splits))
	for i, s := range splits {
		sc := s
		sc.TransactionID = id
		sc.ID = m.nextSplitID
		m.nextSplitID++
		storedSplits[i] = &sc
	}
	m.splits[id] = storedSplits
	return id, nil
}

func (m *mockTransactionRepo) GetTransactionByID(_ context.Context, txID int64) (*model.Transaction, error) {
	if err, ok := m.getByIDErr[txID]; ok {
		return nil, err
	}
	tx, ok := m.transactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction ID %d not found", txID)
	}
	return tx, nil
}

func (m *mockTransactionRepo) GetTransactionsByAccount(_ context.Context, accountID int64, limit int) ([]*model.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepo) GetTransactionsByDateRange(_ context.Context, startTime, endTime int64) ([]*model.Transaction, error) {
	if m.txsByDateRangeErr != nil {
		return nil, m.txsByDateRangeErr
	}
	result := make([]*model.Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		result = append(result, tx)
	}
	return result, nil
}

func (m *mockTransactionRepo) GetAllTransactions(_ context.Context, limit int) ([]*model.Transaction, error) {
	result := make([]*model.Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		result = append(result, tx)
	}
	return result, nil
}

func (m *mockTransactionRepo) UpdateTransactionStatus(_ context.Context, txID int64, status model.TransactionStatus) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	tx, ok := m.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction ID %d not found", txID)
	}
	tx.Status = status
	return nil
}

func (m *mockTransactionRepo) DeleteTransaction(_ context.Context, txID int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.transactions[txID]; !ok {
		return fmt.Errorf("transaction ID %d not found", txID)
	}
	delete(m.transactions, txID)
	delete(m.splits, txID)
	return nil
}

func (m *mockTransactionRepo) UpdateTransactionBasic(_ context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	tx, ok := m.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction ID %d not found", txID)
	}
	tx.Description = description
	tx.Timestamp = timestamp
	tx.Status = status
	tx.Type = txType
	return nil
}

func (m *mockTransactionRepo) CreateSplit(_ context.Context, txID int64, split *model.Split) (int64, error) {
	id := m.nextSplitID
	m.nextSplitID++
	sc := *split
	sc.ID = id
	sc.TransactionID = txID
	m.splits[txID] = append(m.splits[txID], &sc)
	m.createSplitCalls = append(m.createSplitCalls, &sc)
	return id, nil
}

func (m *mockTransactionRepo) UpdateSplit(_ context.Context, splitID int64, accountID int64, amount int64, currency string, memo string) error {
	m.updateSplitCalls = append(m.updateSplitCalls, splitID)
	return nil
}

func (m *mockTransactionRepo) DeleteSplit(_ context.Context, splitID int64) error {
	m.deleteSplitCalls = append(m.deleteSplitCalls, splitID)
	return nil
}

func (m *mockTransactionRepo) GetSplitsByTransaction(_ context.Context, txID int64) ([]*model.Split, error) {
	return m.splits[txID], nil
}

func (m *mockTransactionRepo) GetSplitsWithAccountsByDateRange(_ context.Context, startTime, endTime int64) (map[int64][]model.SplitDetail, error) {
	if m.splitsRangeErr != nil {
		return nil, m.splitsRangeErr
	}
	return m.splitsWithAccts, nil
}

func (m *mockTransactionRepo) GetSplitsWithAccountsByTransaction(_ context.Context, txID int64) ([]model.SplitDetail, error) {
	splits := m.splits[txID]
	result := make([]model.SplitDetail, 0, len(splits))
	for _, s := range splits {
		result = append(result, model.SplitDetail{
			ID:        s.ID,
			AccountID: s.AccountID,
			Amount:    s.Amount,
			Currency:  s.Currency,
			Memo:      s.Memo,
		})
	}
	return result, nil
}

func (m *mockTransactionRepo) GetUnreconciledTransactionsByAccount(_ context.Context, accountID int64) ([]*model.ReconcileEntry, error) {
	if err, ok := m.unreconciledByAccountErr[accountID]; ok {
		return nil, err
	}
	return m.unreconciledByAccount[accountID], nil
}

func (m *mockTransactionRepo) BulkUpdateTransactionStatus(_ context.Context, txIDs []int64, status model.TransactionStatus) error {
	if m.bulkUpdateErr != nil {
		return m.bulkUpdateErr
	}
	ids := make([]int64, len(txIDs))
	copy(ids, txIDs)
	m.bulkUpdateCalls = append(m.bulkUpdateCalls, ids)
	for _, id := range txIDs {
		if tx, ok := m.transactions[id]; ok {
			tx.Status = status
		}
	}
	return nil
}

func (m *mockTransactionRepo) MarkSplitsReconciledByAccount(_ context.Context, accountID int64, txIDs []int64) (int64, error) {
	if m.markSplitsReconciledErr != nil {
		return 0, m.markSplitsReconciledErr
	}
	ids := make([]int64, len(txIDs))
	copy(ids, txIDs)
	m.markSplitsReconciledCalls = append(m.markSplitsReconciledCalls, struct {
		accountID int64
		txIDs     []int64
	}{accountID, ids})
	if m.markSplitsRowsOverride != nil {
		return *m.markSplitsRowsOverride, nil
	}
	return int64(len(txIDs)), nil
}

func (m *mockTransactionRepo) GetLastReconciledBalance(_ context.Context, accountID int64) (int64, error) {
	if err, ok := m.getLastReconciledBalErr[accountID]; ok {
		return 0, err
	}
	return m.lastReconciledBalances[accountID], nil
}

func (m *mockTransactionRepo) SetLastReconciledBalance(_ context.Context, accountID int64, balance int64) error {
	if err, ok := m.setLastReconciledBalErr[accountID]; ok {
		return err
	}
	m.lastReconciledBalances[accountID] = balance
	m.setLastReconciledBalCalls = append(m.setLastReconciledBalCalls, struct {
		accountID int64
		balance   int64
	}{accountID, balance})
	return nil
}

// ──────────────────────────────────────────────
// mockCombinedRepo (implements repository.Repository)
// ──────────────────────────────────────────────

type mockCombinedRepo struct {
	*mockAccountRepo
	*mockTransactionRepo
}

var _ repository.Repository = (*mockCombinedRepo)(nil)

// ──────────────────────────────────────────────
// mockTransactionManager
// ──────────────────────────────────────────────

type mockTransactionManager struct {
	accRepo *mockAccountRepo
	txRepo  *mockTransactionRepo
	failTx  bool
}

func (m *mockTransactionManager) ExecTx(_ context.Context, fn func(repository.Repository) error) error {
	if m.failTx {
		return errors.New("transaction manager: forced failure")
	}

	// Snapshot account state before the transaction so we can roll back.
	accSnapshot := make(map[string]*model.Account, len(m.accRepo.accountsByName))
	for k, v := range m.accRepo.accountsByName {
		accSnapshot[k] = v
	}
	idSnapshot := make(map[int64]*model.Account, len(m.accRepo.accountsByID))
	for k, v := range m.accRepo.accountsByID {
		idSnapshot[k] = v
	}
	nextIDSnapshot := m.accRepo.nextID

	txSnapshot := make(map[int64]*model.Transaction, len(m.txRepo.transactions))
	for k, v := range m.txRepo.transactions {
		txSnapshot[k] = v
	}
	splitsSnapshot := make(map[int64][]*model.Split, len(m.txRepo.splits))
	for k, v := range m.txRepo.splits {
		splitsSnapshot[k] = v
	}
	nextTxIDSnapshot := m.txRepo.nextTxID
	nextSplitIDSnapshot := m.txRepo.nextSplitID

	combined := &mockCombinedRepo{
		mockAccountRepo:     m.accRepo,
		mockTransactionRepo: m.txRepo,
	}
	if err := fn(combined); err != nil {
		// Rollback: restore pre-transaction state.
		m.accRepo.accountsByName = accSnapshot
		m.accRepo.accountsByID = idSnapshot
		m.accRepo.nextID = nextIDSnapshot
		m.txRepo.transactions = txSnapshot
		m.txRepo.splits = splitsSnapshot
		m.txRepo.nextTxID = nextTxIDSnapshot
		m.txRepo.nextSplitID = nextSplitIDSnapshot
		return err
	}
	return nil
}

// ──────────────────────────────────────────────
// Helper factories
// ──────────────────────────────────────────────

func defaultConfig() *config.Config {
	return &config.Config{
		Defaults: config.DefaultsConfig{Currency: "USD"},
	}
}

func newTestTransactionService(accRepo *mockAccountRepo, txRepo *mockTransactionRepo) *TransactionService {
	tm := &mockTransactionManager{accRepo: accRepo, txRepo: txRepo}
	return NewTransactionService(txRepo, accRepo, tm, defaultConfig())
}

func newTestAccountService(accRepo *mockAccountRepo, txRepo *mockTransactionRepo) *AccountService {
	tm := &mockTransactionManager{accRepo: accRepo, txRepo: txRepo}
	return NewAccountService(accRepo, defaultConfig(), tm)
}
