package domain

import "time"

type TechniqueParameters struct {
	SourceType       string  `json:"sourceType"`
	VoltageKV        float64 `json:"voltageKV"`
	CurrentMA        float64 `json:"currentMA"`
	ExposureSeconds  float64 `json:"exposureSeconds"`
	SourceDistanceMM float64 `json:"sourceDistanceMM"`
}

type ExposureParameters struct {
	VoltageKV        float64 `json:"voltageKV"`
	CurrentMA        float64 `json:"currentMA"`
	ExposureSeconds  float64 `json:"exposureSeconds"`
	SourceDistanceMM float64 `json:"sourceDistanceMM"`
}

type AcceptanceRule struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RequiredViews   []string `json:"requiredViews"`
	RequiredZones   []string `json:"requiredZones"`
	MinVoltageKV    float64  `json:"minVoltageKV"`
	MaxVoltageKV    float64  `json:"maxVoltageKV"`
	MaxDefectSizeMM float64  `json:"maxDefectSizeMM"`
	BlockingLevels  []string `json:"blockingLevels"`
}

type AcceptanceRuleSet struct {
	ID      string           `json:"id"`
	Version int              `json:"version"`
	Rules   []AcceptanceRule `json:"rules"`
}

type AuditEvent struct {
	ID      string         `json:"id"`
	CaseID  string         `json:"caseId"`
	Action  string         `json:"action"`
	Actor   string         `json:"actor"`
	Role    string         `json:"role"`
	Version int64          `json:"version"`
	At      time.Time      `json:"at"`
	Details map[string]any `json:"details,omitempty"`
}

type RuleConclusion struct {
	RuleID     string `json:"ruleId"`
	Conclusion string `json:"conclusion"`
	Basis      string `json:"basis"`
}

type CheckProblem struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	RuleID      string `json:"ruleId,omitempty"`
	RevisionID  string `json:"revisionId,omitempty"`
	ViewCode    string `json:"viewCode,omitempty"`
	CoveredZone string `json:"coveredZone,omitempty"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
}
type CheckDifference struct {
	Key     string       `json:"key"`
	State   string       `json:"state"`
	Problem CheckProblem `json:"problem"`
}
type IntegrityCheckBatch struct {
	Sequence        int               `json:"sequence"`
	RevisionDigests map[string]string `json:"revisionDigests"`
	RuleSetVersion  int               `json:"ruleSetVersion"`
	GeneratedAt     time.Time         `json:"generatedAt"`
	Passed          bool              `json:"passed"`
	Problems        []CheckProblem    `json:"problems"`
	Differences     []CheckDifference `json:"differences"`
}
type RetakeIssue struct {
	ID                    string     `json:"id"`
	FindingID             string     `json:"findingId"`
	Requirement           string     `json:"requirement"`
	TargetView            string     `json:"targetView"`
	TargetZone            string     `json:"targetZone"`
	OriginalRevisionID    string     `json:"originalRevisionId"`
	ReplacementRevisionID string     `json:"replacementRevisionId,omitempty"`
	SubmittedAt           *time.Time `json:"submittedAt,omitempty"`
	LatestCheckSequence   int        `json:"latestCheckSequence"`
	Status                string     `json:"status"`
	VerificationBasis     string     `json:"verificationBasis,omitempty"`
}

type CandidateCoverage struct {
	RuleID         string `json:"ruleId"`
	ViewCode       string `json:"viewCode"`
	CoveredZone    string `json:"coveredZone"`
	Status         string `json:"status"`
	RevisionID     string `json:"revisionId,omitempty"`
	RevisionNumber int    `json:"revisionNumber,omitempty"`
}
