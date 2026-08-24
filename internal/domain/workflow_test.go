package domain

import (
	"testing"
	"time"
)

func testCase(t *testing.T) *InspectionCase {
	t.Helper()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	c, err := NewInspectionCase("case-1", "WP-001", "WELD-A", TechniqueParameters{SourceType: "X-ray", VoltageKV: 180, CurrentMA: 5, ExposureSeconds: 2, SourceDistanceMM: 600}, AcceptanceRuleSet{ID: "RT-RULES", Version: 3, Rules: []AcceptanceRule{{ID: "R-1", Name: "覆盖和缺陷限制", RequiredViews: []string{"FRONT"}, RequiredZones: []string{"WELD-A"}, MinVoltageKV: 150, MaxVoltageKV: 220, MaxDefectSizeMM: 2}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func revision(id, digest string, now time.Time) RadiographRevision {
	return RadiographRevision{ID: id, CaptureBatch: "BATCH-1", ViewCode: "FRONT", CoveredZone: "WELD-A", ExposureParameters: ExposureParameters{VoltageKV: 180, CurrentMA: 5, ExposureSeconds: 2, SourceDistanceMM: 600}, ContentDigest: digest, StorageKey: digest[:2] + "/" + digest, SizeBytes: 128, SubmittedAt: now}
}

func TestHappyPathFreezesAndIssuesVerifiableCredential(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	c := testCase(t)
	if err := c.AddRevision(revision("rev-1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	findings, err := c.RunCompletenessCheck(func() string { return "automatic-id" }, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 || !c.LastCheckPassed || c.Status != StatusPendingReview {
		t.Fatalf("检查结果异常: %#v, %s", findings, c.Status)
	}
	if err := c.SetRuleConclusions([]RuleConclusion{{RuleID: "R-1", Conclusion: "pass", Basis: "视图与缺陷均符合规则"}}, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := c.Freeze("判读员甲", now.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Verify() || snapshot.FrozenVersion != c.Version {
		t.Fatalf("冻结快照无效: %#v", snapshot)
	}
	credential, err := c.IssueCredential("RT-20260825-001", "质量负责人乙", snapshot.FrozenVersion, now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !credential.Verify(snapshot) || c.Status != StatusReleased {
		t.Fatal("放行凭据未通过校验")
	}
	credential.Issuer = "被篡改的签发人"
	if credential.Verify(snapshot) {
		t.Fatal("被篡改的凭据不应通过校验")
	}
}

func TestMissingViewProducesDeterministicBlocker(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	c := testCase(t)
	bad := revision("rev-1", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now)
	bad.ViewCode = "SIDE"
	if err := c.AddRevision(bad, now); err != nil {
		t.Fatal(err)
	}
	sequence := 0
	findings, err := c.RunCompletenessCheck(func() string { sequence++; return "f-1" }, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].FindingType != "missing_view" || !findings[0].IsBlocking() {
		t.Fatalf("预期缺失视图阻断，得到 %#v", findings)
	}
	if c.Status != StatusPendingCheck || c.LastCheckPassed {
		t.Fatal("失败检查不应进入人工判读")
	}
	if _, err := c.Freeze("判读员", now); err != ErrInvalidState {
		t.Fatalf("检查失败后冻结应被拒绝，得到 %v", err)
	}
}
