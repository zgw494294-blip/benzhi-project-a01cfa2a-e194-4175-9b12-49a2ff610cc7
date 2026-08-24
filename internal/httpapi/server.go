package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
)

type API struct{ service *application.Service }

func New(service *application.Service, web http.Handler) http.Handler {
	a := &API{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.HealthHandler)
	mux.HandleFunc("GET /api/cases", a.ListCasesHandler)
	mux.HandleFunc("POST /api/cases", a.CreateCaseHandler)
	mux.HandleFunc("GET /api/cases/{caseId}", a.GetCaseHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/revisions", a.SubmitRevisionHandler)
	mux.HandleFunc("GET /api/cases/{caseId}/revisions/{revisionId}/content", a.DownloadRevisionHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/checks", a.RunCheckHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/findings", a.AddFindingHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/conclusions", a.SetConclusionsHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/retake", a.RequestRetakeHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/findings/{findingId}/close", a.CloseFindingHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/retake/close", a.CloseFindingsHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/freeze", a.FreezeHandler)
	mux.HandleFunc("POST /api/cases/{caseId}/release", a.IssueHandler)
	mux.HandleFunc("GET /api/cases/{caseId}/audit", a.AuditHandler)
	mux.HandleFunc("GET /api/cases/{caseId}/checks", a.CheckHistoryHandler)
	mux.HandleFunc("GET /api/credentials/{number}", a.VerifyCredentialHandler)
	mux.Handle("GET /", web)
	return securityHeaders(requestLog(mux))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		slog.Info("HTTP 请求", "method", r.Method, "path", r.URL.Path, "status", wrapped.status, "duration", time.Since(started))
	})
}
