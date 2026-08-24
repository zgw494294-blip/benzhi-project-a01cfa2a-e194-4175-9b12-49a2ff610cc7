package domain

import (
	"testing"
	"time"
)

func TestRetakePreservesHistoryAndAllowsReinspection(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	c := testCase(t)
	first := revision("rev-old", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", now)
	if err := c.AddRevision(first, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunCompletenessCheck(func() string { return "auto-unused" }, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	manual := InterpretationFinding{ID: "manual-1", RevisionID: first.ID, FindingType: "未熔合", Location: "焊缝 35mm", MeasuredSize: 3.2, Severity: "blocking", RuleReference: "R-1", Basis: "黑度突变且延伸连续", Disposition: "返拍确认"}
	if err := c.AddManualFinding(manual, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestRetake("调整角度后重拍 FRONT 视图", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	replacement := revision("rev-new", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", now.Add(4*time.Minute))
	replacement.SupersedesRevisionID = first.ID
	if err := c.AddRevision(replacement, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusReinspection || len(c.Revisions) != 2 || len(c.ActiveRevisions()) != 1 {
		t.Fatalf("替代链异常: status=%s revisions=%d active=%d", c.Status, len(c.Revisions), len(c.ActiveRevisions()))
	}
	if _, err := c.RunCompletenessCheck(func() string { return "auto-new" }, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.CloseFinding("manual-1", "判读员", "替代修订未再显示原缺陷", now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetRuleConclusions([]RuleConclusion{{RuleID: "R-1", Conclusion: "pass", Basis: "替代修订复验通过"}}, now.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := c.Freeze("判读员", now.Add(8*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Revisions) != 1 || snapshot.Revisions[0].ID != "rev-new" {
		t.Fatalf("冻结集合应只含活动修订: %#v", snapshot.Revisions)
	}
	if len(snapshot.Findings) != 1 || snapshot.Findings[0].Status != "closed" {
		t.Fatalf("历史缺陷未保留关闭状态: %#v", snapshot.Findings)
	}
}

func TestVersionAndDuplicateRevisionProtection(t *testing.T) {
	now := time.Now().UTC()
	c := testCase(t)
	if err := c.CheckVersion(c.Version + 1); err == nil {
		t.Fatal("错误版本未被拒绝")
	}
	rev := revision("rev-1", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", now)
	if err := c.AddRevision(rev, now); err != nil {
		t.Fatal(err)
	}
	duplicate := revision("rev-2", rev.ContentDigest, now)
	if err := c.AddRevision(duplicate, now); err != ErrDuplicate {
		t.Fatalf("重复摘要应被拒绝: %v", err)
	}
}
