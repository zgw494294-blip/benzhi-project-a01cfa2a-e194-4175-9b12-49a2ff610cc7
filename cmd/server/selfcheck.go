package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type selfcheckClient struct {
	baseURL string
	client  *http.Client
}

func runSelfcheck(parent context.Context, address string) error {
	dataDir, err := os.MkdirTemp("", "benzhi-rt-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	rt, err := buildRuntime(parent, address, dataDir)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		rt.repository.Close()
		return fmt.Errorf("selfcheck 监听失败: %w", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- rt.server.Serve(listener) }()
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	client := &selfcheckClient{baseURL: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 5 * time.Second}}
	flowErr := client.execute(ctx)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	closeErr := rt.close(shutdownCtx)
	serveErr := <-serveDone
	if serveErr != nil && serveErr != http.ErrServerClosed {
		return serveErr
	}
	if flowErr != nil {
		return flowErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("selfcheck 通过：真实 HTTP 流程已完成，放行凭据摘要有效")
	return nil
}

func (c *selfcheckClient) execute(ctx context.Context) error {
	if err := c.health(ctx); err != nil {
		return err
	}
	created, err := c.createCase(ctx)
	if err != nil {
		return err
	}
	payload := []byte("RT-FILM\x00SELF-CHECK\x01COMPLETE-EVIDENCE")
	withRevision, err := c.upload(ctx, created, payload)
	if err != nil {
		return err
	}
	checked, err := c.postCase(ctx, withRevision.ID, "checks", "reviewer", "selfcheck-reviewer", map[string]any{"expectedVersion": withRevision.Version, "idempotencyKey": "selfcheck-check-0001"})
	if err != nil {
		return err
	}
	if !checked.LastCheckPassed || checked.Status != domain.StatusPendingReview {
		return fmt.Errorf("完整性检查未通过")
	}
	concluded, err := c.postCase(ctx, checked.ID, "conclusions", "reviewer", "selfcheck-reviewer", map[string]any{"expectedVersion": checked.Version, "idempotencyKey": "selfcheck-conclusion-0001", "conclusions": []map[string]any{{"ruleId": "R-SELF", "conclusion": "pass", "basis": "候选底片覆盖完整且曝光参数在范围内"}}})
	if err != nil {
		return err
	}
	frozen, err := c.postCase(ctx, concluded.ID, "freeze", "reviewer", "selfcheck-reviewer", map[string]any{"expectedVersion": concluded.Version, "idempotencyKey": "selfcheck-freeze-0001"})
	if err != nil {
		return err
	}
	if frozen.Frozen == nil || !frozen.Frozen.Verify() {
		return fmt.Errorf("冻结证据校验失败")
	}
	credential, err := c.issue(ctx, frozen)
	if err != nil {
		return err
	}
	return c.verify(ctx, credential.CredentialNumber)
}

func (c *selfcheckClient) health(ctx context.Context) error {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("健康检查返回 %d", response.StatusCode)
	}
	return nil
}

func (c *selfcheckClient) createCase(ctx context.Context) (*domain.InspectionCase, error) {
	payload := map[string]any{"idempotencyKey": "selfcheck-create-0001", "workpieceCode": "SELF-WP-001", "inspectionZone": "WELD-A", "techniqueParameters": map[string]any{"sourceType": "X-ray", "voltageKV": 180, "currentMA": 5, "exposureSeconds": 2, "sourceDistanceMM": 600}, "acceptanceRuleSet": map[string]any{"id": "SELF-RULES", "version": 1, "rules": []map[string]any{{"id": "R-SELF", "name": "自检覆盖规则", "requiredViews": []string{"FRONT"}, "requiredZones": []string{"WELD-A"}, "minVoltageKV": 150, "maxVoltageKV": 220, "maxDefectSizeMM": 2, "blockingLevels": []string{"blocking"}}}}}
	var result domain.InspectionCase
	if err := c.jsonRequest(ctx, http.MethodPost, "/api/cases", "operator", "selfcheck-operator", payload, &result, http.StatusCreated); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *selfcheckClient) upload(ctx context.Context, item *domain.InspectionCase, payload []byte) (*domain.InspectionCase, error) {
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	metadata := map[string]any{"expectedVersion": item.Version, "idempotencyKey": "selfcheck-upload-0001", "captureBatch": "SELF-BATCH", "viewCode": "FRONT", "coveredZone": "WELD-A", "exposureParameters": map[string]any{"voltageKV": 180, "currentMA": 5, "exposureSeconds": 2, "sourceDistanceMM": 600}, "contentDigest": digest, "supersedesRevisionId": ""}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	encoded, _ := json.Marshal(metadata)
	_ = writer.WriteField("metadata", string(encoded))
	part, err := writer.CreateFormFile("file", "selfcheck.rt")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/cases/"+url.PathEscape(item.ID)+"/revisions", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Actor", "selfcheck-operator")
	request.Header.Set("X-Role", "operator")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("上传底片失败(%d): %s", response.StatusCode, data)
	}
	var result domain.InspectionCase
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *selfcheckClient) postCase(ctx context.Context, caseID, suffix, role, actor string, payload any) (*domain.InspectionCase, error) {
	var result domain.InspectionCase
	err := c.jsonRequest(ctx, http.MethodPost, "/api/cases/"+url.PathEscape(caseID)+"/"+suffix, role, actor, payload, &result, http.StatusOK)
	return &result, err
}

func (c *selfcheckClient) issue(ctx context.Context, item *domain.InspectionCase) (*domain.ReleaseCredential, error) {
	payload := map[string]any{"expectedVersion": item.Version, "idempotencyKey": "selfcheck-issue-0001", "frozenVersion": item.Frozen.FrozenVersion}
	var result domain.ReleaseCredential
	err := c.jsonRequest(ctx, http.MethodPost, "/api/cases/"+url.PathEscape(item.ID)+"/release", "quality", "selfcheck-quality", payload, &result, http.StatusCreated)
	return &result, err
}

func (c *selfcheckClient) verify(ctx context.Context, number string) error {
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := c.jsonRequest(ctx, http.MethodGet, "/api/credentials/"+url.PathEscape(number), "operator", "selfcheck-auditor", nil, &result, http.StatusOK); err != nil {
		return err
	}
	if !result.Valid {
		return fmt.Errorf("凭据验证结果无效")
	}
	return nil
}

func (c *selfcheckClient) jsonRequest(ctx context.Context, method, path, role, actor string, payload, result any, expectedStatus int) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Role", role)
	request.Header.Set("X-Actor", actor)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		data, _ := io.ReadAll(response.Body)
		return fmt.Errorf("%s %s 返回 %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(response.Body).Decode(result)
}
