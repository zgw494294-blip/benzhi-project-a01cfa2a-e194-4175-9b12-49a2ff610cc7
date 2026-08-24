package domain

import (
	"strings"
	"time"
)

func (c *InspectionCase) AddManualFinding(f InterpretationFinding, now time.Time) error {
	return c.AddManualFindings([]InterpretationFinding{f}, nil, now)
}

func (c *InspectionCase) AddManualFindings(findings []InterpretationFinding, conclusions []RuleConclusion, now time.Time) error {
	if c.Status != StatusPendingReview {
		return ErrInvalidState
	}
	validated := make([]InterpretationFinding, 0, len(findings))
	seen := map[string]bool{}
	for i, f := range findings {
		if _, ok := c.Revision(f.RevisionID); !ok {
			return Invalid("findings["+decimalIndex(i)+"].revisionId", "关联底片修订不存在或不是当前候选修订")
		}
		active := false
		for _, rev := range c.ActiveRevisions() {
			if rev.ID == f.RevisionID {
				active = true
				break
			}
		}
		if !active {
			return Invalid("findings["+decimalIndex(i)+"].revisionId", "只能引用当前有效底片修订")
		}
		if !c.AcceptanceRuleSet.HasRule(f.RuleReference) {
			return Invalid("findings["+decimalIndex(i)+"].ruleReference", "关联验收规则不存在")
		}
		f.CaseID, f.Source, f.Status, f.CreatedAt = c.ID, SourceManual, FindingOpen, now.UTC()
		if err := f.ValidateManual(); err != nil {
			return err
		}
		if rule := c.rule(f.RuleReference); rule != nil && f.MeasuredSize > rule.MaxDefectSizeMM {
			f.Severity = "blocking"
		}
		key := f.RevisionID + "|" + f.Location + "|" + f.FindingType + "|" + f.RuleReference
		if seen[key] {
			return Invalid("findings["+decimalIndex(i)+"]", "同一批次中不能重复提交相同修订、位置、类型和规则的缺陷")
		}
		seen[key] = true
		for _, existing := range c.Findings {
			if existing.Status == FindingOpen && existing.RevisionID+"|"+existing.Location+"|"+existing.FindingType+"|"+existing.RuleReference == key {
				return ErrDuplicate
			}
		}
		validated = append(validated, f)
	}
	if len(conclusions) > 0 {
		for _, conclusion := range conclusions {
			if conclusion.Conclusion == "pass" {
				for _, f := range validated {
					if f.RuleReference == conclusion.RuleID && f.Severity == "blocking" {
						return Invalid("conclusions", "存在超过规则阈值的阻断缺陷时，该规则不能判为通过")
					}
				}
			}
		}
	}
	c.Findings = append(c.Findings, validated...)
	c.Touch(now)
	return nil
}

func (c *InspectionCase) rule(id string) *AcceptanceRule {
	for i := range c.AcceptanceRuleSet.Rules {
		if c.AcceptanceRuleSet.Rules[i].ID == id {
			return &c.AcceptanceRuleSet.Rules[i]
		}
	}
	return nil
}

func (c *InspectionCase) SetRuleConclusions(conclusions []RuleConclusion, now time.Time) error {
	if c.Status != StatusPendingReview {
		return ErrInvalidState
	}
	seen := make(map[string]bool)
	for _, conclusion := range conclusions {
		if !c.AcceptanceRuleSet.HasRule(conclusion.RuleID) || seen[conclusion.RuleID] {
			return Invalid("ruleConclusions", "规则结论存在未知或重复规则")
		}
		if conclusion.Conclusion != "pass" && conclusion.Conclusion != "fail" {
			return Invalid("ruleConclusions", "规则结论必须为 pass 或 fail")
		}
		if strings.TrimSpace(conclusion.Basis) == "" {
			return Invalid("ruleConclusions", "规则结论依据不能为空")
		}
		if conclusion.Conclusion == "pass" {
			for _, finding := range c.Findings {
				if finding.Status == FindingOpen && finding.RuleReference == conclusion.RuleID {
					if rule := c.rule(conclusion.RuleID); rule != nil && finding.MeasuredSize > rule.MaxDefectSizeMM {
						return Invalid("ruleConclusions", "存在超过规则阈值的阻断缺陷时，该规则不能判为通过")
					}
				}
			}
		}
		seen[conclusion.RuleID] = true
	}
	if len(seen) != len(c.AcceptanceRuleSet.Rules) {
		return Invalid("ruleConclusions", "必须为每条验收规则给出结论")
	}
	c.RuleConclusions = append([]RuleConclusion(nil), conclusions...)
	c.Touch(now)
	return nil
}

func (c *InspectionCase) RequestRetake(requirement string, now time.Time) error {
	items := make([]RetakeIssue, 0)
	for _, finding := range c.OpenBlockingFindings() {
		items = append(items, RetakeIssue{ID: finding.ID + "-retake", FindingID: finding.ID, Requirement: requirement, OriginalRevisionID: finding.RevisionID, Status: "待替代"})
	}
	return c.RequestRetakeWithItems(requirement, items, now)
}

