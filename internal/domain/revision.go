package domain

import (
	"strings"
	"time"
)

type RadiographRevision struct {
	ID                   string             `json:"id"`
	CaseID               string             `json:"caseId"`
	RevisionNumber       int                `json:"revisionNumber"`
	CaptureBatch         string             `json:"captureBatch"`
	ViewCode             string             `json:"viewCode"`
	CoveredZone          string             `json:"coveredZone"`
	ExposureParameters   ExposureParameters `json:"exposureParameters"`
	ContentDigest        string             `json:"contentDigest"`
	StorageKey           string             `json:"storageKey"`
	SizeBytes            int64              `json:"sizeBytes"`
	SupersedesRevisionID string             `json:"supersedesRevisionId,omitempty"`
	SubmittedAt          time.Time          `json:"submittedAt"`
}

func (r RadiographRevision) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return Invalid("id", "底片修订标识不能为空")
	}
	if strings.TrimSpace(r.CaptureBatch) == "" {
		return Invalid("captureBatch", "拍摄批次不能为空")
	}
	if strings.TrimSpace(r.ViewCode) == "" {
		return Invalid("viewCode", "视图代码不能为空")
	}
	if strings.TrimSpace(r.CoveredZone) == "" {
		return Invalid("coveredZone", "覆盖区域不能为空")
	}
	if len(r.ContentDigest) != 64 {
		return Invalid("contentDigest", "文件摘要必须为 SHA-256 十六进制值")
	}
	if r.SizeBytes <= 0 {
		return Invalid("payload", "底片载荷不能为空")
	}
	if r.ExposureParameters.VoltageKV <= 0 || r.ExposureParameters.ExposureSeconds <= 0 {
		return Invalid("exposureParameters", "管电压和曝光时间必须大于零")
	}
	return nil
}
