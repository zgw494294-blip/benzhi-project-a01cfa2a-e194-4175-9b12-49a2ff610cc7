package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type revisionMetadata struct {
	ExpectedVersion      int64                     `json:"expectedVersion"`
	IdempotencyKey       string                    `json:"idempotencyKey"`
	CaptureBatch         string                    `json:"captureBatch"`
	ViewCode             string                    `json:"viewCode"`
	CoveredZone          string                    `json:"coveredZone"`
	Exposure             domain.ExposureParameters `json:"exposureParameters"`
	ContentDigest        string                    `json:"contentDigest"`
	SupersedesRevisionID string                    `json:"supersedesRevisionId"`
}

func (a *API) SubmitRevisionHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (32<<20)+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, domain.Invalid("body", "multipart 请求无效或超过大小限制"))
		return
	}
	metadataText := r.FormValue("metadata")
	var metadata revisionMetadata
	decoder := json.NewDecoder(strings.NewReader(metadataText))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		writeError(w, domain.Invalid("metadata", "底片元数据 JSON 无效"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, domain.Invalid("file", "必须上传底片文件"))
		return
	}
	defer file.Close()
	p, err := principal(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.SubmitRevision(r.Context(), application.SubmitRevisionCommand{Meta: application.CommandMeta{ExpectedVersion: metadata.ExpectedVersion, IdempotencyKey: metadata.IdempotencyKey, Principal: p}, CaseID: r.PathValue("caseId"), CaptureBatch: metadata.CaptureBatch, ViewCode: metadata.ViewCode, CoveredZone: metadata.CoveredZone, Exposure: metadata.Exposure, ExpectedDigest: metadata.ContentDigest, SupersedesRevisionID: metadata.SupersedesRevisionID, Payload: file})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) DownloadRevisionHandler(w http.ResponseWriter, r *http.Request) {
	reader, size, digest, err := a.service.OpenRevision(r.Context(), r.PathValue("caseId"), r.PathValue("revisionId"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("ETag", `"sha256-`+digest+`"`)
	w.Header().Set("Content-Disposition", `attachment; filename="radiograph.bin"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}