func (c *InspectionCase) RequestRetakeWithItems(requirement string, items []RetakeIssue, now time.Time) error {
	if c.Status != StatusPendingReview && c.Status != StatusPendingCheck {
		return ErrInvalidState
	}
	if strings.TrimSpace(requirement) == "" {
		return Invalid("requirement", "返拍要求不能为空")
	}
	if len(c.OpenBlockingFindings()) == 0 {
		return Invalid("findings", "没有阻断问题时不能发起返拍")
	}
	if len(items) == 0 {
		return Invalid("retakeItems", "至少选择一项未关闭阻断问题")
	}
	open := map[string]bool{}
	for _, f := range c.OpenBlockingFindings() {
		open[f.ID] = true
	}
	seen := map[string]bool{}
	for i := range items {
		item := &items[i]
		if !open[item.FindingID] {
			return Invalid("retakeItems["+decimalIndex(i)+"].findingId", "只能选择当前未关闭阻断问题")
		}
		if seen[item.FindingID] {
			return Invalid("retakeItems["+decimalIndex(i)+"]", "返拍问题不能重复")
		}
		seen[item.FindingID] = true
		if item.OriginalRevisionID == "" {
			for _, f := range c.Findings {
				if f.ID == item.FindingID {
					item.OriginalRevisionID = f.RevisionID
				}
			}
		}
		if rev, ok := c.Revision(item.OriginalRevisionID); ok {
			if item.TargetView == "" {
				item.TargetView = rev.ViewCode
			}
			if item.TargetZone == "" {
				item.TargetZone = rev.CoveredZone
			}
		}
		if strings.TrimSpace(item.TargetView) == "" || strings.TrimSpace(item.TargetZone) == "" {
			return Invalid("retakeItems["+decimalIndex(i)+"]", "返拍项目必须指定目标视图和区域")
		}
		if item.Requirement == "" {
			item.Requirement = requirement
		}
		if item.Status == "" {
			item.Status = "待替代"
		}
	}
	c.Status = StatusRetake
	c.RetakeRequirement = strings.TrimSpace(requirement)
	c.RetakeIssues = append(c.RetakeIssues, items...)
	c.Touch(now)
	return nil
}

func (c *InspectionCase) CloseFindings(ids []string, actor, basis string, now time.Time) error {
	if strings.TrimSpace(basis) == "" {
		return Invalid("verificationBasis", "复验依据不能为空")
	}
	if len(ids) == 0 {
		return Invalid("findingIds", "至少选择一个问题")
	}
	problems := &ValidationErrors{}
	for _, id := range ids {
		var issue *RetakeIssue
		for i := range c.RetakeIssues {
			if c.RetakeIssues[i].FindingID == id {
				issue = &c.RetakeIssues[i]
				break
			}
		}
		if issue == nil {
			problems.Add("findingIds."+id, "问题未建立返拍闭环")
			continue
		}
		if issue.ReplacementRevisionID == "" {
			problems.Add("findingIds."+id, "问题尚未提交有效替代修订")
		}
		if !c.LastCheckPassed {
			problems.Add("findingIds."+id, "最新完整性检查尚未通过")
		}
	}
	if !problems.Empty() {
		return problems
	}
	for _, id := range ids {
		_ = c.CloseFinding(id, actor, basis, now)
		for i := range c.RetakeIssues {
			if c.RetakeIssues[i].FindingID == id {
				c.RetakeIssues[i].Status = "已关闭"
				c.RetakeIssues[i].VerificationBasis = basis
				c.RetakeIssues[i].LatestCheckSequence = c.CheckSequence
			}
		}
	}
	return nil
}

func (c *InspectionCase) CloseFinding(id, actor, note string, now time.Time) error {
	if c.Status != StatusPendingReview && c.Status != StatusReinspection && c.Status != StatusRetake {
		return ErrInvalidState
	}
	if strings.TrimSpace(note) == "" {
		return Invalid("closureNote", "关闭说明不能为空")
	}
	for i := range c.RetakeIssues {
		if c.RetakeIssues[i].FindingID == id {
			if c.RetakeIssues[i].ReplacementRevisionID == "" {
				return Invalid("findingId", "返拍问题尚未提交有效替代修订")
			}
			if !c.LastCheckPassed {
				return Invalid("findingId", "最新完整性检查尚未通过")
			}
		}
	}
	for i := range c.Findings {
		if c.Findings[i].ID == id {
			if c.Findings[i].Status == FindingClosed {
				return nil
			}
			closed := now.UTC()
			c.Findings[i].Status = FindingClosed
			c.Findings[i].ClosedAt = &closed
			c.Findings[i].ClosedBy = actor
			c.Findings[i].ClosureNote = note
			for j := range c.RetakeIssues {
				if c.RetakeIssues[j].FindingID == id {
					c.RetakeIssues[j].Status = "已关闭"
					c.RetakeIssues[j].VerificationBasis = note
					c.RetakeIssues[j].LatestCheckSequence = c.CheckSequence
				}
			}
			c.Touch(now)
			return nil
		}
	}
	return ErrNotFound
}
