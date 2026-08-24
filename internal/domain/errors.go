package domain

import (
	"errors"
	"strings"
)

var (
	ErrInvalid             = errors.New("参数不符合业务规则")
	ErrConflict            = errors.New("任务版本冲突")
	ErrNotFound            = errors.New("记录不存在")
	ErrForbidden           = errors.New("当前角色无权执行该操作")
	ErrInvalidState        = errors.New("当前任务状态不允许该操作")
	ErrDuplicate           = errors.New("载荷或记录已存在")
	ErrIntegrity           = errors.New("完整性校验失败")
	ErrOpenBlocker         = errors.New("仍有未关闭的阻断问题")
	ErrAlreadyIssued       = errors.New("放行凭据已经签发")
	ErrIdempotencyConflict = errors.New("幂等键对应的载荷不一致")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

type ValidationErrors struct{ Items []*ValidationError }

func (e *ValidationErrors) Error() string {
	parts := make([]string, 0, len(e.Items))
	for _, item := range e.Items {
		if item != nil {
			parts = append(parts, item.Error())
		}
	}
	return strings.Join(parts, "；")
}
func (e *ValidationErrors) Add(field, message string) {
	e.Items = append(e.Items, &ValidationError{Field: field, Message: message})
}
func (e *ValidationErrors) Empty() bool { return len(e.Items) == 0 }
