package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func TestStopStreamPostsAppCompatiblePayload(t *testing.T) {
	var seenURL string
	var seenPayload map[string]any
	var seenToken string
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			seenURL = req.URL.String()
			seenToken = req.Header.Get("authorization")
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if err := json.Unmarshal(body, &seenPayload); err != nil {
				t.Fatalf("decode stop payload: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"","data":{"biz_code":0,"biz_msg":"","biz_data":null}}`)),
				Request:    req,
			}, nil
		}),
		fallback: &http.Client{},
	}

	err := client.StopStream(context.Background(), &auth.RequestAuth{
		DeepSeekToken: "token-123",
		AccountID:     "acct",
	}, "session-123", 2, 1)
	if err != nil {
		t.Fatalf("StopStream failed: %v", err)
	}
	if seenURL != dsprotocol.DeepSeekStopStreamURL {
		t.Fatalf("stop stream url=%q want=%q", seenURL, dsprotocol.DeepSeekStopStreamURL)
	}
	if seenToken != "Bearer token-123" {
		t.Fatalf("authorization=%q want bearer token", seenToken)
	}
	if got := seenPayload["chat_session_id"]; got != "session-123" {
		t.Fatalf("chat_session_id=%#v want session-123", got)
	}
	if got := intFrom(seenPayload["message_id"]); got != 2 {
		t.Fatalf("message_id=%#v want 2", seenPayload["message_id"])
	}
}
