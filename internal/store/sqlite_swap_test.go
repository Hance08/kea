package store_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwap_QueriesHitNewDB(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")
	dbB := filepath.Join(dir, "b.db")

	s, err := store.NewStore(dbA, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Create an account in DB A.
	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankA", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Pre-create DB B with its own account.
	sB, err := store.NewStore(dbB, migrations.FS)
	require.NoError(t, err)
	err = sB.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankB", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, sB.Close())

	// Swap to DB B.
	err = s.Swap(dbB, migrations.FS)
	require.NoError(t, err)

	// After swap, BankA should not exist; BankB should.
	_, err = s.GetAccountByName(ctx, "Assets:BankA")
	assert.Error(t, err, "BankA should not exist in DB B")

	acc, err := s.GetAccountByName(ctx, "Assets:BankB")
	require.NoError(t, err)
	assert.Equal(t, "Assets:BankB", acc.Name)
}

func TestSwap_FailedSwapKeepsOldConnection(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")

	s, err := store.NewStore(dbA, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankA", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Swap to an invalid path — should fail.
	err = s.Swap("/nonexistent/dir/bad.db", migrations.FS)
	require.Error(t, err)

	// Old connection should still work.
	acc, err := s.GetAccountByName(ctx, "Assets:BankA")
	require.NoError(t, err)
	assert.Equal(t, "Assets:BankA", acc.Name)
}

func TestSwap_ConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")
	dbB := filepath.Join(dir, "b.db")

	s, err := store.NewStore(dbA, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankA", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Pre-create DB B.
	sB, err := store.NewStore(dbB, migrations.FS)
	require.NoError(t, err)
	err = sB.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankB", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, sB.Close())

	// Launch concurrent readers and a swap in the middle.
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.GetAllAccounts(ctx)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Swap(dbB, migrations.FS); err != nil {
			errs <- err
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent operation failed: %v", err)
	}
}
