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
	createErr          error
	deleteErr          error
	getByNameErr       map[string]error
	getByIDErr         map[int64]error
	getBalanceErr      map[int64]error
	getAllBalancesErr   error
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

func (m *mockAccountRepo) CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	if _, exists := m.accountsByName[name]; exists {
		return 0, errors.New("account already exists")
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

func (m *mockAccountRepo) GetAllAccounts() ([]*model.Account, error) {
	result := make([]*model.Account, 0, len(m.accountsByID))
	for _, acc := range m.accountsByID {
		result = append(result, acc)
	}
	return result, nil
}

func (m *mockAccountRepo) GetAccountByName(name string) (*model.Account, error) {
	if err, ok := m.getByNameErr[name]; ok {
		return nil, err
	}
	acc, ok := m.accountsByName[name]
	if !ok {
		return nil, fmt.Errorf("account %q not found", name)
	}
	return acc, nil
}

func (m *mockAccountRepo) GetAccountByID(id int64) (*model.Account, error) {
	if err, ok := m.getByIDErr[id]; ok {
		return nil, err
	}
	acc, ok := m.accountsByID[id]
	if !ok {
		return nil, fmt.Errorf("account ID %d not found", id)
	}
	return acc, nil
}

func (m *mockAccountRepo) AccountExists(name string) (bool, error) {
	_, ok := m.accountsByName[name]
	return ok, nil
}

func (m *mockAccountRepo) GetAccountsByType(accType model.AccountType) ([]*model.Account, error) {
	var result []*model.Account
	for _, acc := range m.accountsByID {
		if acc.Type == accType {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepo) GetAccountBalance(accountID int64) (int64, error) {
	if err, ok := m.getBalanceErr[accountID]; ok {
		return 0, err
	}
	return m.balances[accountID], nil
}

func (m *mockAccountRepo) GetAllAccountBalances(_ int64) (map[int64]int64, error) {
	if m.getAllBalancesErr != nil {
		return nil, m.getAllBalancesErr
	}
	result := make(map[int64]int64, len(m.balances))
	for id, bal := range m.balances {
		result[id] = bal
	}
	return result, nil
}

func (m *mockAccountRepo) HasChildAccounts(accountID int64) (bool, error) {
	return m.childMap[accountID], nil
}

func (m *mockAccountRepo) AccountHasTransactions(accountID int64) (bool, error) {
	return m.txExistsMap[accountID], nil
}

func (m *mockAccountRepo) DeleteAccount(accountID int64) error {
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

	// call recorders for interaction verification
	deleteSplitCalls []int64
	createSplitCalls []*model.Split
	updateSplitCalls []int64
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{
		transactions:    make(map[int64]*model.Transaction),
		splits:          make(map[int64][]*model.Split),
		getByIDErr:      make(map[int64]error),
		splitsWithAccts: make(map[int64][]model.SplitDetail),
		nextTxID:        1,
		nextSplitID:     100,
	}
}

func (m *mockTransactionRepo) addTransaction(tx *model.Transaction, splits []*model.Split) {
	m.transactions[tx.ID] = tx
	m.splits[tx.ID] = splits
}

func (m *mockTransactionRepo) CreateTransactionWithSplits(tx model.Transaction, splits []model.Split) (int64, error) {
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

func (m *mockTransactionRepo) GetTransactionByID(txID int64) (*model.Transaction, error) {
	if err, ok := m.getByIDErr[txID]; ok {
		return nil, err
	}
	tx, ok := m.transactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction ID %d not found", txID)
	}
	return tx, nil
}

func (m *mockTransactionRepo) GetTransactionsByAccount(accountID int64, limit int) ([]*model.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepo) GetTransactionsByDateRange(startTime, endTime int64) ([]*model.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionRepo) GetAllTransactions(limit int) ([]*model.Transaction, error) {
	result := make([]*model.Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		result = append(result, tx)
	}
	return result, nil
}

func (m *mockTransactionRepo) UpdateTransactionStatus(txID int64, status model.TransactionStatus) error {
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

func (m *mockTransactionRepo) DeleteTransaction(txID int64) error {
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

func (m *mockTransactionRepo) UpdateTransactionBasic(txID int64, description string, timestamp int64, status model.TransactionStatus) error {
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
	return nil
}

func (m *mockTransactionRepo) CreateSplit(txID int64, split *model.Split) (int64, error) {
	id := m.nextSplitID
	m.nextSplitID++
	sc := *split
	sc.ID = id
	sc.TransactionID = txID
	m.splits[txID] = append(m.splits[txID], &sc)
	m.createSplitCalls = append(m.createSplitCalls, &sc)
	return id, nil
}

func (m *mockTransactionRepo) UpdateSplit(splitID int64, accountID int64, amount int64, currency string, memo string) error {
	m.updateSplitCalls = append(m.updateSplitCalls, splitID)
	return nil
}

func (m *mockTransactionRepo) DeleteSplit(splitID int64) error {
	m.deleteSplitCalls = append(m.deleteSplitCalls, splitID)
	return nil
}

func (m *mockTransactionRepo) GetSplitsByTransaction(txID int64) ([]*model.Split, error) {
	return m.splits[txID], nil
}

func (m *mockTransactionRepo) GetSplitsWithAccountsByDateRange(startTime, endTime int64) (map[int64][]model.SplitDetail, error) {
	if m.splitsRangeErr != nil {
		return nil, m.splitsRangeErr
	}
	return m.splitsWithAccts, nil
}

func (m *mockTransactionRepo) GetSplitsWithAccountsByTransaction(txID int64) ([]model.SplitDetail, error) {
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
	combined := &mockCombinedRepo{
		mockAccountRepo:     m.accRepo,
		mockTransactionRepo: m.txRepo,
	}
	return fn(combined)
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
