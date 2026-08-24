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

const maxJSONBody int64 = 1 << 20

type commandEnvelope struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		return domain.Invalid("contentType", "请求必须使用 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if err == io.EOF {
			return domain.Invalid("body", "请求体不能为空")
		}
		return domain.Invalid("body", "JSON 请求体无效: "+err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.Invalid("body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func principal(r *http.Request) (application.Principal, error) {
	p := application.Principal{Name: strings.TrimSpace(r.Header.Get("X-Actor")), Role: application.Role(strings.TrimSpace(r.Header.Get("X-Role")))}
	if p.Name == "" {
		return p, domain.Invalid("X-Actor", "必须提供操作人请求头")
	}
	if p.Role != application.RoleOperator && p.Role != application.RoleReviewer && p.Role != application.RoleQuality {
		return p, domain.Invalid("X-Role", "角色必须为 operator、reviewer 或 quality")
	}
	return p, nil
}

func meta(r *http.Request, envelope commandEnvelope) (application.CommandMeta, error) {
	p, err := principal(r)
	if err != nil {
		return application.CommandMeta{}, err
	}
	if envelope.ExpectedVersion < 1 {
		return application.CommandMeta{}, domain.Invalid("expectedVersion", "必须提供大于零的任务版本")
	}
	if strings.TrimSpace(envelope.IdempotencyKey) == "" {
		return application.CommandMeta{}, domain.Invalid("idempotencyKey", "必须提供幂等键")
	}
	return application.CommandMeta{ExpectedVersion: envelope.ExpectedVersion, IdempotencyKey: envelope.IdempotencyKey, Principal: p}, nil
}

func parsePositiveInt64(value, field string) (int64, error) {
	var parsed int64
	if _, err := fmt.Sscan(value, &parsed); err != nil || parsed < 1 {
		return 0, domain.Invalid(field, "必须为大于零的整数")
	}
	return parsed, nil
}
