package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

const maxRadiographBytes int64 = 32 << 20

type Service struct {
	repository       Repository
	payloads         PayloadStore
	clock            Clock
	ids              IDGenerator
	verifiedPayloads sync.Map
}

func NewService(repository Repository, payloads PayloadStore, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, payloads: payloads, clock: clock, ids: ids}
}

func validateIdempotency(key string) error {
	key = strings.TrimSpace(key)
	if len(key) < 8 || len(key) > 128 {
		return domain.Invalid("idempotencyKey", "幂等键长度必须为 8 到 128 个字符")
	}
	return nil
}

func validateExpectedVersion(version int64) error {
	if version < 1 {
		return domain.Invalid("expectedVersion", "必须提供大于零的任务版本")
	}
	return nil
}

func encodeResult(value any) ([]byte, error) { return json.Marshal(value) }

func decodeReplay[T any](ctx context.Context, repo Repository, key, operation string) (*T, bool, error) {
	if err := validateIdempotency(key); err != nil {
		return nil, false, err
	}
	data, ok, err := repo.IdempotencyResult(ctx, key, operation)
	if err != nil || !ok {
		return nil, ok, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false, fmt.Errorf("读取幂等结果: %w", err)
	}
	return &result, true, nil
}

func (s *Service) eventAt(c *domain.InspectionCase, action string, p Principal, details map[string]any) domain.AuditEvent {
	return domain.AuditEvent{ID: s.ids.NewID("evt"), CaseID: c.ID, Action: action, Actor: p.Name, Role: string(p.Role), Version: c.Version, At: s.clock.Now(), Details: details}
}

func (s *Service) loadForWrite(ctx context.Context, id string, meta CommandMeta, allowed ...Role) (*domain.InspectionCase, error) {
	if err := meta.Principal.Validate(allowed...); err != nil {
		return nil, err
	}
	if err := validateIdempotency(meta.IdempotencyKey); err != nil {
		return nil, err
	}
	if err := validateExpectedVersion(meta.ExpectedVersion); err != nil {
		return nil, err
	}
	c, err := s.repository.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.CheckVersion(meta.ExpectedVersion); err != nil {
		return nil, err
	}
	return c, nil
}
