package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	Write *sql.DB
	Read  *sql.DB
}

func Open(path string) (*DB, error) {
	dsn := path + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"

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

	if err := write.Ping(); err != nil {
		write.Close()
		read.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return &DB{Write: write, Read: read}, nil
}

func (d *DB) Close() error {
	werr := d.Write.Close()
	rerr := d.Read.Close()
	if werr != nil {
		return werr
	}
	return rerr
}
