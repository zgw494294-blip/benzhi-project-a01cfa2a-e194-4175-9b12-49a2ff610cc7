package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

func testHandler(t *testing.T) (http.Handler, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := store.OpenSQLite(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := store.NewFilePayloadStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, payloads, application.SystemClock{}, &store.RandomIDGenerator{})
	return New(service, http.NotFoundHandler()), func() { repo.Close() }
}

func TestCreateCaseProtocolAndVersionConflict(t *testing.T) {
	handler, closeFn := testHandler(t)
	defer closeFn()
	payload := map[string]any{"idempotencyKey": "http-create-0001", "workpieceCode": "WP-HTTP", "inspectionZone": "WELD-A", "techniqueParameters": map[string]any{"sourceType": "X-ray", "voltageKV": 180, "exposureSeconds": 2}, "acceptanceRuleSet": map[string]any{"id": "RULES", "version": 1, "rules": []map[string]any{{"id": "R1", "name": "规则", "requiredViews": []string{"FRONT"}, "requiredZones": []string{"WELD-A"}}}}}
	encoded, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/api/cases", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "操作员")
	request.Header.Set("X-Role", "operator")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("建档返回 %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	checkPayload := map[string]any{"expectedVersion": created.Version + 1, "idempotencyKey": "http-check-0001"}
	encoded, _ = json.Marshal(checkPayload)
	request = httptest.NewRequest(http.MethodPost, "/api/cases/"+created.ID+"/checks", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "判读员")
	request.Header.Set("X-Role", "reviewer")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("错误版本预期 409，得到 %d: %s", response.Code, response.Body.String())
	}
}

func TestRejectsUnknownJSONAndMissingRole(t *testing.T) {
	handler, closeFn := testHandler(t)
	defer closeFn()
	request := httptest.NewRequest(http.MethodPost, "/api/cases", bytes.NewBufferString(`{"idempotencyKey":"unknown-json-0001","unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("未知字段应返回 400，得到 %d", response.Code)
	}
}
