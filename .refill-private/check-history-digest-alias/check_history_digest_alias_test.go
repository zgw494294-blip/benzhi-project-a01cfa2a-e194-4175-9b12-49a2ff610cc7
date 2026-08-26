package check_history_digest_alias_test

import (
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func TestReinspectionKeepsCheckDigestSnapshotsIsolated(t *testing.T) {
	now := time.Date(2026, 8, 25, 13, 0, 0, 0, time.UTC)
	rules := domain.AcceptanceRuleSet{
		ID:      "RT-RULES",
		Version: 1,
		Rules: []domain.AcceptanceRule{{
			ID:              "R-1",
			Name:            "正视图覆盖规则",
			RequiredViews:   []string{"FRONT"},
			RequiredZones:   []string{"WELD-A"},
			MinVoltageKV:    150,
			MaxVoltageKV:    220,
			MaxDefectSizeMM: 2,
		}},
	}
	inspection, err := domain.NewInspectionCase(
		"case-history",
		"WP-HISTORY",
		"WELD-A",
		domain.TechniqueParameters{SourceType: "X-ray", VoltageKV: 180, ExposureSeconds: 2},
		rules,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	oldRevision := revision("rev-old", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now)
	if err := inspection.AddRevision(oldRevision, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := inspection.RunCompletenessCheck(func() string { return "check-old" }, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := inspection.AddManualFinding(domain.InterpretationFinding{
		ID:            "finding-retake",
		RevisionID:    oldRevision.ID,
		FindingType:   "未熔合",
		Location:      "焊缝 35mm",
		MeasuredSize:  3.2,
		Severity:      "blocking",
		RuleReference: "R-1",
		Basis:         "黑度突变且延伸连续",
		Disposition:   "返拍确认",
	}, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := inspection.RequestRetake("调整角度后重拍 FRONT 视图", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}

	newRevision := revision("rev-new", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now.Add(5*time.Minute))
	newRevision.SupersedesRevisionID = oldRevision.ID
	if err := inspection.AddRevision(newRevision, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := inspection.RunCompletenessCheck(func() string { return "check-new" }, now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if len(inspection.CheckBatches) != 2 {
		t.Fatalf("预期保留两次检查，实际为 %d", len(inspection.CheckBatches))
	}
	first := inspection.CheckBatches[0].RevisionDigests
	second := inspection.CheckBatches[1].RevisionDigests
	if len(first) != 1 || first[oldRevision.ID] != oldRevision.ContentDigest {
		t.Fatalf("首次检查的摘要快照被后续复验污染: %#v", first)
	}
	if _, exists := first[newRevision.ID]; exists {
		t.Fatalf("首次检查不应包含后续替代修订 %q: %#v", newRevision.ID, first)
	}
	if len(second) != 1 || second[newRevision.ID] != newRevision.ContentDigest {
		t.Fatalf("复验摘要快照应只包含当前替代修订: %#v", second)
	}
}

func revision(id, digest string, submittedAt time.Time) domain.RadiographRevision {
	return domain.RadiographRevision{
		ID:           id,
		CaptureBatch: "BATCH-HISTORY",
		ViewCode:     "FRONT",
		CoveredZone:  "WELD-A",
		ExposureParameters: domain.ExposureParameters{
			VoltageKV:       180,
			ExposureSeconds: 2,
		},
		ContentDigest: digest,
		StorageKey:    digest[:2] + "/" + digest,
		SizeBytes:     128,
		SubmittedAt:   submittedAt,
	}
}
