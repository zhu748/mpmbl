package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/httpapi/openai/chat"
)

type claudeCurrentInputAuth struct{}

func (claudeCurrentInputAuth) Determine(*http.Request) (*auth.RequestAuth, error) {
	return &auth.RequestAuth{
		DeepSeekToken: "direct-token",
		CallerID:      "caller:test",
		TriedAccounts: map[string]bool{},
	}, nil
}

func (a claudeCurrentInputAuth) DetermineCaller(req *http.Request) (*auth.RequestAuth, error) {
	return a.Determine(req)
}

func (claudeCurrentInputAuth) Release(*auth.RequestAuth) {}

func (claudeCurrentInputAuth) BanCurrentAccount(*auth.RequestAuth, time.Duration) {}

type claudeCurrentInputDS struct {
	uploads []dsclient.UploadFileRequest
	payload map[string]any
}

func (d *claudeCurrentInputDS) CreateSession(context.Context, *auth.RequestAuth, int) (string, error) {
	return "session-id", nil
}

func (d *claudeCurrentInputDS) GetPow(context.Context, *auth.RequestAuth, int) (string, error) {
	return "pow", nil
}

func (d *claudeCurrentInputDS) UploadFile(_ context.Context, _ *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	d.uploads = append(d.uploads, req)
	return &dsclient.UploadFileResult{ID: "file-claude-history"}, nil
}

func (d *claudeCurrentInputDS) CallCompletion(_ context.Context, _ *auth.RequestAuth, payload map[string]any, _ string, _ int) (*http.Response, error) {
	d.payload = payload
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("data: {\"p\":\"response/content\",\"v\":\"ok\"}\n")),
	}, nil
}

func (d *claudeCurrentInputDS) DeleteSessionForToken(context.Context, string, string) (*dsclient.DeleteSessionResult, error) {
	return &dsclient.DeleteSessionResult{}, nil
}

func (d *claudeCurrentInputDS) DeleteAllSessionsForToken(context.Context, string) error {
	return nil
}

type claudeCurrentInputStore struct {
	mockClaudeConfig
}

func (s claudeCurrentInputStore) CompatWideInputStrictOutput() bool { return false }
func (s claudeCurrentInputStore) ToolcallMode() string { return "" }
func (s claudeCurrentInputStore) ToolcallEarlyEmitConfidence() string { return "" }
func (s claudeCurrentInputStore) ResponsesStoreTTLSeconds() int { return 0 }
func (s claudeCurrentInputStore) EmbeddingsProvider() string { return "" }
func (s claudeCurrentInputStore) AutoDeleteMode() string { return "none" }
func (s claudeCurrentInputStore) AutoDeleteSessions() bool { return false }
func (s claudeCurrentInputStore) HistorySplitEnabled() bool { return false }
func (s claudeCurrentInputStore) HistorySplitTriggerAfterTurns() int { return 0 }
func (s claudeCurrentInputStore) CurrentInputFileEnabled() bool { return true }
func (s claudeCurrentInputStore) CurrentInputFileMinChars() int { return 0 }
func (s claudeCurrentInputStore) ThinkingInjectionEnabled() bool { return false }
func (s claudeCurrentInputStore) ThinkingInjectionPrompt() string { return "" }

func TestClaudeDirectAppliesCurrentInputFile(t *testing.T) {
	ds := &claudeCurrentInputDS{}
	store := claudeCurrentInputStore{mockClaudeConfig{aliases: map[string]string{"claude-sonnet-4-6": "deepseek-v4-flash"}}}
	h := &Handler{
		Store: store,
		OpenAI: &chat.Handler{
			Store: store,
			Auth:  claudeCurrentInputAuth{},
			DS:    ds,
		},
	}
	reqBody := `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello from claude"}],"max_tokens":1024}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Messages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(ds.uploads) != 1 {
		t.Fatalf("expected one current input upload, got %d", len(ds.uploads))
	}
	if ds.uploads[0].Filename != "IGNORE.txt" {
		t.Fatalf("unexpected upload filename: %q", ds.uploads[0].Filename)
	}
	refIDs, _ := ds.payload["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-claude-history" {
		t.Fatalf("expected uploaded history ref id, got %#v", ds.payload["ref_file_ids"])
	}
	prompt, _ := ds.payload["prompt"].(string)
	if !strings.Contains(prompt, "Answer the latest user request directly.") {
		t.Fatalf("expected continuation prompt, got %q", prompt)
	}
}
