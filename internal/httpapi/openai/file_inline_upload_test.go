package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

type inlineUploadDSStub struct {
	mu             sync.Mutex
	uploadCalls    []dsclient.UploadFileRequest
	lastCtx        context.Context
	completionReq  map[string]any
	createSession  string
	uploadErr      error
	uploadDelay    time.Duration
	activeUploads  int
	maxConcurrent  int
	completionResp *http.Response
}

func (m *inlineUploadDSStub) CreateSession(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	if strings.TrimSpace(m.createSession) == "" {
		return "session-id", nil
	}
	return m.createSession, nil
}

func (m *inlineUploadDSStub) GetPow(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "pow", nil
}

func (m *inlineUploadDSStub) UploadFile(ctx context.Context, _ *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	m.mu.Lock()
	m.lastCtx = ctx
	m.uploadCalls = append(m.uploadCalls, req)
	callID := len(m.uploadCalls)
	m.activeUploads++
	if m.activeUploads > m.maxConcurrent {
		m.maxConcurrent = m.activeUploads
	}
	delay := m.uploadDelay
	uploadErr := m.uploadErr
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			m.mu.Lock()
			m.activeUploads--
			m.mu.Unlock()
			return nil, ctx.Err()
		}
	}

	m.mu.Lock()
	m.activeUploads--
	m.mu.Unlock()
	if uploadErr != nil {
		return nil, uploadErr
	}
	return &dsclient.UploadFileResult{
		ID:       fmt.Sprintf("file-inline-%d", callID),
		Filename: req.Filename,
		Bytes:    int64(len(req.Data)),
		Status:   "uploaded",
		Purpose:  req.Purpose,
	}, nil
}

func (m *inlineUploadDSStub) CallCompletion(_ context.Context, _ *auth.RequestAuth, payload map[string]any, _ string, _ int) (*http.Response, error) {
	m.completionReq = payload
	if m.completionResp != nil {
		return m.completionResp, nil
	}
	return makeOpenAISSEHTTPResponse(
		`data: {"p":"response/content","v":"ok"}`,
		`data: [DONE]`,
	), nil
}

func (m *inlineUploadDSStub) DeleteSessionForToken(_ context.Context, _ string, _ string) (*dsclient.DeleteSessionResult, error) {
	return &dsclient.DeleteSessionResult{Success: true}, nil
}

func (m *inlineUploadDSStub) DeleteAllSessionsForToken(_ context.Context, _ string) error {
	return nil
}

func TestPreprocessInlineFileInputsReplacesDataURLAndCollectsRefFileIDs(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{DS: ds}
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":      "image_url",
						"image_url": map[string]any{"url": "data:image/png;base64,QUJDRA=="},
					},
				},
			},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := h.preprocessInlineFileInputs(ctx, &auth.RequestAuth{DeepSeekToken: "token"}, req); err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(ds.uploadCalls))
	}
	if ds.lastCtx != ctx {
		t.Fatalf("expected upload to use request context")
	}
	if ds.uploadCalls[0].ContentType != "image/png" {
		t.Fatalf("expected image/png, got %q", ds.uploadCalls[0].ContentType)
	}
	if ds.uploadCalls[0].Filename != "image.png" {
		t.Fatalf("expected inferred filename image.png, got %q", ds.uploadCalls[0].Filename)
	}
	messages, _ := req["messages"].([]any)
	first, _ := messages[0].(map[string]any)
	content, _ := first["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["type"] != "input_image" {
		t.Fatalf("expected input_image replacement, got %#v", block)
	}
	if block["file_id"] != "file-inline-1" {
		t.Fatalf("expected file-inline-1 replacement id, got %#v", block)
	}
	refIDs, _ := req["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-inline-1" {
		t.Fatalf("unexpected ref_file_ids: %#v", req["ref_file_ids"])
	}
}

func TestPreprocessInlineFileInputsUploadsUniqueFilesConcurrently(t *testing.T) {
	ds := &inlineUploadDSStub{uploadDelay: 25 * time.Millisecond}
	h := &openAITestSurface{DS: ds}
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,QUJDRA=="}},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,RUZHSA=="}},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,QUJDRA=="}},
				},
			},
		},
	}

	if err := h.preprocessInlineFileInputs(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, req); err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 2 {
		t.Fatalf("expected 2 unique uploads, got %d", len(ds.uploadCalls))
	}
	if ds.maxConcurrent < 2 {
		t.Fatalf("expected concurrent uploads, max concurrency was %d", ds.maxConcurrent)
	}
	messages, _ := req["messages"].([]any)
	first, _ := messages[0].(map[string]any)
	content, _ := first["content"].([]any)
	firstBlock, _ := content[0].(map[string]any)
	secondBlock, _ := content[1].(map[string]any)
	thirdBlock, _ := content[2].(map[string]any)
	if firstBlock["file_id"] == "" || thirdBlock["file_id"] != firstBlock["file_id"] {
		t.Fatalf("expected deduped first/third file ids, got %#v %#v", firstBlock["file_id"], thirdBlock["file_id"])
	}
	if secondBlock["file_id"] == "" || secondBlock["file_id"] == firstBlock["file_id"] {
		t.Fatalf("expected distinct second file id, got %#v first=%#v", secondBlock["file_id"], firstBlock["file_id"])
	}
	refIDs, _ := req["ref_file_ids"].([]any)
	if len(refIDs) != 2 || refIDs[0] != firstBlock["file_id"] || refIDs[1] != secondBlock["file_id"] {
		t.Fatalf("unexpected ref_file_ids order: %#v", req["ref_file_ids"])
	}
}

