package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

func (c *InspectionCase) RunCompletenessCheck(idFactory func() string, now time.Time) ([]InterpretationFinding, error) {
	return c.RunCompletenessCheckWithEvidence(idFactory, now, nil)
}

func (c *InspectionCase) RunCompletenessCheckWithEvidence(idFactory func() string, now time.Time, evidence func(RadiographRevision) error) ([]InterpretationFinding, error) {
	if c.Status != StatusPendingCheck && c.Status != StatusReinspection && c.Status != StatusPendingReview {
		return nil, ErrInvalidState
	}
	active := c.ActiveRevisions()
	c.CheckSequence++
	sequence := c.CheckSequence
	var generated []InterpretationFinding
	problems := make([]CheckProblem, 0)
	add := func(revisionID, typ, location, severity, rule, basis string) {
		generated = append(generated, InterpretationFinding{ID: idFactory(), CaseID: c.ID, RevisionID: revisionID, Source: SourceAutomatic, FindingType: typ, Location: location, Severity: severity, RuleReference: rule, Basis: basis, Disposition: "补充或替换底片后重新检查", Status: FindingOpen, CreatedAt: now.UTC()})
		p := CheckProblem{Type: typ, RuleID: rule, RevisionID: revisionID, Message: basis, Severity: severity}
		if revisionID != "" {
			if rev, ok := c.Revision(revisionID); ok {
				p.ViewCode, p.CoveredZone = rev.ViewCode, rev.CoveredZone
			}
		} else {
			if typ == "missing_zone" {
				p.CoveredZone = location
			} else {
				p.ViewCode = location
			}
		}
		p.Key = problemKey(p)
		problems = append(problems, p)
	}
	for _, rule := range c.AcceptanceRuleSet.Rules {
		for _, requiredView := range rule.RequiredViews {
			found := false
			for _, rev := range active {
				if rev.ViewCode == requiredView {
					found = true
					break
				}
			}
			if !found {
				add("", "missing_view", requiredView, "blocking", rule.ID, "缺少必需视图 "+requiredView)
			}
		}
		for _, zone := range rule.RequiredZones {
			found := false
			for _, rev := range active {
				if rev.CoveredZone == zone {
					found = true
					break
				}
			}
			if !found {
				add("", "missing_zone", zone, "blocking", rule.ID, "检测区域未被候选底片覆盖")
			}
		}
		for _, rev := range active {
			if rule.MinVoltageKV > 0 && rev.ExposureParameters.VoltageKV < rule.MinVoltageKV {
				add(rev.ID, "exposure_out_of_range", rev.ViewCode, "blocking", rule.ID, fmt.Sprintf("管电压 %.1fkV 低于下限 %.1fkV", rev.ExposureParameters.VoltageKV, rule.MinVoltageKV))
			}
			if rule.MaxVoltageKV > 0 && rev.ExposureParameters.VoltageKV > rule.MaxVoltageKV {
				add(rev.ID, "exposure_out_of_range", rev.ViewCode, "blocking", rule.ID, fmt.Sprintf("管电压 %.1fkV 高于上限 %.1fkV", rev.ExposureParameters.VoltageKV, rule.MaxVoltageKV))
			}
			if rev.ContentDigest == "" || rev.StorageKey == "" || rev.SizeBytes <= 0 {
				add(rev.ID, "metadata_missing", rev.ViewCode, "blocking", rule.ID, "底片摘要、存储键或载荷大小缺失")
			}
			if evidence != nil {
				if err := evidence(rev); err != nil {
					add(rev.ID, "payload_integrity", rev.ViewCode, "blocking", rule.ID, "底片载荷完整性校验失败")
				}
			}
		}
	}
	for i := range c.Findings {
		if c.Findings[i].Source == SourceAutomatic && c.Findings[i].Status == FindingOpen {
			closed := now.UTC()
			c.Findings[i].Status = FindingClosed
			c.Findings[i].ClosedAt = &closed
			c.Findings[i].ClosedBy = "system"
			c.Findings[i].ClosureNote = fmt.Sprintf("第 %d 次完整性检查已替代此结果", sequence)
		}
	}
	sort.Slice(generated, func(i, j int) bool {
		if generated[i].FindingType != generated[j].FindingType {
			return generated[i].FindingType < generated[j].FindingType
		}
		if generated[i].Location != generated[j].Location {
			return generated[i].Location < generated[j].Location
		}
		return generated[i].RevisionID < generated[j].RevisionID
	})
	sort.Slice(problems, func(i, j int) bool { return problems[i].Key < problems[j].Key })
	c.Findings = append(c.Findings, generated...)
	c.LastCheckPassed = len(problems) == 0
	checked := now.UTC()
	c.LastCheckedAt = &checked
	if c.LastCheckPassed {
		c.Status = StatusPendingReview
	}
	previous := map[string]CheckProblem{}
	if n := len(c.CheckBatches); n > 0 {
		for _, p := range c.CheckBatches[n-1].Problems {
			previous[p.Key] = p
		}
	}
	current := map[string]CheckProblem{}
	differences := make([]CheckDifference, 0, len(previous)+len(problems))
	for _, p := range problems {
		current[p.Key] = p
		state := "持续"
		if _, ok := previous[p.Key]; !ok {
			state = "新增"
		}
		differences = append(differences, CheckDifference{Key: p.Key, State: state, Problem: p})
	}
	for key, p := range previous {
		if _, ok := current[key]; !ok {
			differences = append(differences, CheckDifference{Key: key, State: "已解决", Problem: p})
		}
	}
	sort.Slice(differences, func(i, j int) bool {
		if differences[i].Key == differences[j].Key {
			return differences[i].State < differences[j].State
		}
		return differences[i].Key < differences[j].Key
	})
	digests := make(map[string]string, len(active))
	for _, rev := range active {
		digests[rev.ID] = rev.ContentDigest
	}
	c.CheckBatches = append(c.CheckBatches, IntegrityCheckBatch{Sequence: sequence, RevisionDigests: digests, RuleSetVersion: c.AcceptanceRuleSet.Version, GeneratedAt: now.UTC(), Passed: c.LastCheckPassed, Problems: problems, Differences: differences})
	for i := range c.RetakeIssues {
		if c.RetakeIssues[i].ReplacementRevisionID != "" {
			c.RetakeIssues[i].LatestCheckSequence = sequence
			if c.LastCheckPassed {
				c.RetakeIssues[i].Status = "待复验关闭"
			}
		}
	}
	c.Touch(now)
	return generated, nil
}

func problemKey(p CheckProblem) string {
	sum := sha256.Sum256([]byte(p.Type + "|" + p.RuleID + "|" + p.ViewCode + "|" + p.CoveredZone + "|" + p.RevisionID))
	return hex.EncodeToString(sum[:])
}
