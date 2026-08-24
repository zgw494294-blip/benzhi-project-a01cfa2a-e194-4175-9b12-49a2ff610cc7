package application

import (
	"context"
	"reflect"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func (s *Service) CreateCase(ctx context.Context, cmd CreateCaseCommand) (*domain.InspectionCase, error) {
	if err := cmd.Principal.Validate(RoleOperator); err != nil {
		return nil, err
	}
	if err := cmd.RuleSet.Validate(); err != nil {
		return nil, err
	}
	replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.IdempotencyKey, "create_case")
	if err != nil {
		return nil, err
	}
	if ok {
		probe, probeErr := domain.NewInspectionCase("preflight", cmd.WorkpieceCode, cmd.InspectionZone, cmd.Technique, cmd.RuleSet, s.clock.Now())
		if probeErr != nil {
			return nil, probeErr
		}
		if replay.WorkpieceCode != probe.WorkpieceCode || replay.InspectionZone != probe.InspectionZone || !reflect.DeepEqual(replay.Technique, probe.Technique) || !reflect.DeepEqual(replay.AcceptanceRuleSet, probe.AcceptanceRuleSet) {
			return nil, domain.ErrIdempotencyConflict
		}
		return replay, nil
	}
	now := s.clock.Now()
	// Validate the complete payload before allocating an identifier or touching
	// the repository, so failed preflight has no observable side effects.
	probe, err := domain.NewInspectionCase("preflight", cmd.WorkpieceCode, cmd.InspectionZone, cmd.Technique, cmd.RuleSet, now)
	if err != nil {
		return nil, err
	}
	probe.ID = s.ids.NewID("case")
	c := probe
	result, err := encodeResult(c)
	if err != nil {
		return nil, err
	}
	e := s.eventAt(c, "case.created", cmd.Principal, map[string]any{"workpieceCode": c.WorkpieceCode})
	if err := s.repository.Create(ctx, c, e, cmd.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) SubmitRevision(ctx context.Context, cmd SubmitRevisionCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := s.replaySubmittedRevision(ctx, cmd.Meta.IdempotencyKey); err != nil || ok {
		return replay, err
	}
	if err := cmd.Meta.Principal.Validate(RoleOperator); err != nil {
		return nil, err
	}
	if err := validateExpectedVersion(cmd.Meta.ExpectedVersion); err != nil {
		return nil, err
	}
	c, err := s.repository.Load(ctx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if err := c.CheckVersion(cmd.Meta.ExpectedVersion); err != nil {
		return nil, err
	}
	candidate := domain.RadiographRevision{ID: "preflight", CaptureBatch: cmd.CaptureBatch, ViewCode: cmd.ViewCode, CoveredZone: cmd.CoveredZone, ExposureParameters: cmd.Exposure, ContentDigest: cmd.ExpectedDigest, StorageKey: "pending", SizeBytes: 1, SupersedesRevisionID: cmd.SupersedesRevisionID}
	if err := c.ValidateRevisionCandidate(candidate); err != nil {
		return nil, err
	}
	storageKey, digest, size, err := s.payloads.Put(ctx, cmd.ExpectedDigest, cmd.Payload, maxRadiographBytes)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	rev := domain.RadiographRevision{ID: s.ids.NewID("rev"), CaptureBatch: cmd.CaptureBatch, ViewCode: cmd.ViewCode, CoveredZone: cmd.CoveredZone, ExposureParameters: cmd.Exposure, ContentDigest: digest, StorageKey: storageKey, SizeBytes: size, SupersedesRevisionID: cmd.SupersedesRevisionID, SubmittedAt: now}
	previous := c.Version
	if err := c.AddRevision(rev, now); err != nil {
		return nil, err
	}
	result, err := encodeResult(c)
	if err != nil {
		return nil, err
	}
	e := s.eventAt(c, "revision.submitted", cmd.Meta.Principal, map[string]any{"revisionId": rev.ID, "digest": digest, "sizeBytes": size})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) replaySubmittedRevision(ctx context.Context, key string) (*domain.InspectionCase, bool, error) {
	replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, key, "submit_revision")
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return replay, true, nil
}

func (s *Service) RunCheck(ctx context.Context, caseID string, meta CommandMeta) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, meta.IdempotencyKey, "run_check"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, caseID, meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	findings, err := c.RunCompletenessCheckWithEvidence(func() string { return s.ids.NewID("finding") }, s.clock.Now(), func(rev domain.RadiographRevision) error {
		return s.payloads.Verify(ctx, rev.StorageKey, rev.ContentDigest, rev.SizeBytes)
	})
	if err != nil {
		return nil, err
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "check.completed", meta.Principal, map[string]any{"passed": c.LastCheckPassed, "findingCount": len(findings), "sequence": c.CheckSequence})
	if err := s.repository.Save(ctx, c, previous, e, meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}
