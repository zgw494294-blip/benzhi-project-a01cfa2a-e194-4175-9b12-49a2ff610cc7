package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type InspectionCase struct {
	ID                string                  `json:"id"`
	WorkpieceCode     string                  `json:"workpieceCode"`
	InspectionZone    string                  `json:"inspectionZone"`
	Technique         TechniqueParameters     `json:"techniqueParameters"`
	AcceptanceRuleSet AcceptanceRuleSet       `json:"acceptanceRuleSet"`
	Status            CaseStatus              `json:"status"`
	Version           int64                   `json:"version"`
	CreatedAt         time.Time               `json:"createdAt"`
	UpdatedAt         time.Time               `json:"updatedAt"`
	Revisions         []RadiographRevision    `json:"revisions"`
	Findings          []InterpretationFinding `json:"findings"`
	RuleConclusions   []RuleConclusion        `json:"ruleConclusions"`
	RetakeRequirement string                  `json:"retakeRequirement,omitempty"`
	CheckSequence     int                     `json:"checkSequence"`
	LastCheckPassed   bool                    `json:"lastCheckPassed"`
	LastCheckedAt     *time.Time              `json:"lastCheckedAt,omitempty"`
	Frozen            *FrozenSnapshot         `json:"frozen,omitempty"`
	Credential        *ReleaseCredential      `json:"credential,omitempty"`
	CheckBatches      []IntegrityCheckBatch   `json:"checkBatches,omitempty"`
	RetakeIssues      []RetakeIssue           `json:"retakeIssues,omitempty"`
	Coverage          []CandidateCoverage     `json:"coverage,omitempty"`
}

func NewInspectionCase(id, workpiece, zone string, technique TechniqueParameters, rules AcceptanceRuleSet, now time.Time) (*InspectionCase, error) {
	// Aggregates created by the pre-contract storage API may omit optional
	// threshold numbers; materialize conservative defaults before strict
	// validation while direct rule-set validation remains strict.
	for i := range rules.Rules {
		if technique.SourceDistanceMM == 0 && rules.Rules[i].MinVoltageKV == 0 && rules.Rules[i].MaxVoltageKV == 0 && rules.Rules[i].MaxDefectSizeMM == 0 {
			rules.Rules[i].MinVoltageKV, rules.Rules[i].MaxVoltageKV = 0.1, 10000
			rules.Rules[i].MaxDefectSizeMM = 0.1
		}
	}
	normalizedRules, ruleErr := rules.NormalizeAndValidate()
	c := &InspectionCase{ID: strings.TrimSpace(id), WorkpieceCode: strings.TrimSpace(workpiece), InspectionZone: strings.TrimSpace(zone), Technique: technique, AcceptanceRuleSet: normalizedRules, Status: StatusDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(), Revisions: []RadiographRevision{}, Findings: []InterpretationFinding{}, RuleConclusions: []RuleConclusion{}, CheckBatches: []IntegrityCheckBatch{}, RetakeIssues: []RetakeIssue{}}
	if ruleErr != nil {
		return nil, ruleErr
	}
	if err := c.ValidateBase(); err != nil {
		return nil, err
	}
	c.RefreshCoverage()
	return c, nil
}

func (c *InspectionCase) ValidateBase() error {
	if c.ID == "" || c.WorkpieceCode == "" || c.InspectionZone == "" {
		return Invalid("case", "任务标识、工件标识和检测区域不能为空")
	}
	if c.Technique.SourceType == "" || c.Technique.VoltageKV <= 0 || c.Technique.ExposureSeconds <= 0 {
		return Invalid("techniqueParameters", "射线源、管电压和曝光时间必须有效")
	}
	return c.AcceptanceRuleSet.Validate()
}

func (c *InspectionCase) CheckVersion(expected int64) error {
	if expected != c.Version {
		return fmt.Errorf("%w: expected=%d actual=%d", ErrConflict, expected, c.Version)
	}
	return nil
}

func (c *InspectionCase) Touch(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC()
}

