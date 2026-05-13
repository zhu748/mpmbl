package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatStreamKeepAliveEmitsEmptyChoiceDataFrame(t *testing.T) {
	rec := httptest.NewRecorder()
	runtime := newChatStreamRuntime(
		rec,
		http.NewResponseController(rec),
		true,
		"chatcmpl-test",
		time.Now().Unix(),
		"deepseek-v4-flash",
		"prompt",
		false,
		false,
		true,
		nil,
		nil,
		false,
		false,
	)

	runtime.sendKeepAlive()

	body := rec.Body.String()
	if body != ": keep-alive\n\n" {
		t.Fatalf("expected keep-alive comment only, got %q", body)
	}
	frames, done := parseSSEDataFrames(t, body)
	if done {
		t.Fatalf("keep-alive must not emit [DONE], body=%q", body)
	}
	if len(frames) != 0 {
		t.Fatalf("expected no data frames, got %d body=%q", len(frames), body)
	}
}

func TestChatStreamFinalizeSendsEmptyOutputError(t *testing.T) {
	rec := httptest.NewRecorder()
	runtime := newChatStreamRuntime(
		rec,
		http.NewResponseController(rec),
		true,
		"chatcmpl-test",
		time.Now().Unix(),
		"deepseek-v4-flash",
		"prompt",
		false,
		false,
		true,
		[]string{"Write"},
		nil,
		true,
		false,
	)

	if !runtime.finalize("stop", false) {
		t.Fatalf("expected terminal error to be written")
	}
	if runtime.finalErrorCode != "upstream_empty_output" {
		t.Fatalf("expected upstream_empty_output, got %q body=%s", runtime.finalErrorCode, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "empty output") {
		t.Fatalf("expected empty output error in stream body, got %s", rec.Body.String())
	}
}
