package application

import (
	"io"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type CommandMeta struct {
	ExpectedVersion int64
	IdempotencyKey  string
	Principal       Principal
}

type CreateCaseCommand struct {
	IdempotencyKey string
	Principal      Principal
	WorkpieceCode  string
	InspectionZone string
	Technique      domain.TechniqueParameters
	RuleSet        domain.AcceptanceRuleSet
}

type SubmitRevisionCommand struct {
	Meta                 CommandMeta
	CaseID               string
	CaptureBatch         string
	ViewCode             string
	CoveredZone          string
	Exposure             domain.ExposureParameters
	ExpectedDigest       string
	SupersedesRevisionID string
	Payload              io.Reader
}

type AddFindingCommand struct {
	Meta          CommandMeta
	CaseID        string
	RevisionID    string
	FindingType   string
	Location      string
	MeasuredSize  float64
	Severity      string
	RuleReference string
	Basis         string
	Disposition   string
}

type FindingInput struct {
	RevisionID    string  `json:"revisionId"`
	FindingType   string  `json:"findingType"`
	Location      string  `json:"location"`
	MeasuredSize  float64 `json:"measuredSize"`
	Severity      string  `json:"severity"`
	RuleReference string  `json:"ruleReference"`
	Basis         string  `json:"basis"`
	Disposition   string  `json:"disposition"`
}
type AddFindingsCommand struct {
	Meta        CommandMeta
	CaseID      string
	Findings    []FindingInput
	Conclusions []domain.RuleConclusion
}

type SetConclusionsCommand struct {
	Meta        CommandMeta
	CaseID      string
	Conclusions []domain.RuleConclusion
}

type RetakeCommand struct {
	Meta        CommandMeta
	CaseID      string
	Requirement string
	Items       []domain.RetakeIssue
}

type CloseFindingCommand struct {
	Meta        CommandMeta
	CaseID      string
	FindingID   string
	ClosureNote string
}
type CloseFindingsCommand struct {
	Meta              CommandMeta
	CaseID            string
	FindingIDs        []string
	VerificationBasis string
}

type FreezeCommand struct {
	Meta   CommandMeta
	CaseID string
}
type IssueCommand struct {
	Meta          CommandMeta
	CaseID        string
	FrozenVersion int64
}