func TestPreprocessInlineFileInputsUploadFailureDoesNotMutateRequest(t *testing.T) {
	ds := &inlineUploadDSStub{uploadErr: errors.New("boom")}
	h := &openAITestSurface{DS: ds}
	originalBlock := map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,QUJDRA=="}}
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": []any{originalBlock},
			},
		},
	}

	if err := h.preprocessInlineFileInputs(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, req); err == nil {
		t.Fatal("expected preprocess failure")
	}
	messages, _ := req["messages"].([]any)
	first, _ := messages[0].(map[string]any)
	content, _ := first["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["type"] != "image_url" {
		t.Fatalf("expected original image_url block after failed upload, got %#v", block)
	}
	if _, ok := block["file_id"]; ok {
		t.Fatalf("did not expect replacement file_id after failed upload: %#v", block)
	}
	if _, ok := req["ref_file_ids"]; ok {
		t.Fatalf("did not expect ref_file_ids after failed upload: %#v", req["ref_file_ids"])
	}
}

func TestPreprocessInlineFileInputsDeduplicatesIdenticalPayloads(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{DS: ds}
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,QUJDRA=="}},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,QUJDRA=="}},
				},
			},
		},
	}

	if err := h.preprocessInlineFileInputs(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, req); err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected deduplicated single upload, got %d", len(ds.uploadCalls))
	}
	refIDs, _ := req["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-inline-1" {
		t.Fatalf("unexpected ref_file_ids after dedupe: %#v", req["ref_file_ids"])
	}
}

func TestChatCompletionsUploadsInlineFilesBeforeCompletion(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{wideInput: true}, Auth: streamStatusAuthStub{}, DS: ds}
	reqBody := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":[{"type":"input_text","text":"hi"},{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJDRA=="}}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(ds.uploadCalls))
	}
	if ds.completionReq == nil {
		t.Fatal("expected completion payload to be captured")
	}
	refIDs, _ := ds.completionReq["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-inline-1" {
		t.Fatalf("unexpected completion ref_file_ids: %#v", ds.completionReq["ref_file_ids"])
	}
}

func TestResponsesUploadsInlineFilesBeforeCompletion(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{wideInput: true}, Auth: streamStatusAuthStub{}, DS: ds}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)
	reqBody := `{"model":"deepseek-v4-flash","input":[{"role":"user","content":[{"type":"input_text","text":"hi"},{"type":"input_image","image_url":{"url":"data:image/png;base64,QUJDRA=="}}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(ds.uploadCalls))
	}
	refIDs, _ := ds.completionReq["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-inline-1" {
		t.Fatalf("unexpected completion ref_file_ids: %#v", ds.completionReq["ref_file_ids"])
	}
}

func TestChatCompletionsInlineUploadFailureReturnsBadRequest(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{wideInput: true}, Auth: streamStatusAuthStub{}, DS: ds}
	reqBody := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%%%"}}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ds.completionReq != nil {
		t.Fatalf("did not expect completion call on upload decode error")
	}
}

func TestResponsesInlineUploadFailureReturnsInternalServerError(t *testing.T) {
	ds := &inlineUploadDSStub{uploadErr: errors.New("boom")}
	h := &openAITestSurface{Store: mockOpenAIConfig{wideInput: true}, Auth: streamStatusAuthStub{}, DS: ds}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)
	reqBody := `{"model":"deepseek-v4-flash","input":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJDRA=="}}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ds.completionReq != nil {
		t.Fatalf("did not expect completion call after upload failure")
	}
}

func TestVercelPrepareUploadsInlineFilesBeforeLeasePayload(t *testing.T) {
	t.Setenv("VERCEL", "1")
	t.Setenv("DS2API_VERCEL_INTERNAL_SECRET", "stream-secret")
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{wideInput: true}, Auth: streamStatusAuthStub{}, DS: ds}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)
	reqBody := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":[{"type":"input_text","text":"hi"},{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJDRA=="}}]}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?__stream_prepare=1", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("X-Ds2-Internal-Token", "stream-secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(ds.uploadCalls))
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, rec.Body.String())
	}
	payload, _ := out["payload"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected payload in prepare response, got %#v", out)
	}
	refIDs, _ := payload["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-inline-1" {
		t.Fatalf("unexpected payload ref_file_ids: %#v", payload["ref_file_ids"])
	}
}
