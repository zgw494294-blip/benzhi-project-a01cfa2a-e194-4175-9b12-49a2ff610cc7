package credentialevidencerace

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type credentialRepository struct {
	item       *domain.InspectionCase
	credential *domain.ReleaseCredential
}

func (r *credentialRepository) Create(context.Context, *domain.InspectionCase, domain.AuditEvent, string, []byte) error {
	panic("unexpected Create")
}

func (r *credentialRepository) Load(context.Context, string) (*domain.InspectionCase, error) {
	panic("unexpected Load")
}

func (r *credentialRepository) Save(context.Context, *domain.InspectionCase, int64, domain.AuditEvent, string, []byte) error {
	panic("unexpected Save")
}

func (r *credentialRepository) List(context.Context) ([]domain.InspectionCase, error) {
	panic("unexpected List")
}

func (r *credentialRepository) FindCredential(context.Context, string) (*domain.InspectionCase, *domain.ReleaseCredential, error) {
	return r.item, r.credential, nil
}

func (r *credentialRepository) IdempotencyResult(context.Context, string, string) ([]byte, bool, error) {
	panic("unexpected IdempotencyResult")
}

func (r *credentialRepository) AuditTrail(context.Context, string) ([]domain.AuditEvent, error) {
	panic("unexpected AuditTrail")
}

func (r *credentialRepository) CheckIntegrity(context.Context) error { return nil }
func (r *credentialRepository) Close() error                         { return nil }

type synchronizedPayloadStore struct {
	mu      sync.Mutex
	started int
	release chan struct{}
}

func newSynchronizedPayloadStore() *synchronizedPayloadStore {
	return &synchronizedPayloadStore{release: make(chan struct{})}
}

func (s *synchronizedPayloadStore) Put(context.Context, string, io.Reader, int64) (string, string, int64, error) {
	panic("unexpected Put")
}

func (s *synchronizedPayloadStore) Open(context.Context, string) (io.ReadCloser, int64, error) {
	panic("unexpected Open")
}

func (s *synchronizedPayloadStore) Verify(context.Context, string, string, int64) error {
	s.mu.Lock()
	s.started++
	if s.started == 2 {
		close(s.release)
	}
	s.mu.Unlock()
	<-s.release
	return nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type fixedIDs struct{}

func (fixedIDs) NewID(prefix string) string { return prefix + "-fixed" }

func releasedCase(t *testing.T) (*domain.InspectionCase, *domain.ReleaseCredential) {
	t.Helper()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	rules := domain.AcceptanceRuleSet{ID: "RULES", Version: 1, Rules: []domain.AcceptanceRule{{
		ID: "R-1", Name: "双视图规则", RequiredViews: []string{"FRONT", "SIDE"},
		RequiredZones: []string{"WELD-A"}, MinVoltageKV: 150, MaxVoltageKV: 220, MaxDefectSizeMM: 2,
	}}}
	item, err := domain.NewInspectionCase("case-race", "WP-RACE", "WELD-A", domain.TechniqueParameters{
		SourceType: "X-ray", VoltageKV: 180, CurrentMA: 5, ExposureSeconds: 2, SourceDistanceMM: 600,
	}, rules, now)
	if err != nil {
		t.Fatal(err)
	}
	digests := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	for i, view := range []string{"FRONT", "SIDE"} {
		revision := domain.RadiographRevision{
			ID: "rev-" + view, CaptureBatch: "BATCH-1", ViewCode: view, CoveredZone: "WELD-A",
			ExposureParameters: domain.ExposureParameters{VoltageKV: 180, CurrentMA: 5, ExposureSeconds: 2, SourceDistanceMM: 600},
			ContentDigest:      digests[i], StorageKey: digests[i][:2] + "/" + digests[i], SizeBytes: 128, SubmittedAt: now,
		}
		if err := item.AddRevision(revision, now.Add(time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := item.RunCompletenessCheck(func() string { return "unused" }, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := item.SetRuleConclusions([]domain.RuleConclusion{{RuleID: "R-1", Conclusion: "pass", Basis: "双视图均通过"}}, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := item.Freeze("reviewer", now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	credential, err := item.IssueCredential("RT-RACE-001", "quality", snapshot.FrozenVersion, now.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return item, credential
}

func TestCredentialPayloadChecksAreRaceFree(t *testing.T) {
	item, credential := releasedCase(t)
	repository := &credentialRepository{item: item, credential: credential}
	payloads := newSynchronizedPayloadStore()
	service := application.NewService(repository, payloads, fixedClock{now: time.Now()}, fixedIDs{})

	result, err := service.VerifyCredential(context.Background(), credential.CredentialNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Fatal("两个有效底片的凭据核验不应失败")
	}
	if len(result.Evidence) != 5 {
		t.Fatalf("应返回 3 个凭据证据项和 2 个底片证据项，得到 %d", len(result.Evidence))
	}
}
