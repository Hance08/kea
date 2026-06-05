// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package ledger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FreshInstall(t *testing.T) {
	dir := t.TempDir()

	r, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r.Ledgers, "default", "fresh install should bootstrap a default ledger")
	assert.Equal(t, filepath.Join(dir, "kea.db"), r.Ledgers["default"].Path)
	assert.Equal(t, "default", r.ActiveLedger)
	_, statErr := os.Stat(filepath.Join(dir, "ledgers.yaml"))
	assert.NoError(t, statErr, "ledgers.yaml should be created on fresh install")
}

func TestLoad_AutoMigratesLegacyDB(t *testing.T) {
	dir := t.TempDir()
	legacyDB := filepath.Join(dir, "kea.db")
	require.NoError(t, os.WriteFile(legacyDB, []byte(""), 0644))

	r, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r.Ledgers, "default")
	assert.Equal(t, legacyDB, r.Ledgers["default"].Path)
	assert.Equal(t, "default", r.ActiveLedger)
	assert.True(t, r.MigratedLegacy, "should signal migration to caller")
}

func TestLoad_NormalLoad(t *testing.T) {
	dir := t.TempDir()
	r1, err := Load(dir)
	require.NoError(t, err)
	r1.Ledgers["work"] = Entry{Path: "/tmp/work.db"}
	r1.ActiveLedger = "work"
	require.NoError(t, r1.Save())

	r2, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r2.Ledgers, "work")
	assert.Equal(t, "/tmp/work.db", r2.Ledgers["work"].Path)
	assert.Equal(t, "work", r2.ActiveLedger)
}

func TestAdd_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Add("personal", "/tmp/personal.db")

	require.NoError(t, err)
	assert.Equal(t, "/tmp/personal.db", r.Ledgers["personal"].Path)
}

func TestAdd_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	err = r.Add("personal", "/tmp/other.db")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLedgerExists)
	assert.Equal(t, "/tmp/personal.db", r.Ledgers["personal"].Path, "existing entry must not be overwritten")
}

func TestAdd_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.Contains(t, r2.Ledgers, "work")
}

func TestSwitch_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	err = r.Switch("work")

	require.NoError(t, err)
	assert.Equal(t, "work", r.ActiveLedger)
}

func TestSwitch_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Switch("nonexistent")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLedgerNotFound)
}

func TestSwitch_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "work", r2.ActiveLedger)
}

func TestActive_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	path, err := r.Active()

	require.NoError(t, err)
	assert.Equal(t, "/tmp/work.db", path)
}

func TestActive_NoActiveLedger(t *testing.T) {
	// Construct a registry with no active ledger directly (e.g. corrupted/manually edited ledgers.yaml).
	r := &Registry{Ledgers: map[string]Entry{"work": {Path: "/tmp/work.db"}}}

	_, err := r.Active()

	assert.ErrorIs(t, err, ErrNoActiveLedger)
}

func TestActive_EnvVarOverride(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))
	require.NoError(t, r.Switch("work"))
	t.Setenv("KEA_LEDGER", "personal")

	path, err := r.Active()

	require.NoError(t, err)
	assert.Equal(t, "/tmp/personal.db", path)
}

func TestActive_UnregisteredActiveName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	r.ActiveLedger = "ghost"

	_, err = r.Active()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestRemove_UnregisterOnly(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	dbFile := filepath.Join(dir, "personal.db")
	require.NoError(t, os.WriteFile(dbFile, []byte(""), 0644))
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", dbFile))
	require.NoError(t, r.Switch("work"))

	err = r.Remove("personal", false)

	require.NoError(t, err)
	assert.NotContains(t, r.Ledgers, "personal")
	_, statErr := os.Stat(dbFile)
	assert.NoError(t, statErr, "file should NOT be deleted when deleteFile=false")
}

func TestRemove_RefusesActiveLedger(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	err = r.Remove("work", false)

	assert.ErrorIs(t, err, ErrRemoveActive)
}

func TestRemove_DeleteFile(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	dbFile := filepath.Join(dir, "personal.db")
	require.NoError(t, os.WriteFile(dbFile, []byte(""), 0644))
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", dbFile))
	require.NoError(t, r.Switch("work"))

	err = r.Remove("personal", true)

	require.NoError(t, err)
	_, statErr := os.Stat(dbFile)
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "database file should be deleted")
}

func TestRemove_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Remove("nonexistent", false)

	assert.ErrorIs(t, err, ErrLedgerNotFound)
}

func TestRemove_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))
	require.NoError(t, r.Switch("work"))
	require.NoError(t, r.Remove("personal", false))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.NotContains(t, r2.Ledgers, "personal")
}

