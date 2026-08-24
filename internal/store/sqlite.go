package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
	_ "benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/sqlite3local"
)

type SQLiteRepository struct {
	db      *sql.DB
	cacheMu sync.RWMutex
	cases   map[string]*domain.InspectionCase
}

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
	repo := &SQLiteRepository{db: db, cases: make(map[string]*domain.InspectionCase)}
	if err := repo.CheckIntegrity(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Close() error { return r.db.Close() }

func (r *SQLiteRepository) cachedCase(id string) (*domain.InspectionCase, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	c, ok := r.cases[id]
	return c, ok
}

func (r *SQLiteRepository) rememberCase(c *domain.InspectionCase) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cases[c.ID] = c
}
