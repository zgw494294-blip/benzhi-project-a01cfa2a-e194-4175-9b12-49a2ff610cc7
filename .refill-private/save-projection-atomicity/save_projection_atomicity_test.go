package saveprojectionatomicity_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

func TestFailedProjectionRollsBackAggregate(t *testing.T) {
	ctx := context.Background()
	repo, err := store.OpenSQLite(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	first := newCase(t, "case-first", now)
	addRevision(t, first, "revision-collision", now)
	createCase(t, repo, first, "event-first", "create-first-0001", now)

	second := newCase(t, "case-second", now)
	createCase(t, repo, second, "event-second", "create-second-0001", now)
	previousVersion := second.Version
	addRevision(t, second, "revision-collision", now.Add(time.Minute))
	result, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	event := domain.AuditEvent{
		ID: "event-save-second", CaseID: second.ID, Action: "revision.submitted",
		Actor: "操作员", Role: "operator", Version: second.Version, At: now.Add(time.Minute),
	}
	if err := repo.Save(ctx, second, previousVersion, event, "save-second-0001", result); err == nil {
		t.Fatal("重复投影主键应使保存失败")
	}

	loaded, err := repo.Load(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != previousVersion || len(loaded.Revisions) != 0 {
		t.Fatalf("投影写入失败后聚合仍被提交: version=%d revisions=%d", loaded.Version, len(loaded.Revisions))
	}
	if events, err := repo.AuditTrail(ctx, second.ID); err != nil || len(events) != 1 {
		t.Fatalf("失败保存不应新增审计事件: count=%d err=%v", len(events), err)
	}
}

func newCase(t *testing.T, id string, now time.Time) *domain.InspectionCase {
	t.Helper()
	c, err := domain.NewInspectionCase(id, "WP-ATOMIC", "ZONE-A", domain.TechniqueParameters{
		SourceType: "X-ray", VoltageKV: 180, ExposureSeconds: 2,
	}, domain.AcceptanceRuleSet{ID: "RULES", Version: 1, Rules: []domain.AcceptanceRule{{
		ID: "R1", Name: "焊缝规则", RequiredViews: []string{"FRONT"}, RequiredZones: []string{"ZONE-A"},
	}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func addRevision(t *testing.T, c *domain.InspectionCase, id string, now time.Time) {
	t.Helper()
	err := c.AddRevision(domain.RadiographRevision{
		ID: id, CaptureBatch: "BATCH-1", ViewCode: "FRONT", CoveredZone: "ZONE-A",
		ExposureParameters: domain.ExposureParameters{VoltageKV: 180, ExposureSeconds: 2},
		ContentDigest:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StorageKey:         "aa/payload", SizeBytes: 16, SubmittedAt: now,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
}

func createCase(t *testing.T, repo *store.SQLiteRepository, c *domain.InspectionCase, eventID, key string, now time.Time) {
	t.Helper()
	result, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	event := domain.AuditEvent{ID: eventID, CaseID: c.ID, Action: "case.created", Actor: "操作员", Role: "operator", Version: c.Version, At: now}
	if err := repo.Create(context.Background(), c, event, key, result); err != nil {
		t.Fatal(err)
	}
}
