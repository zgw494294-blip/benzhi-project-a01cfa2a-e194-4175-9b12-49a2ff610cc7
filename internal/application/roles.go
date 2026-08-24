package application

import (
	"fmt"
	"strings"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type Role string

const (
	RoleOperator Role = "operator"
	RoleReviewer Role = "reviewer"
	RoleQuality  Role = "quality"
)

type Principal struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

func (p Principal) Validate(allowed ...Role) error {
	if strings.TrimSpace(p.Name) == "" {
		return domain.Invalid("actor", "操作人不能为空")
	}
	for _, role := range allowed {
		if p.Role == role {
			return nil
		}
	}
	return fmt.Errorf("%w: 角色 %s 不允许执行此操作", domain.ErrForbidden, p.Role)
}
