package idempotency_case_isolation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

func TestSubmitRevisionIdempotencyKeyCannotCrossCases(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := store.OpenSQLite(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	payloads, err := store.NewFilePayloadStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, payloads, application.SystemClock{}, &store.RandomIDGenerator{})
	operator := application.Principal{Name: "操作员", Role: application.RoleOperator}
	rules := domain.AcceptanceRuleSet{ID: "RULES", Version: 1, Rules: []domain.AcceptanceRule{{ID: "R1", Name: "规则", RequiredViews: []string{"FRONT"}, RequiredZones: []string{"ZONE-A"}, MinVoltageKV: 100, MaxVoltageKV: 250, MaxDefectSizeMM: 1}}}
	create := func(key, workpiece string) *domain.InspectionCase {
		c, createErr := service.CreateCase(ctx, application.CreateCaseCommand{IdempotencyKey: key, Principal: operator, WorkpieceCode: workpiece, InspectionZone: "ZONE-A", Technique: domain.TechniqueParameters{SourceType: "X-ray", VoltageKV: 180, ExposureSeconds: 2}, RuleSet: rules})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return c
	}
	first := create("create-iso-0001", "WP-A")
	second := create("create-iso-0002", "WP-B")
	payload := []byte("radiograph-for-case-a")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	command := application.SubmitRevisionCommand{Meta: application.CommandMeta{ExpectedVersion: first.Version, IdempotencyKey: "submit-iso-0001", Principal: operator}, CaseID: first.ID, CaptureBatch: "BATCH-A", ViewCode: "FRONT", CoveredZone: "ZONE-A", Exposure: domain.ExposureParameters{VoltageKV: 180, ExposureSeconds: 2}, ExpectedDigest: digest, Payload: bytes.NewReader(payload)}
	if _, err := service.SubmitRevision(ctx, command); err != nil {
		t.Fatal(err)
	}
	command.CaseID = second.ID
	command.Meta.ExpectedVersion = second.Version
	command.Payload = bytes.NewReader(payload)
	if replay, err := service.SubmitRevision(ctx, command); !errors.Is(err, domain.ErrIdempotencyConflict) || replay != nil {
		t.Fatalf("跨任务复用幂等键应返回冲突且不重放聚合: replay=%#v err=%v", replay, err)
	}
	loaded, err := service.GetCase(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != second.Version || len(loaded.Revisions) != 0 {
		t.Fatalf("任务 B 不应被任务 A 的重放结果污染: version=%d revisions=%d", loaded.Version, len(loaded.Revisions))
	}
}
