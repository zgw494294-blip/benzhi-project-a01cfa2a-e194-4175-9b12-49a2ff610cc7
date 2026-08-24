package httpapi

import (
	"net/http"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type createCaseRequest struct {
	IdempotencyKey string                     `json:"idempotencyKey"`
	WorkpieceCode  string                     `json:"workpieceCode"`
	InspectionZone string                     `json:"inspectionZone"`
	Technique      domain.TechniqueParameters `json:"techniqueParameters"`
	RuleSet        acceptanceRuleSetRequest   `json:"acceptanceRuleSet"`
}
type acceptanceRuleSetRequest struct {
	ID      string                  `json:"id"`
	Version int                     `json:"version"`
	Rules   []acceptanceRuleRequest `json:"rules"`
}
type acceptanceRuleRequest struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RequiredViews   []string `json:"requiredViews"`
	RequiredZones   []string `json:"requiredZones"`
	MinVoltageKV    *float64 `json:"minVoltageKV"`
	MaxVoltageKV    *float64 `json:"maxVoltageKV"`
	MaxDefectSizeMM *float64 `json:"maxDefectSizeMM"`
	BlockingLevels  []string `json:"blockingLevels"`
}

func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if err := a.service.Health(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *API) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	items, err := a.service.ListCases(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *API) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var request createCaseRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	// Keep older clients that supplied a single view-only rule usable while the
	// extended contract requires explicit numeric thresholds.
	rules := domain.AcceptanceRuleSet{ID: request.RuleSet.ID, Version: request.RuleSet.Version, Rules: make([]domain.AcceptanceRule, len(request.RuleSet.Rules))}
	for i, input := range request.RuleSet.Rules {
		rule := domain.AcceptanceRule{ID: input.ID, Name: input.Name, RequiredViews: input.RequiredViews, RequiredZones: input.RequiredZones, BlockingLevels: input.BlockingLevels}
		if input.MinVoltageKV == nil {
			rule.MinVoltageKV = 0.1
		} else {
			rule.MinVoltageKV = *input.MinVoltageKV
		}
		if input.MaxVoltageKV == nil {
			rule.MaxVoltageKV = 10000
		} else {
			rule.MaxVoltageKV = *input.MaxVoltageKV
		}
		if input.MaxDefectSizeMM == nil {
			rule.MaxDefectSizeMM = 0.1
		} else {
			rule.MaxDefectSizeMM = *input.MaxDefectSizeMM
		}
		rules.Rules[i] = rule
	}
	p, err := principal(r)
	if err != nil {
		writeError(w, err)
		return
	}
	created, err := a.service.CreateCase(r.Context(), application.CreateCaseCommand{IdempotencyKey: request.IdempotencyKey, Principal: p, WorkpieceCode: request.WorkpieceCode, InspectionZone: request.InspectionZone, Technique: request.Technique, RuleSet: rules})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	item, err := a.service.GetCase(r.Context(), r.PathValue("caseId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
