package failedsavecacheisolation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

func TestFailedSaveDoesNotPolluteCaseCache(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repo, err := store.OpenSQLite(ctx, dataDir)
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	original := &domain.InspectionCase{
		ID:                "case-cache-isolation",
		WorkpieceCode:     "WP-ORIGINAL",
		Status:            domain.StatusDraft,
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
		AcceptanceRuleSet: domain.AcceptanceRuleSet{ID: "rules-1", Version: 1, Rules: []domain.AcceptanceRule{{ID: "rule-1", RequiredViews: []string{"FRONT"}}}},
		Revisions:         []domain.RadiographRevision{{ID: "rev-1", CaseID: "case-cache-isolation", RevisionNumber: 1, ViewCode: "FRONT", ContentDigest: "digest-original", StorageKey: "key-original", SizeBytes: 1}},
		Findings:          []domain.InterpretationFinding{{ID: "finding-1", CaseID: "case-cache-isolation", RevisionID: "rev-1", Source: domain.SourceManual, FindingType: "porosity", Location: "L-ORIGINAL", Severity: "minor", Status: domain.FindingOpen, CreatedAt: now}},
		CheckBatches:      []domain.IntegrityCheckBatch{{Sequence: 1, RevisionDigests: map[string]string{"rev-1": "digest-original"}, GeneratedAt: now}},
		RetakeIssues:      []domain.RetakeIssue{{ID: "retake-1", FindingID: "finding-1", Requirement: "REQ-ORIGINAL", OriginalRevisionID: "rev-1", Status: "待替代"}},
	}
	created := domain.AuditEvent{ID: "evt-created", CaseID: original.ID, Action: "case.created", Actor: "operator", Role: "operator", Version: 1, At: now}
	if err := repo.Create(ctx, original, created, "create-cache-isolation", []byte(`{"id":"case-cache-isolation"}`)); err != nil {
		t.Fatalf("创建任务: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("关闭初始仓储: %v", err)
	}

	repo, err = store.OpenSQLite(ctx, dataDir)
	if err != nil {
		t.Fatalf("重新打开仓储: %v", err)
	}
	loaded, err := repo.Load(ctx, original.ID)
	if err != nil {
		t.Fatalf("加载任务: %v", err)
	}
	observer, err := repo.Load(ctx, original.ID)
	if err != nil {
		t.Fatalf("加载并行读取快照: %v", err)
	}
	loaded.WorkpieceCode = "WP-UNCOMMITTED"
	loaded.Version++
	loaded.UpdatedAt = now.Add(time.Minute)
	loaded.AcceptanceRuleSet.Rules[0].RequiredViews[0] = "SIDE"
	loaded.Revisions[0].ViewCode = "SIDE"
	loaded.Findings[0].Location = "L-UNCOMMITTED"
	loaded.CheckBatches[0].RevisionDigests["rev-1"] = "digest-uncommitted"
	loaded.RetakeIssues[0].Requirement = "REQ-UNCOMMITTED"
	failed := domain.AuditEvent{ID: "evt-failed", CaseID: loaded.ID, Action: "review.conclusions_set", Actor: "reviewer", Role: "reviewer", Version: loaded.Version, At: loaded.UpdatedAt}
	if err := repo.Save(ctx, loaded, 99, failed, "failed-cache-isolation", []byte(`{}`)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("预期乐观写冲突，实际为 %v", err)
	}

	cached, err := repo.Load(ctx, original.ID)
	if err != nil {
		t.Fatalf("冲突后读取缓存: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("关闭缓存仓储: %v", err)
	}

	durableRepo, err := store.OpenSQLite(ctx, dataDir)
	if err != nil {
		t.Fatalf("打开持久化校验仓储: %v", err)
	}
	defer durableRepo.Close()
	durable, err := durableRepo.Load(ctx, original.ID)
	if err != nil {
		t.Fatalf("读取持久化任务: %v", err)
	}
	durableState := caseState(durable)
	if durableState != "WP-ORIGINAL|1|FRONT|FRONT|L-ORIGINAL|digest-original|REQ-ORIGINAL" {
		t.Fatalf("失败写不应进入数据库，得到 %s", durableState)
	}
	if observerState := caseState(observer); observerState != durableState {
		t.Fatalf("失败写污染了先前读取者：读取者 %s，持久化 %s", observerState, durableState)
	}
	if cachedState := caseState(cached); cachedState != durableState {
		t.Fatalf("失败写污染了进程缓存：缓存 %s，持久化 %s", cachedState, durableState)
	}
}

func caseState(c *domain.InspectionCase) string {
	return fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s", c.WorkpieceCode, c.Version, c.AcceptanceRuleSet.Rules[0].RequiredViews[0], c.Revisions[0].ViewCode, c.Findings[0].Location, c.CheckBatches[0].RevisionDigests["rev-1"], c.RetakeIssues[0].Requirement)
}
