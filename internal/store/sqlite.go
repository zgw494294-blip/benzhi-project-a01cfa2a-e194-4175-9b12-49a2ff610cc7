package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/sqlite3local"
)

type SQLiteRepository struct{ db *sql.DB }

func OpenSQLite(ctx context.Context, dataDir string) (*SQLiteRepository, error) {
	path := filepath.Join(dataDir, "inspection.db")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 SQLite: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	repo := &SQLiteRepository{db: db}
	if err := repo.CheckIntegrity(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Close() error { return r.db.Close() }
