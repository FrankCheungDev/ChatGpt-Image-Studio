package accounts

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteAccountStorageWaitsForBusyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.sqlite")
	backend, err := newSQLiteAccountStorage(path)
	if err != nil {
		t.Fatalf("newSQLiteAccountStorage() returned error: %v", err)
	}
	storage := backend.(*sqliteAccountStorage)
	t.Cleanup(func() { _ = storage.Close() })
	if err := storage.Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	lockDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(lockDB) returned error: %v", err)
	}
	t.Cleanup(func() { _ = lockDB.Close() })
	if _, err := lockDB.Exec(`BEGIN IMMEDIATE`); err != nil {
		t.Fatalf("BEGIN IMMEDIATE returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- storage.SaveAuthRaw("busy.json", []byte(`{"access_token":"busy-token"}`))
	}()

	select {
	case err := <-done:
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "locked") {
			t.Fatalf("SaveAuthRaw returned SQLITE_BUSY instead of waiting: %v", err)
		}
		t.Fatalf("SaveAuthRaw returned before the writer lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if _, err := lockDB.Exec(`COMMIT`); err != nil {
		t.Fatalf("COMMIT returned error: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SaveAuthRaw after lock release returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SaveAuthRaw did not complete after writer lock was released")
	}
}
