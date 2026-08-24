package application

import (
	"context"
	"fmt"
	"io"
	"sort"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type CredentialVerification struct {
	Credential *domain.ReleaseCredential `json:"credential"`
	Snapshot   *domain.FrozenSnapshot    `json:"snapshot"`
	Valid      bool                      `json:"valid"`
	Message    string                    `json:"message"`
	Evidence   []CredentialEvidenceItem  `json:"evidence"`
}
type CredentialEvidenceItem struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Valid     bool   `json:"valid"`
	Message   string `json:"message"`
	Digest    string `json:"digest,omitempty"`
}

func (s *Service) GetCase(ctx context.Context, id string) (*domain.InspectionCase, error) {
	c, err := s.repository.Load(ctx, id)
	if err == nil {
		c.RefreshCoverage()
		c.NormalizeOrdering()
	}
	return c, err
}

func (s *Service) CheckHistory(ctx context.Context, id string) ([]domain.IntegrityCheckBatch, error) {
	c, err := s.repository.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.CheckBatches, nil
}

func (s *Service) ListCases(ctx context.Context) ([]domain.InspectionCase, error) {
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].RefreshCoverage()
		items[i].NormalizeOrdering()
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *Service) VerifyCredential(ctx context.Context, number string) (*CredentialVerification, error) {
	c, credential, err := s.repository.FindCredential(ctx, number)
	if err != nil {
		return nil, err
	}
	items := []CredentialEvidenceItem{}
	credValid := credential.Verify(c.Frozen)
	items = append(items, CredentialEvidenceItem{Type: "credential_digest", Reference: credential.CredentialNumber, Valid: credValid, Message: map[bool]string{true: "凭据材料摘要通过", false: "凭据材料摘要异常"}[credValid], Digest: credential.VerificationDigest})
	snapshotValid := c.Frozen != nil && c.Frozen.Verify()
	items = append(items, CredentialEvidenceItem{Type: "snapshot_digest", Reference: c.ID, Valid: snapshotValid, Message: map[bool]string{true: "冻结快照摘要通过", false: "冻结快照摘要异常"}[snapshotValid], Digest: func() string {
		if c.Frozen != nil {
			return c.Frozen.EvidenceDigest
		}
		return ""
	}()})
	versionValid := c.Frozen != nil && credential.FrozenVersion == c.Frozen.FrozenVersion && c.Version >= credential.FrozenVersion
	items = append(items, CredentialEvidenceItem{Type: "frozen_version", Reference: fmt.Sprint(credential.FrozenVersion), Valid: versionValid, Message: map[bool]string{true: "凭据与冻结版本一致", false: "凭据与冻结版本不一致"}[versionValid]})
	valid := credValid && snapshotValid && versionValid
	if c.Frozen != nil {
		for _, rev := range c.Frozen.Revisions {
			err := s.payloads.Verify(ctx, rev.StorageKey, rev.ContentDigest, rev.SizeBytes)
			ok := err == nil
			items = append(items, CredentialEvidenceItem{Type: "payload", Reference: rev.ID, Valid: ok, Message: map[bool]string{true: "底片载荷摘要与大小通过", false: "底片载荷缺失或摘要不一致"}[ok], Digest: rev.ContentDigest})
			valid = valid && ok
		}
	}
	message := "凭据逐项完整性核验通过"
	if !valid {
		message = "凭据存在完整性异常，请按证据项定位"
	}
	return &CredentialVerification{Credential: credential, Snapshot: c.Frozen, Valid: valid, Message: message, Evidence: items}, nil
}

func (s *Service) OpenRevision(ctx context.Context, caseID, revisionID string) (io.ReadCloser, int64, string, error) {
	c, err := s.repository.Load(ctx, caseID)
	if err != nil {
		return nil, 0, "", err
	}
	rev, ok := c.Revision(revisionID)
	if !ok {
		return nil, 0, "", domain.ErrNotFound
	}
	if err := s.payloads.Verify(ctx, rev.StorageKey, rev.ContentDigest, rev.SizeBytes); err != nil {
		return nil, 0, "", fmt.Errorf("%w: %v", domain.ErrIntegrity, err)
	}
	s.openReadersMu.Lock()
	defer s.openReadersMu.Unlock()
	if cached, ok := s.openReaders[rev.StorageKey]; ok {
		return cached.reader, cached.size, rev.ContentDigest, nil
	}
	reader, size, err := s.payloads.Open(ctx, rev.StorageKey)
	if err != nil {
		return nil, 0, "", err
	}
	s.openReaders[rev.StorageKey] = cachedRevisionReader{reader: reader, size: size}
	return reader, size, rev.ContentDigest, nil
}

func (s *Service) AuditTrail(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	return s.repository.AuditTrail(ctx, caseID)
}

func (s *Service) Health(ctx context.Context) error { return s.repository.CheckIntegrity(ctx) }
