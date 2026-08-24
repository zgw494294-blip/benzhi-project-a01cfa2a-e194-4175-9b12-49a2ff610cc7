package httpapi

import (
	"net/http"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type addFindingRequest struct {
	commandEnvelope
	Findings      []application.FindingInput `json:"findings"`
	Conclusions   []domain.RuleConclusion    `json:"conclusions"`
	RevisionID    string                     `json:"revisionId"`
	FindingType   string                     `json:"findingType"`
	Location      string                     `json:"location"`
	MeasuredSize  float64                    `json:"measuredSize"`
	Severity      string                     `json:"severity"`
	RuleReference string                     `json:"ruleReference"`
	Basis         string                     `json:"basis"`
	Disposition   string                     `json:"disposition"`
}
type conclusionsRequest struct {
	commandEnvelope
	Conclusions []domain.RuleConclusion `json:"conclusions"`
}
type retakeRequest struct {
	commandEnvelope
	Requirement string               `json:"requirement"`
	Items       []domain.RetakeIssue `json:"items"`
}
type closeFindingRequest struct {
	commandEnvelope
	ClosureNote       string   `json:"closureNote"`
	FindingIDs        []string `json:"findingIds"`
	VerificationBasis string   `json:"verificationBasis"`
}
type issueRequest struct {
	commandEnvelope
	FrozenVersion int64 `json:"frozenVersion"`
}

func (a *API) RunCheckHandler(w http.ResponseWriter, r *http.Request) {
	var request commandEnvelope
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.RunCheck(r.Context(), r.PathValue("caseId"), m)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) AddFindingHandler(w http.ResponseWriter, r *http.Request) {
	var request addFindingRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request.commandEnvelope)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(request.Findings) == 0 {
		request.Findings = []application.FindingInput{{RevisionID: request.RevisionID, FindingType: request.FindingType, Location: request.Location, MeasuredSize: request.MeasuredSize, Severity: request.Severity, RuleReference: request.RuleReference, Basis: request.Basis, Disposition: request.Disposition}}
	}
	result, err := a.service.AddFindings(r.Context(), application.AddFindingsCommand{Meta: m, CaseID: r.PathValue("caseId"), Findings: request.Findings, Conclusions: request.Conclusions})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) SetConclusionsHandler(w http.ResponseWriter, r *http.Request) {
	var request conclusionsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request.commandEnvelope)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.SetConclusions(r.Context(), application.SetConclusionsCommand{Meta: m, CaseID: r.PathValue("caseId"), Conclusions: request.Conclusions})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) RequestRetakeHandler(w http.ResponseWriter, r *http.Request) {
	var request retakeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request.commandEnvelope)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.RequestRetake(r.Context(), application.RetakeCommand{Meta: m, CaseID: r.PathValue("caseId"), Requirement: request.Requirement, Items: request.Items})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) CloseFindingsHandler(w http.ResponseWriter, r *http.Request) {
	var request closeFindingRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request.commandEnvelope)
	if err != nil {
		writeError(w, err)
		return
	}
	ids := request.FindingIDs
	if len(ids) == 0 && r.PathValue("findingId") != "" {
		ids = []string{r.PathValue("findingId")}
	}
	basis := request.VerificationBasis
	if basis == "" {
		basis = request.ClosureNote
	}
	result, err := a.service.CloseFindings(r.Context(), application.CloseFindingsCommand{Meta: m, CaseID: r.PathValue("caseId"), FindingIDs: ids, VerificationBasis: basis})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) CloseFindingHandler(w http.ResponseWriter, r *http.Request) {
	var request closeFindingRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request.commandEnvelope)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.CloseFinding(r.Context(), application.CloseFindingCommand{Meta: m, CaseID: r.PathValue("caseId"), FindingID: r.PathValue("findingId"), ClosureNote: request.ClosureNote})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	var request commandEnvelope
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.Freeze(r.Context(), application.FreezeCommand{Meta: m, CaseID: r.PathValue("caseId")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) IssueHandler(w http.ResponseWriter, r *http.Request) {
	var request issueRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, err)
		return
	}
	m, err := meta(r, request.commandEnvelope)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.Issue(r.Context(), application.IssueCommand{Meta: m, CaseID: r.PathValue("caseId"), FrozenVersion: request.FrozenVersion})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
