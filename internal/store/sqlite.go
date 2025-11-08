package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string, migrationsPath string) (*Store, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("can not create database directory %s: %w", dbDir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("can not open database : %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("can not connect with database : %w", err)
	}
	if err := runMigrations(db, migrationsPath); err != nil {
		return nil, fmt.Errorf("failed to migrate database : %w", err)
	}
	fmt.Println("Successfully migrate database !")

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func runMigrations(db *sql.DB, migrationsPath string) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to set up migrate driver : %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"sqlite3",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to set up migrate instance : %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migration(up) : %w", err)
	}

	return nil
}

func (s *Store) CreateAccount(name, accType, description string, parentID *int64) (int64, error) {
	stmt, err := s.db.Prepare(`
		INSERT INFO accounts (name, type, description, parent_id)
		VALUE (?, ?, ?, ?, ?)
		RETURNING id;
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare SQL : %w", err)
	}
	defer stmt.Close()

	var newID int64

	err = stmt.QueryRow(name, accType, description, parentID).Scan(&newID)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: accounts.name") {
			return 0, fmt.Errorf("account name '%s' is already existed", name)
		}
		return 0, fmt.Errorf("failed to executing SQL insertion : %w", err)
	}

	return newID, nil
}