package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type problem struct {
	Code    string                    `json:"code"`
	Message string                    `json:"message"`
	Field   string                    `json:"field,omitempty"`
	Errors  []*domain.ValidationError `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("写入 HTTP 响应失败", "error", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "服务处理请求时发生错误"
	var validation *domain.ValidationError
	switch {
	case func() bool { var many *domain.ValidationErrors; return errors.As(err, &many) }():
		var many *domain.ValidationErrors
		_ = errors.As(err, &many)
		writeJSON(w, http.StatusBadRequest, problem{Code: "validation_error", Message: many.Error(), Errors: many.Items})
		return
	case errors.As(err, &validation):
		status, code, message = http.StatusBadRequest, "validation_error", validation.Message
		writeJSON(w, status, problem{Code: code, Message: message, Field: validation.Field})
		return
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "请求的记录不存在"
	case errors.Is(err, domain.ErrConflict):
		status, code, message = http.StatusConflict, "version_conflict", "任务版本已变化，请刷新后重试"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		status, code, message = http.StatusConflict, "idempotency_conflict", "同一幂等键不能提交不同载荷"
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "当前角色无权执行该操作"
	case errors.Is(err, domain.ErrInvalidState):
		status, code, message = http.StatusConflict, "invalid_state", "当前任务状态不允许该操作"
	case errors.Is(err, domain.ErrDuplicate):
		status, code, message = http.StatusConflict, "duplicate", "底片载荷或记录已经存在"
	case errors.Is(err, domain.ErrIntegrity):
		status, code, message = http.StatusUnprocessableEntity, "integrity_error", "底片载荷或证据完整性校验失败"
	case errors.Is(err, domain.ErrOpenBlocker):
		status, code, message = http.StatusConflict, "open_blocker", "仍有未关闭的阻断问题或未通过规则"
	case errors.Is(err, domain.ErrAlreadyIssued):
		status, code, message = http.StatusConflict, "already_issued", "放行凭据已经签发"
	default:
		slog.Error("HTTP 请求处理失败", "error", err)
	}
	writeJSON(w, status, problem{Code: code, Message: message})
}
