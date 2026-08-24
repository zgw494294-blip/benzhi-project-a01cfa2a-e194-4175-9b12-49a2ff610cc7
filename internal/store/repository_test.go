package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func TestSQLiteRepositoryPersistsAggregateAuditAndIdempotency(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := OpenSQLite(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	c, err := domain.NewInspectionCase("case-store", "WP-DB", "ZONE-1", domain.TechniqueParameters{SourceType: "X-ray", VoltageKV: 180, ExposureSeconds: 2}, domain.AcceptanceRuleSet{ID: "RULES", Version: 1, Rules: []domain.AcceptanceRule{{ID: "R1", Name: "规则"}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(c)
	event := domain.AuditEvent{ID: "event-1", CaseID: c.ID, Action: "case.created", Actor: "操作员", Role: "operator", Version: c.Version, At: now}
	if err := repo.Create(ctx, c, event, "store-idempotency-0001", encoded); err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenSQLite(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.Load(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.WorkpieceCode != "WP-DB" || loaded.Version != 1 {
		t.Fatalf("恢复聚合异常: %#v", loaded)
	}
	result, ok, err := reopened.IdempotencyResult(ctx, "store-idempotency-0001", "create_case")
	if err != nil || !ok || len(result) == 0 {
		t.Fatalf("幂等结果丢失: ok=%v err=%v", ok, err)
	}
	audit, err := reopened.AuditTrail(ctx, c.ID)
	if err != nil || len(audit) != 1 || audit[0].Action != "case.created" {
		t.Fatalf("审计事件异常: %#v %v", audit, err)
	}
	loaded.Touch(now.Add(time.Minute))
	encoded, _ = json.Marshal(loaded)
	event2 := domain.AuditEvent{ID: "event-2", CaseID: loaded.ID, Action: "check.completed", Actor: "判读员", Role: "reviewer", Version: loaded.Version, At: now.Add(time.Minute)}
	if err := reopened.Save(ctx, loaded, 99, event2, "store-idempotency-0002", encoded); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("错误前置版本应冲突: %v", err)
	}
}
