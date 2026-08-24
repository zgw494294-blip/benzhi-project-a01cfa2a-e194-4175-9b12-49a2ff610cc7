package httpapi

import "net/http"

func (a *API) AuditHandler(w http.ResponseWriter, r *http.Request) {
	items, err := a.service.AuditTrail(r.Context(), r.PathValue("caseId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *API) CheckHistoryHandler(w http.ResponseWriter, r *http.Request) {
	items, err := a.service.CheckHistory(r.Context(), r.PathValue("caseId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *API) VerifyCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.VerifyCredential(r.Context(), r.PathValue("number"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
