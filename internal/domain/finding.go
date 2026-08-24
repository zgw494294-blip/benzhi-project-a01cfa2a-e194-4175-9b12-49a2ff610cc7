package domain

import (
	"strings"
	"time"
)

type FindingSource string
type FindingStatus string

const (
	SourceAutomatic FindingSource = "automatic"
	SourceManual    FindingSource = "manual"
	FindingOpen     FindingStatus = "open"
	FindingClosed   FindingStatus = "closed"
)

type InterpretationFinding struct {
	ID            string        `json:"id"`
	CaseID        string        `json:"caseId"`
	RevisionID    string        `json:"revisionId,omitempty"`
	Source        FindingSource `json:"source"`
	FindingType   string        `json:"findingType"`
	Location      string        `json:"location"`
	MeasuredSize  float64       `json:"measuredSize"`
	Severity      string        `json:"severity"`
	RuleReference string        `json:"ruleReference"`
	Basis         string        `json:"basis"`
	Disposition   string        `json:"disposition"`
	Status        FindingStatus `json:"status"`
	CreatedAt     time.Time     `json:"createdAt"`
	ClosedAt      *time.Time    `json:"closedAt,omitempty"`
	ClosedBy      string        `json:"closedBy,omitempty"`
	ClosureNote   string        `json:"closureNote,omitempty"`
}

func (f InterpretationFinding) ValidateManual() error {
	if strings.TrimSpace(f.RevisionID) == "" {
		return Invalid("revisionId", "人工缺陷必须关联底片修订")
	}
	if strings.TrimSpace(f.FindingType) == "" || strings.TrimSpace(f.Location) == "" {
		return Invalid("finding", "缺陷类型和位置不能为空")
	}
	if f.MeasuredSize < 0 {
		return Invalid("measuredSize", "缺陷尺寸不能为负数")
	}
	if f.Severity != "info" && f.Severity != "warning" && f.Severity != "blocking" {
		return Invalid("severity", "严重度必须为 info、warning 或 blocking")
	}
	if strings.TrimSpace(f.RuleReference) == "" || strings.TrimSpace(f.Basis) == "" || strings.TrimSpace(f.Disposition) == "" {
		return Invalid("finding", "规则依据、判读依据和处置意见不能为空")
	}
	return nil
}

func (f InterpretationFinding) IsBlocking() bool {
	return f.Status == FindingOpen && f.Severity == "blocking"
}
