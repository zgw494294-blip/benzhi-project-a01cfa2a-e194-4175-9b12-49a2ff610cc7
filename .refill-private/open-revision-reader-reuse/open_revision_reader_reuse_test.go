package open_revision_reader_reuse_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func TestSequentialRevisionDownloadsUseFreshReaders(t *testing.T) {
	caseRecord := &domain.InspectionCase{
		ID: "case-reader-owner",
		Revisions: []domain.RadiographRevision{{
			ID:            "rev-stable",
			ContentDigest: strings.Repeat("a", 64),
			StorageKey:    "sha256/aa/stable",
			SizeBytes:     17,
		}},
	}
	payloads := &freshReaderStore{content: "radiograph bytes"}
	service := application.NewService(&loadOnlyRepository{caseRecord: caseRecord}, payloads, application.SystemClock{}, fixedIDs{})

	first, _, _, err := service.OpenRevision(context.Background(), caseRecord.ID, "rev-stable")
	if err != nil {
		t.Fatalf("first download failed: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first download: %v", err)
	}

	second, _, _, err := service.OpenRevision(context.Background(), caseRecord.ID, "rev-stable")
	if err != nil {
		t.Fatalf("second download failed: %v", err)
	}
	defer second.Close()
	content, err := io.ReadAll(second)
	if err != nil {
		t.Fatalf("second download reused an invalid reader: %v", err)
	}
	if string(content) != payloads.content {
		t.Fatalf("second download content = %q, want %q", content, payloads.content)
	}
	if payloads.openCalls != 2 {
		t.Fatalf("payload Open calls = %d, want 2 independent readers", payloads.openCalls)
	}
}

type guardedReader struct {
	reader *strings.Reader
	closed bool
}

func (r *guardedReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("reader is closed")
	}
	return r.reader.Read(p)
}

func (r *guardedReader) Close() error {
	r.closed = true
	return nil
}

type freshReaderStore struct {
	content   string
	openCalls int
}

func (p *freshReaderStore) Put(context.Context, string, io.Reader, int64) (string, string, int64, error) {
	return "", "", 0, errors.New("unexpected Put")
}

func (p *freshReaderStore) Open(context.Context, string) (io.ReadCloser, int64, error) {
	p.openCalls++
	return &guardedReader{reader: strings.NewReader(p.content)}, int64(len(p.content)), nil
}

func (p *freshReaderStore) Verify(context.Context, string, string, int64) error { return nil }

type loadOnlyRepository struct {
	caseRecord *domain.InspectionCase
}

func (r *loadOnlyRepository) Create(context.Context, *domain.InspectionCase, domain.AuditEvent, string, []byte) error {
	return errors.New("unexpected Create")
}

func (r *loadOnlyRepository) Load(context.Context, string) (*domain.InspectionCase, error) {
	return r.caseRecord, nil
}

func (r *loadOnlyRepository) Save(context.Context, *domain.InspectionCase, int64, domain.AuditEvent, string, []byte) error {
	return errors.New("unexpected Save")
}

func (r *loadOnlyRepository) List(context.Context) ([]domain.InspectionCase, error) {
	return nil, errors.New("unexpected List")
}

func (r *loadOnlyRepository) FindCredential(context.Context, string) (*domain.InspectionCase, *domain.ReleaseCredential, error) {
	return nil, nil, errors.New("unexpected FindCredential")
}

func (r *loadOnlyRepository) IdempotencyResult(context.Context, string, string) ([]byte, bool, error) {
	return nil, false, errors.New("unexpected IdempotencyResult")
}

func (r *loadOnlyRepository) AuditTrail(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, errors.New("unexpected AuditTrail")
}

func (r *loadOnlyRepository) CheckIntegrity(context.Context) error {
	return errors.New("unexpected CheckIntegrity")
}

func (r *loadOnlyRepository) Close() error { return nil }

type fixedIDs struct{}

func (fixedIDs) NewID(string) string { return "unused" }
