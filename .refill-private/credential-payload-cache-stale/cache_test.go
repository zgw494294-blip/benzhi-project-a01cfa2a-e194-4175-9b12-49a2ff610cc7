package credential_payload_cache_stale_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type credentialRepository struct {
	caseData   *domain.InspectionCase
	credential *domain.ReleaseCredential
}

func (r *credentialRepository) Create(context.Context, *domain.InspectionCase, domain.AuditEvent, string, []byte) error {
	return errors.New("unexpected Create")
}

func (r *credentialRepository) Load(context.Context, string) (*domain.InspectionCase, error) {
	return nil, errors.New("unexpected Load")
}

func (r *credentialRepository) Save(context.Context, *domain.InspectionCase, int64, domain.AuditEvent, string, []byte) error {
	return errors.New("unexpected Save")
}

func (r *credentialRepository) List(context.Context) ([]domain.InspectionCase, error) {
	return nil, errors.New("unexpected List")
}

func (r *credentialRepository) FindCredential(context.Context, string) (*domain.InspectionCase, *domain.ReleaseCredential, error) {
	return r.caseData, r.credential, nil
}

func (r *credentialRepository) IdempotencyResult(context.Context, string, string) ([]byte, bool, error) {
	return nil, false, errors.New("unexpected IdempotencyResult")
}

func (r *credentialRepository) AuditTrail(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, errors.New("unexpected AuditTrail")
}

func (r *credentialRepository) CheckIntegrity(context.Context) error { return nil }
func (r *credentialRepository) Close() error                         { return nil }

type invalidatablePayloadStore struct {
	invalid     bool
	verifyCalls int
}

func (s *invalidatablePayloadStore) Put(context.Context, string, io.Reader, int64) (string, string, int64, error) {
	return "", "", 0, errors.New("unexpected Put")
}

func (s *invalidatablePayloadStore) Open(context.Context, string) (io.ReadCloser, int64, error) {
	return nil, 0, errors.New("unexpected Open")
}

func (s *invalidatablePayloadStore) Verify(context.Context, string, string, int64) error {
	s.verifyCalls++
	if s.invalid {
		return domain.ErrNotFound
	}
	return nil
}

func issuedCase(t *testing.T) (*domain.InspectionCase, *domain.ReleaseCredential) {
	t.Helper()
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	c, err := domain.NewInspectionCase(
		"case-cache",
		"WP-CACHE",
		"ZONE-A",
		domain.TechniqueParameters{SourceType: "X-ray", VoltageKV: 150, ExposureSeconds: 2},
		domain.AcceptanceRuleSet{ID: "RULES", Version: 1, Rules: []domain.AcceptanceRule{{
			ID: "R1", Name: "焊缝规则", RequiredViews: []string{"FRONT"}, RequiredZones: []string{"ZONE-A"},
			MinVoltageKV: 100, MaxVoltageKV: 200, MaxDefectSizeMM: 1,
		}}},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err = c.AddRevision(domain.RadiographRevision{
		ID: "rev-cache", CaptureBatch: "BATCH-1", ViewCode: "FRONT", CoveredZone: "ZONE-A",
		ExposureParameters: domain.ExposureParameters{VoltageKV: 150, ExposureSeconds: 2},
		ContentDigest:      digest, StorageKey: "aa/" + digest, SizeBytes: 128,
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunCompletenessCheck(func() string { return "finding-unused" }, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetRuleConclusions([]domain.RuleConclusion{{RuleID: "R1", Conclusion: "pass", Basis: "未见超标缺陷"}}, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Freeze("判读员", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	credential, err := c.IssueCredential("RT-CACHE-001", "质量负责人", c.Frozen.FrozenVersion, now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return c, credential
}

func TestCredentialVerificationRechecksInvalidatedPayload(t *testing.T) {
	c, credential := issuedCase(t)
	repository := &credentialRepository{caseData: c, credential: credential}
	payloads := &invalidatablePayloadStore{}
	service := application.NewService(repository, payloads, nil, nil)

	first, err := service.VerifyCredential(context.Background(), credential.CredentialNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Valid {
		t.Fatal("初次凭据核验应通过")
	}

	payloads.invalid = true
	second, err := service.VerifyCredential(context.Background(), credential.CredentialNumber)
	if err != nil {
		t.Fatal(err)
	}
	if second.Valid {
		t.Fatal("载荷失效后二次凭据核验仍错误通过")
	}
	if payloads.verifyCalls != 2 {
		t.Fatalf("每次凭据核验都应重新读取载荷，实际调用 %d 次", payloads.verifyCalls)
	}
}
