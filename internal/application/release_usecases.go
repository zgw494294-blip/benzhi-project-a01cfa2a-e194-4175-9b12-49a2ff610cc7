package application

import (
	"context"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func (s *Service) Freeze(ctx context.Context, cmd FreezeCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.Meta.IdempotencyKey, "freeze"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	snapshot, err := c.Freeze(cmd.Meta.Principal.Name, s.clock.Now())
	if err != nil {
		return nil, err
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "case.frozen", cmd.Meta.Principal, map[string]any{"evidenceDigest": snapshot.EvidenceDigest, "frozenVersion": snapshot.FrozenVersion})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) Issue(ctx context.Context, cmd IssueCommand) (*domain.ReleaseCredential, error) {
	if replay, ok, err := decodeReplay[domain.ReleaseCredential](ctx, s.repository, cmd.Meta.IdempotencyKey, "issue"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleQuality)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	credential, err := c.IssueCredential(s.ids.NewID("RT"), cmd.Meta.Principal.Name, cmd.FrozenVersion, s.clock.Now())
	if err != nil {
		return nil, err
	}
	result, _ := encodeResult(credential)
	e := s.eventAt(c, "credential.issued", cmd.Meta.Principal, map[string]any{"credentialNumber": credential.CredentialNumber, "verificationDigest": credential.VerificationDigest})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return credential, nil
}
