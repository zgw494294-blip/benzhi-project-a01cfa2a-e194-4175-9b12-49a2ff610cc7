package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func (r *SQLiteRepository) FindCredential(ctx context.Context, number string) (*domain.InspectionCase, *domain.ReleaseCredential, error) {
	var caseID string
	var encoded []byte
	err := r.db.QueryRowContext(ctx, `SELECT case_id,data_json FROM credentials WHERE credential_number=?`, number).Scan(&caseID, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	var credential domain.ReleaseCredential
	if err := json.Unmarshal(encoded, &credential); err != nil {
		return nil, nil, err
	}
	c, err := r.Load(ctx, caseID)
	return c, &credential, err
}

func (r *SQLiteRepository) AuditTrail(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT data_json FROM audit_events WHERE case_id=? ORDER BY occurred_at,id`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.AuditEvent{}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		var item domain.AuditEvent
		if err := json.Unmarshal(encoded, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].At.Equal(items[j].At) {
			return items[i].ID < items[j].ID
		}
		return items[i].At.Before(items[j].At)
	})
	return items, rows.Err()
}
