package application

import (
	"context"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func (s *Service) AddFinding(ctx context.Context, cmd AddFindingCommand) (*domain.InspectionCase, error) {
	return s.AddFindings(ctx, AddFindingsCommand{Meta: cmd.Meta, CaseID: cmd.CaseID, Findings: []FindingInput{{RevisionID: cmd.RevisionID, FindingType: cmd.FindingType, Location: cmd.Location, MeasuredSize: cmd.MeasuredSize, Severity: cmd.Severity, RuleReference: cmd.RuleReference, Basis: cmd.Basis, Disposition: cmd.Disposition}}})
}

func (s *Service) AddFindings(ctx context.Context, cmd AddFindingsCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.Meta.IdempotencyKey, "add_finding"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	inputs := make([]domain.InterpretationFinding, 0, len(cmd.Findings))
	for _, input := range cmd.Findings {
		inputs = append(inputs, domain.InterpretationFinding{ID: s.ids.NewID("finding"), RevisionID: input.RevisionID, FindingType: input.FindingType, Location: input.Location, MeasuredSize: input.MeasuredSize, Severity: input.Severity, RuleReference: input.RuleReference, Basis: input.Basis, Disposition: input.Disposition})
	}
	if err := c.AddManualFindings(inputs, cmd.Conclusions, s.clock.Now()); err != nil {
		return nil, err
	}
	if len(cmd.Conclusions) > 0 {
		if err := c.SetRuleConclusions(cmd.Conclusions, s.clock.Now()); err != nil {
			return nil, err
		}
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "finding.added", cmd.Meta.Principal, map[string]any{"count": len(inputs)})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) SetConclusions(ctx context.Context, cmd SetConclusionsCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.Meta.IdempotencyKey, "set_conclusions"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	if err := c.SetRuleConclusions(cmd.Conclusions, s.clock.Now()); err != nil {
		return nil, err
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "review.conclusions_set", cmd.Meta.Principal, map[string]any{"count": len(cmd.Conclusions)})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) RequestRetake(ctx context.Context, cmd RetakeCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.Meta.IdempotencyKey, "request_retake"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	var retakeErr error
	if len(cmd.Items) == 0 {
		retakeErr = c.RequestRetake(cmd.Requirement, s.clock.Now())
	} else {
		retakeErr = c.RequestRetakeWithItems(cmd.Requirement, cmd.Items, s.clock.Now())
	}
	if retakeErr != nil {
		return nil, retakeErr
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "retake.requested", cmd.Meta.Principal, map[string]any{"requirement": cmd.Requirement})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) CloseFindings(ctx context.Context, cmd CloseFindingsCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.Meta.IdempotencyKey, "close_findings"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	if err := c.CloseFindings(cmd.FindingIDs, cmd.Meta.Principal.Name, cmd.VerificationBasis, s.clock.Now()); err != nil {
		return nil, err
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "retake.findings_closed", cmd.Meta.Principal, map[string]any{"findingIds": cmd.FindingIDs})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) CloseFinding(ctx context.Context, cmd CloseFindingCommand) (*domain.InspectionCase, error) {
	if replay, ok, err := decodeReplay[domain.InspectionCase](ctx, s.repository, cmd.Meta.IdempotencyKey, "close_finding"); err != nil || ok {
		return replay, err
	}
	c, err := s.loadForWrite(ctx, cmd.CaseID, cmd.Meta, RoleReviewer)
	if err != nil {
		return nil, err
	}
	previous := c.Version
	if err := c.CloseFinding(cmd.FindingID, cmd.Meta.Principal.Name, cmd.ClosureNote, s.clock.Now()); err != nil {
		return nil, err
	}
	result, _ := encodeResult(c)
	e := s.eventAt(c, "finding.closed", cmd.Meta.Principal, map[string]any{"findingId": cmd.FindingID, "note": cmd.ClosureNote})
	if err := s.repository.Save(ctx, c, previous, e, cmd.Meta.IdempotencyKey, result); err != nil {
		return nil, err
	}
	return c, nil
}
