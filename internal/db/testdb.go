package db

import (
	"database/sql"
	"fmt"
)

// OpenMemory creates an in-memory SQLite database for testing.
// It uses a shared cache so both Write and Read pools access the same data.
func OpenMemory() (*DB, error) {
	dsn := "file::memory:?mode=memory&cache=shared&_foreign_keys=on"

	write, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open write: %w", err)
	}
	write.SetMaxOpenConns(1)
	write.SetMaxIdleConns(1)

	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		write.Close()
		return nil, fmt.Errorf("db: open read: %w", err)
	}
	read.SetMaxOpenConns(4)
	read.SetMaxIdleConns(4)

	return &DB{Write: write, Read: read}, nil
}
