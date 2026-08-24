package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func (r *SQLiteRepository) Create(ctx context.Context, c *domain.InspectionCase, event domain.AuditEvent, idempotencyKey string, result []byte) error {
	encoded, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO cases(id,version,status,workpiece_code,updated_at,aggregate_json) VALUES(?,?,?,?,?,?)`, c.ID, c.Version, c.Status, c.WorkpieceCode, c.UpdatedAt.Format(time.RFC3339Nano), encoded)
	if err != nil {
		if isConstraint(err) {
			return domain.ErrDuplicate
		}
		return err
	}
	if err := writeProjections(ctx, tx, c); err != nil {
		return err
	}
	if err := insertEventAndResult(ctx, tx, event, idempotencyKey, result); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.rememberCase(c)
	return nil
}

func (r *SQLiteRepository) Load(ctx context.Context, id string) (*domain.InspectionCase, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cached, ok := r.cachedCase(id); ok {
		return cached, nil
	}
	var encoded []byte
	err := r.db.QueryRowContext(ctx, `SELECT aggregate_json FROM cases WHERE id=?`, id).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var c domain.InspectionCase
	if err := json.Unmarshal(encoded, &c); err != nil {
		return nil, fmt.Errorf("解析任务聚合: %w", err)
	}
	r.rememberCase(&c)
	return &c, nil
}

func (r *SQLiteRepository) Save(ctx context.Context, c *domain.InspectionCase, previousVersion int64, event domain.AuditEvent, idempotencyKey string, result []byte) error {
	encoded, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	update, err := tx.ExecContext(ctx, `UPDATE cases SET version=?,status=?,workpiece_code=?,updated_at=?,aggregate_json=? WHERE id=? AND version=?`, c.Version, c.Status, c.WorkpieceCode, c.UpdatedAt.Format(time.RFC3339Nano), encoded, c.ID, previousVersion)
	if err != nil {
		return err
	}
	changed, err := update.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.ErrConflict
	}
	if err := writeProjections(ctx, tx, c); err != nil {
		return err
	}
	if err := insertEventAndResult(ctx, tx, event, idempotencyKey, result); err != nil {
		if isConstraint(err) {
			return domain.ErrConflict
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.rememberCase(c)
	return nil
}

func (r *SQLiteRepository) List(ctx context.Context) ([]domain.InspectionCase, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT aggregate_json FROM cases ORDER BY updated_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.InspectionCase{}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		var item domain.InspectionCase
		if err := json.Unmarshal(encoded, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) IdempotencyResult(ctx context.Context, key, operation string) ([]byte, bool, error) {
	var encoded []byte
	err := r.db.QueryRowContext(ctx, `SELECT result_json FROM idempotency_results WHERE idempotency_key=? AND operation=?`, key, operation).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return encoded, err == nil, err
}

func isConstraint(err error) bool {
	return err != nil && (contains(err.Error(), "constraint failed") || contains(err.Error(), "UNIQUE constraint"))
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
