package revisionpayloadrollback_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

var errSaveFailed = errors.New("forced save failure")

type failingSaveRepository struct {
	application.Repository
	item *domain.InspectionCase
}

func (r failingSaveRepository) IdempotencyResult(context.Context, string, string) ([]byte, bool, error) {
	return nil, false, nil
}

func (r failingSaveRepository) Load(context.Context, string) (*domain.InspectionCase, error) {
	return r.item, nil
}

func (r failingSaveRepository) Save(context.Context, *domain.InspectionCase, int64, domain.AuditEvent, string, []byte) error {
	return errSaveFailed
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type fixedIDs struct{}

func (fixedIDs) NewID(prefix string) string { return prefix + "-fixed" }

func TestFailedRevisionSaveRemovesStoredPayload(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	c, err := domain.NewInspectionCase("case-1", "WP-1", "A", domain.TechniqueParameters{SourceType: "X-ray", VoltageKV: 180, ExposureSeconds: 2}, domain.AcceptanceRuleSet{ID: "rules", Version: 1, Rules: []domain.AcceptanceRule{{ID: "r1", Name: "rule", RequiredViews: []string{"FRONT"}, RequiredZones: []string{"A"}, MinVoltageKV: 100, MaxVoltageKV: 300, MaxDefectSizeMM: 2}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	payloads, err := store.NewFilePayloadStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(failingSaveRepository{item: c}, payloads, fixedClock{now: now}, fixedIDs{})
	payload := []byte("radiograph-to-rollback")
	sum := sha256.Sum256(payload)
	_, err = service.SubmitRevision(context.Background(), application.SubmitRevisionCommand{
		Meta:   application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: "rollback-upload-0001", Principal: application.Principal{Name: "operator", Role: application.RoleOperator}},
		CaseID: "case-1", CaptureBatch: "batch-1", ViewCode: "FRONT", CoveredZone: "A",
		Exposure: domain.ExposureParameters{VoltageKV: 180, ExposureSeconds: 2}, ExpectedDigest: hex.EncodeToString(sum[:]), Payload: bytes.NewReader(payload),
	})
	if !errors.Is(err, errSaveFailed) {
		t.Fatalf("expected forced save failure, got %v", err)
	}
	files := 0
	err = filepath.WalkDir(filepath.Join(dataDir, "radiographs"), func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files != 0 {
		t.Fatalf("failed aggregate save left %d unreferenced payload file(s)", files)
	}
}
