package domain

type CaseStatus string

const (
	StatusDraft         CaseStatus = "draft"
	StatusPendingCheck  CaseStatus = "pending_check"
	StatusPendingReview CaseStatus = "pending_review"
	StatusRetake        CaseStatus = "retake"
	StatusReinspection  CaseStatus = "reinspection"
	StatusFrozen        CaseStatus = "frozen"
	StatusReleased      CaseStatus = "released"
)

func (s CaseStatus) Label() string {
	switch s {
	case StatusDraft:
		return "草稿"
	case StatusPendingCheck:
		return "待完整性检查"
	case StatusPendingReview:
		return "待人工判读"
	case StatusRetake:
		return "待返拍"
	case StatusReinspection:
		return "待复验"
	case StatusFrozen:
		return "已冻结"
	case StatusReleased:
		return "已放行"
	default:
		return "未知"
	}
}

func (s CaseStatus) Mutable() bool {
	return s != StatusFrozen && s != StatusReleased
}
