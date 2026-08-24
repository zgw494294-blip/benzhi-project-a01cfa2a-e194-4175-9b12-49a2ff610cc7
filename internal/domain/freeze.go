package domain

import (
	"sort"
	"time"
)

type FrozenFinding struct {
	ID            string  `json:"id"`
	RevisionID    string  `json:"revisionId,omitempty"`
	FindingType   string  `json:"findingType"`
	Location      string  `json:"location"`
	MeasuredSize  float64 `json:"measuredSize"`
	Severity      string  `json:"severity"`
	RuleReference string  `json:"ruleReference"`
	Disposition   string  `json:"disposition"`
	Status        string  `json:"status"`
}

type FrozenRevision struct {
	ID             string `json:"id"`
	RevisionNumber int    `json:"revisionNumber"`
	ViewCode       string `json:"viewCode"`
	CoveredZone    string `json:"coveredZone"`
	ContentDigest  string `json:"contentDigest"`
	StorageKey     string `json:"storageKey,omitempty"`
	SizeBytes      int64  `json:"sizeBytes,omitempty"`
}

type FrozenSnapshot struct {
	CaseID          string           `json:"caseId"`
	FrozenVersion   int64            `json:"frozenVersion"`
	RuleSetID       string           `json:"ruleSetId"`
	RuleSetVersion  int              `json:"ruleSetVersion"`
	Revisions       []FrozenRevision `json:"revisions"`
	Findings        []FrozenFinding  `json:"findings"`
	RuleConclusions []RuleConclusion `json:"ruleConclusions"`
	FrozenBy        string           `json:"frozenBy"`
	FrozenAt        time.Time        `json:"frozenAt"`
	EvidenceDigest  string           `json:"evidenceDigest"`
}

type snapshotMaterial struct {
	CaseID          string           `json:"caseId"`
	FrozenVersion   int64            `json:"frozenVersion"`
	RuleSetID       string           `json:"ruleSetId"`
	RuleSetVersion  int              `json:"ruleSetVersion"`
	Revisions       []FrozenRevision `json:"revisions"`
	Findings        []FrozenFinding  `json:"findings"`
	RuleConclusions []RuleConclusion `json:"ruleConclusions"`
}

func (s FrozenSnapshot) material() snapshotMaterial {
	return snapshotMaterial{CaseID: s.CaseID, FrozenVersion: s.FrozenVersion, RuleSetID: s.RuleSetID, RuleSetVersion: s.RuleSetVersion, Revisions: s.Revisions, Findings: s.Findings, RuleConclusions: s.RuleConclusions}
}

func (s FrozenSnapshot) Verify() bool { return VerifyDigest(s.material(), s.EvidenceDigest) }

func (c *InspectionCase) Freeze(actor string, now time.Time) (*FrozenSnapshot, error) {
	if c.Status != StatusPendingReview || !c.LastCheckPassed {
		return nil, ErrInvalidState
	}
	if len(c.OpenBlockingFindings()) > 0 {
		return nil, ErrOpenBlocker
	}
	if len(c.RuleConclusions) != len(c.AcceptanceRuleSet.Rules) {
		return nil, Invalid("ruleConclusions", "必须完成全部规则结论后才能冻结")
	}
	for _, conclusion := range c.RuleConclusions {
		if conclusion.Conclusion != "pass" {
			return nil, ErrOpenBlocker
		}
	}
	snapshot := FrozenSnapshot{CaseID: c.ID, FrozenVersion: c.Version + 1, RuleSetID: c.AcceptanceRuleSet.ID, RuleSetVersion: c.AcceptanceRuleSet.Version, FrozenBy: actor, FrozenAt: now.UTC()}
	for _, rev := range c.ActiveRevisions() {
		snapshot.Revisions = append(snapshot.Revisions, FrozenRevision{ID: rev.ID, RevisionNumber: rev.RevisionNumber, ViewCode: rev.ViewCode, CoveredZone: rev.CoveredZone, ContentDigest: rev.ContentDigest, StorageKey: rev.StorageKey, SizeBytes: rev.SizeBytes})
	}
	for _, f := range c.Findings {
		snapshot.Findings = append(snapshot.Findings, FrozenFinding{ID: f.ID, RevisionID: f.RevisionID, FindingType: f.FindingType, Location: f.Location, MeasuredSize: f.MeasuredSize, Severity: f.Severity, RuleReference: f.RuleReference, Disposition: f.Disposition, Status: string(f.Status)})
	}
	snapshot.RuleConclusions = append([]RuleConclusion(nil), c.RuleConclusions...)
	sort.Slice(snapshot.Findings, func(i, j int) bool { return snapshot.Findings[i].ID < snapshot.Findings[j].ID })
	sort.Slice(snapshot.RuleConclusions, func(i, j int) bool { return snapshot.RuleConclusions[i].RuleID < snapshot.RuleConclusions[j].RuleID })
	digest, err := StableDigest(snapshot.material())
	if err != nil {
		return nil, err
	}
	snapshot.EvidenceDigest = digest
	c.Frozen = &snapshot
	c.Status = StatusFrozen
	c.Touch(now)
	return &snapshot, nil
}
