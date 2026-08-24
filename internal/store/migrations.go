package store

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 1

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS cases (
		id TEXT PRIMARY KEY,
		version INTEGER NOT NULL,
		status TEXT NOT NULL,
		workpiece_code TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		aggregate_json BLOB NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS revisions (
		id TEXT PRIMARY KEY,
		case_id TEXT NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
		revision_number INTEGER NOT NULL,
		content_digest TEXT NOT NULL,
		storage_key TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		data_json BLOB NOT NULL,
		UNIQUE(case_id, content_digest)
	)`,
	`CREATE TABLE IF NOT EXISTS findings (
		id TEXT PRIMARY KEY,
		case_id TEXT NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
		revision_id TEXT,
		source TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT NOT NULL,
		data_json BLOB NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS frozen_snapshots (
		case_id TEXT PRIMARY KEY REFERENCES cases(id) ON DELETE CASCADE,
		frozen_version INTEGER NOT NULL,
		evidence_digest TEXT NOT NULL,
		data_json BLOB NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS credentials (
		credential_number TEXT PRIMARY KEY,
		case_id TEXT NOT NULL UNIQUE REFERENCES cases(id) ON DELETE CASCADE,
		verification_digest TEXT NOT NULL,
		data_json BLOB NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		case_id TEXT NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
		action TEXT NOT NULL,
		actor TEXT NOT NULL,
		role TEXT NOT NULL,
		case_version INTEGER NOT NULL,
		occurred_at TEXT NOT NULL,
		data_json BLOB NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS audit_case_order ON audit_events(case_id, occurred_at, id)`,
	`CREATE TABLE IF NOT EXISTS idempotency_results (
		idempotency_key TEXT NOT NULL,
		operation TEXT NOT NULL,
		result_json BLOB NOT NULL,
		created_at TEXT NOT NULL,
		PRIMARY KEY(idempotency_key, operation)
	)`,
}

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range schemaStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行数据库迁移: %w", err)
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_meta`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta(version) VALUES (?)`, schemaVersion); err != nil {
			return err
		}
	} else {
		var version int
		if err := tx.QueryRowContext(ctx, `SELECT version FROM schema_meta LIMIT 1`).Scan(&version); err != nil {
			return err
		}
		if version != schemaVersion {
			return fmt.Errorf("不支持的 schemaVersion: %d", version)
		}
	}
	return tx.Commit()
}