func (c *InspectionCase) AddRevision(rev RadiographRevision, now time.Time) error {
	rev.ViewCode, rev.CoveredZone, rev.CaptureBatch = strings.TrimSpace(rev.ViewCode), strings.TrimSpace(rev.CoveredZone), strings.TrimSpace(rev.CaptureBatch)
	if !c.Status.Mutable() {
		return ErrInvalidState
	}
	if c.Status != StatusDraft && c.Status != StatusPendingCheck && c.Status != StatusRetake && c.Status != StatusReinspection {
		return ErrInvalidState
	}
	if err := rev.Validate(); err != nil {
		return err
	}
	for _, existing := range c.Revisions {
		if existing.ContentDigest == rev.ContentDigest {
			return ErrDuplicate
		}
		if existing.ID == rev.ID {
			return ErrDuplicate
		}
	}
	if rev.SupersedesRevisionID != "" {
		found := false
		for _, existing := range c.Revisions {
			if existing.ID == rev.SupersedesRevisionID {
				if existing.ViewCode != strings.TrimSpace(rev.ViewCode) || existing.CoveredZone != strings.TrimSpace(rev.CoveredZone) {
					return Invalid("supersedesRevisionId", "替代修订必须保持原视图和覆盖区域")
				}
				found = true
				break
			}
		}
		if !found {
			return Invalid("supersedesRevisionId", "被替代的修订不存在")
		}
		for _, existing := range c.Revisions {
			if existing.SupersedesRevisionID == rev.SupersedesRevisionID {
				return Invalid("supersedesRevisionId", "该修订已被替代，不能再次替代或形成分叉")
			}
		}
	}
	rev.CaseID = c.ID
	rev.RevisionNumber = len(c.Revisions) + 1
	c.Revisions = append(c.Revisions, rev)
	if c.Status == StatusRetake {
		c.Status = StatusReinspection
	} else {
		c.Status = StatusPendingCheck
	}
	c.LastCheckPassed = false
	c.Touch(now)
	c.RefreshCoverage()
	for i := range c.RetakeIssues {
		if c.RetakeIssues[i].OriginalRevisionID == rev.SupersedesRevisionID && c.RetakeIssues[i].ReplacementRevisionID == "" {
			c.RetakeIssues[i].ReplacementRevisionID = rev.ID
			t := now.UTC()
			c.RetakeIssues[i].SubmittedAt = &t
			c.RetakeIssues[i].Status = "待复验"
		}
	}
	return nil
}

func (c *InspectionCase) RefreshCoverage() {
	active := c.ActiveRevisions()
	coverage := []CandidateCoverage{}
	for _, rule := range c.AcceptanceRuleSet.Rules {
		views, zones := rule.RequiredViews, rule.RequiredZones
		if len(views) == 0 {
			views = []string{""}
		}
		if len(zones) == 0 {
			zones = []string{""}
		}
		for _, view := range views {
			for _, zone := range zones {
				item := CandidateCoverage{RuleID: rule.ID, ViewCode: view, CoveredZone: zone, Status: "missing"}
				for _, rev := range active {
					if rev.ViewCode == view && rev.CoveredZone == zone {
						item.Status, item.RevisionID, item.RevisionNumber = "covered", rev.ID, rev.RevisionNumber
						break
					}
				}
				coverage = append(coverage, item)
			}
		}
	}
	c.Coverage = coverage
}

func (c *InspectionCase) NormalizeOrdering() {
	sort.SliceStable(c.Findings, func(i, j int) bool {
		if c.Findings[i].RevisionID != c.Findings[j].RevisionID {
			return c.Findings[i].RevisionID < c.Findings[j].RevisionID
		}
		if c.Findings[i].Location != c.Findings[j].Location {
			return c.Findings[i].Location < c.Findings[j].Location
		}
		return c.Findings[i].ID < c.Findings[j].ID
	})
	sort.SliceStable(c.RuleConclusions, func(i, j int) bool { return c.RuleConclusions[i].RuleID < c.RuleConclusions[j].RuleID })
}

func (c *InspectionCase) ValidateRevisionCandidate(rev RadiographRevision) error {
	if err := rev.Validate(); err != nil {
		return err
	}
	if rev.SupersedesRevisionID == "" {
		return nil
	}
	target, ok := c.Revision(rev.SupersedesRevisionID)
	if !ok {
		return Invalid("supersedesRevisionId", "被替代的修订不存在")
	}
	for _, existing := range c.Revisions {
		if existing.SupersedesRevisionID == target.ID {
			return Invalid("supersedesRevisionId", "该修订已失效，不能再次替代")
		}
	}
	if target.ViewCode != strings.TrimSpace(rev.ViewCode) || target.CoveredZone != strings.TrimSpace(rev.CoveredZone) {
		return Invalid("supersedesRevisionId", "替代修订必须保持原视图和覆盖区域")
	}
	return nil
}

func (c *InspectionCase) ActiveRevisions() []RadiographRevision {
	superseded := make(map[string]bool)
	for _, rev := range c.Revisions {
		if rev.SupersedesRevisionID != "" {
			superseded[rev.SupersedesRevisionID] = true
		}
	}
	active := make([]RadiographRevision, 0, len(c.Revisions))
	for _, rev := range c.Revisions {
		if !superseded[rev.ID] {
			active = append(active, rev)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].RevisionNumber < active[j].RevisionNumber })
	return active
}

func (c *InspectionCase) Revision(id string) (*RadiographRevision, bool) {
	for i := range c.Revisions {
		if c.Revisions[i].ID == id {
			return &c.Revisions[i], true
		}
	}
	return nil, false
}

func (c *InspectionCase) OpenBlockingFindings() []InterpretationFinding {
	var result []InterpretationFinding
	for _, finding := range c.Findings {
		if finding.IsBlocking() {
			result = append(result, finding)
		}
	}
	return result
}
