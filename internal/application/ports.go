package application

import (
	"context"
	"io"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, c *domain.InspectionCase, event domain.AuditEvent, idempotencyKey string, result []byte) error
	Load(ctx context.Context, id string) (*domain.InspectionCase, error)
	Save(ctx context.Context, c *domain.InspectionCase, previousVersion int64, event domain.AuditEvent, idempotencyKey string, result []byte) error
	List(ctx context.Context) ([]domain.InspectionCase, error)
	FindCredential(ctx context.Context, number string) (*domain.InspectionCase, *domain.ReleaseCredential, error)
	IdempotencyResult(ctx context.Context, key, operation string) ([]byte, bool, error)
	AuditTrail(ctx context.Context, caseID string) ([]domain.AuditEvent, error)
	CheckIntegrity(ctx context.Context) error
	Close() error
}

type PayloadStore interface {
	Put(ctx context.Context, expectedDigest string, source io.Reader, maxBytes int64) (storageKey, digest string, size int64, err error)
	Open(ctx context.Context, storageKey string) (io.ReadCloser, int64, error)
	Verify(ctx context.Context, storageKey, digest string, size int64) error
	Delete(ctx context.Context, storageKey string) error
}

type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID(prefix string) string }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