func TestWatch_FiresOnSwitch(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	var gotName, gotPath string
	done := make(chan struct{})
	r.OnSwitch(func(name, path string) {
		gotName = name
		gotPath = path
		close(done)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = r.Watch(ctx)
	}()

	// Give the watcher time to start.
	time.Sleep(200 * time.Millisecond)

	// Simulate an external process switching the ledger.
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("work"))

	select {
	case <-done:
		assert.Equal(t, "work", gotName)
		assert.Equal(t, "/tmp/work.db", gotPath)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OnSwitch callback")
	}
}

func TestWatch_NoCallbackWhenActiveUnchanged(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	called := make(chan struct{}, 1)
	r.OnSwitch(func(name, path string) {
		called <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = r.Watch(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Write the same active ledger — should NOT fire callback.
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("default"))

	select {
	case <-called:
		t.Fatal("callback should not fire when active ledger is unchanged")
	case <-time.After(500 * time.Millisecond):
		// OK — no callback fired.
	}
}

func TestWatch_StopWatchPreventsCallbacks(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	called := make(chan struct{}, 1)
	r.OnSwitch(func(name, path string) {
		called <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		_ = r.Watch(ctx)
		close(watchDone)
	}()

	time.Sleep(200 * time.Millisecond)

	// Stop the watcher.
	r.StopWatch()
	cancel()
	<-watchDone

	// Switch after stop — no callback should fire.
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("work"))

	select {
	case <-called:
		t.Fatal("callback should not fire after StopWatch")
	case <-time.After(500 * time.Millisecond):
		// OK.
	}
}

func TestWatch_DebouncesRapidWrites(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	var callCount int32
	var (
		lastNameMu sync.Mutex
		lastName   string
	)
	done := make(chan struct{}, 10)
	r.OnSwitch(func(name, path string) {
		atomic.AddInt32(&callCount, 1)
		lastNameMu.Lock()
		lastName = name
		lastNameMu.Unlock()
		done <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		_ = r.Watch(ctx)
		close(watchDone)
	}()

	time.Sleep(200 * time.Millisecond)

	// Rapid-fire switches: default → work → personal (within debounce window).
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("work"))
	require.NoError(t, r2.Switch("personal"))

	select {
	case <-done:
		// Wait a bit more to see if extra callbacks arrive.
		time.Sleep(300 * time.Millisecond)
		count := atomic.LoadInt32(&callCount)
		lastNameMu.Lock()
		last := lastName
		lastNameMu.Unlock()
		assert.Equal(t, int32(1), count, "debounce should coalesce rapid writes into a single callback")
		assert.Equal(t, "personal", last, "should reflect the final state")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OnSwitch callback")
	}

	cancel()
	<-watchDone
}

// TestRegistry_ConcurrentAccess stresses every Registry accessor that touches
// r.ActiveLedger or r.Ledgers from multiple goroutines. With unsynchronized
// access (the pre-fix state) Go's runtime panics with "concurrent map read
// and map write" — that's the loud failure mode this test pins. Run under
// -race for additional data-race coverage.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("seed1", "/tmp/seed1.db"))
	require.NoError(t, r.Add("seed2", "/tmp/seed2.db"))

	const goroutines = 8
	const dur = 100 * time.Millisecond

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				switch i % 6 {
				case 0:
					name := fmt.Sprintf("g%d-%d", id, i)
					_ = r.Add(name, "/tmp/"+name+".db")
				case 1:
					_ = r.Switch("seed1")
				case 2:
					_, _ = r.EntryFor("seed1")
				case 3:
					_ = r.Names()
				case 4:
					_ = r.ActiveName()
				case 5:
					_, _ = r.Active()
				}
				i++
			}
		}(g)
	}
	time.Sleep(dur)
	close(stop)
	wg.Wait()
}

// TestRegistry_SwitchVsReload reproduces the production-shape race: the
// fsnotify watcher's reload() rewrites r.ActiveLedger and r.Ledgers under
// the lock, while an in-process Switch caller mutates them concurrently
// without it. Each Switch persists ledgers.yaml, which triggers reload()
// after the debounce window. With the fix in place this completes cleanly
// under -race; without the fix `-race` flags the data race on r.ActiveLedger.
func TestRegistry_SwitchVsReload(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		_ = r.Watch(ctx)
		close(watchDone)
	}()
	// Let the watcher install its fsnotify hook before we start mutating.
	time.Sleep(200 * time.Millisecond)

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := r.Switch("work"); err != nil {
			t.Fatalf("Switch(work): %v", err)
		}
		if err := r.Switch("personal"); err != nil {
			t.Fatalf("Switch(personal): %v", err)
		}
	}

	assert.Equal(t, "personal", r.ActiveName(), "last Switch in the loop sets personal")
	cancel()
	<-watchDone
}
