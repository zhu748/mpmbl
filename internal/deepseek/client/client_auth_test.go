package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/config"
)

func TestExtractCreateSessionIDSupportsLegacyShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "legacy-session-id",
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "legacy-session-id" {
		t.Fatalf("expected legacy session id, got %q", got)
	}
}

func TestExtractCreateSessionIDSupportsNestedChatSessionShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"chat_session": map[string]any{
					"id":         "nested-session-id",
					"model_type": "default",
				},
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "nested-session-id" {
		t.Fatalf("expected nested session id, got %q", got)
	}
}

func TestLoginUsesConfiguredDeviceID(t *testing.T) {
	var payload map[string]any
	client := &Client{
		regular:  loginCaptureDoer(t, &payload),
		fallback: &http.Client{},
	}

	_, err := client.Login(context.Background(), config.Account{
		Email:    "user@example.com",
		Password: "secret",
		DeviceID: "custom-device-id",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if got, _ := payload["device_id"].(string); got != "custom-device-id" {
		t.Fatalf("device_id=%q want custom-device-id", got)
	}
	if got, _ := payload["os"].(string); got != "android" {
		t.Fatalf("os=%q want android", got)
	}
}

func TestLoginDerivesStableDeviceIDPerAccount(t *testing.T) {
	first := loginDeviceID(config.Account{Email: "user@example.com"})
	second := loginDeviceID(config.Account{Email: " user@example.com "})
	other := loginDeviceID(config.Account{Email: "other@example.com"})

	if first == "" {
		t.Fatal("expected derived device id")
	}
	if first != second {
		t.Fatalf("expected stable derived id, got %q and %q", first, second)
	}
	if first == other {
		t.Fatalf("expected different accounts to get different ids, got %q", first)
	}
}

func loginCaptureDoer(t *testing.T, payload *map[string]any) doerFunc {
	t.Helper()
	return doerFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, payload); err != nil {
			t.Fatalf("decode login payload %q: %v", string(body), err)
		}
		resp := `{"code":0,"msg":"ok","data":{"biz_code":0,"biz_msg":"ok","biz_data":{"user":{"token":"token-123"}}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(resp)),
			Request:    req,
		}, nil
	})
}
