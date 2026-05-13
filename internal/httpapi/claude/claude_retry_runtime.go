package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	openaifmt "ds2api/internal/format/openai"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/sse"
	streamengine "ds2api/internal/stream"
	"ds2api/internal/toolcall"
	"ds2api/internal/translatorcliproxy"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

type claudeNonStreamResult struct {
	thinking              string
	toolDetectionThinking string
	text                  string
	contentFilter         bool
	parsed                toolcall.ToolCallParseResult
	responseMessageID     int
}

func (h *Handler) handleClaudeNonStreamWithRetry(w http.ResponseWriter, r *http.Request, a *auth.RequestAuth, stdReq claudeNormalizedRequest, payload map[string]any, pow string) {
	attempts := 0
	currentResp := h.executeClaudeRequest(r.Context(), a, payload, pow)
	usagePrompt := stdReq.Standard.FinalPrompt
	accumulatedThinking := ""
	accumulatedToolDetectionThinking := ""

	for {
		result, ok := h.collectClaudeNonStreamAttempt(w, currentResp, usagePrompt, stdReq)
		if !ok {
			return
		}
		accumulatedThinking += sse.TrimContinuationOverlap(accumulatedThinking, result.thinking)
		accumulatedToolDetectionThinking += sse.TrimContinuationOverlap(accumulatedToolDetectionThinking, result.toolDetectionThinking)
		result.thinking = accumulatedThinking
		result.toolDetectionThinking = accumulatedToolDetectionThinking
		result.parsed = detectClaudeToolCalls(result.text, result.thinking, result.toolDetectionThinking, stdReq.Standard.ToolNames)

		if !shouldRetryClaudeNonStream(result, attempts) {
			h.finishClaudeNonStreamResult(w, r, result, attempts, stdReq)
			return
		}

		attempts++
		config.Logger.Info("[claude_empty_retry] attempting synthetic retry", "surface", "claude.messages", "stream", false, "retry_attempt", attempts, "parent_message_id", result.responseMessageID)

		// Rate limit 检测：踢出当前账号 5 分钟
		if strings.Contains(strings.ToLower(result.text), "limit") {
			h.Auth.BanCurrentAccount(a, 5*time.Minute)
			config.Logger.Warn("[claude_empty_retry] rate limit detected, banned account", "account", a.AccountID, "duration", "5m")
		}

		retryPow, powErr := h.DS.GetPow(r.Context(), a, 3)
		if powErr != nil {
			config.Logger.Warn("[claude_empty_retry] retry PoW fetch failed, falling back to original PoW", "surface", "claude.messages", "retry_attempt", attempts, "error", powErr)
			retryPow = pow
		}
		retryPayload := shared.ClonePayloadForEmptyOutputRetry(payload, result.responseMessageID)
		nextResp, err := h.DS.CallCompletion(r.Context(), a, retryPayload, retryPow, 3)
		if err != nil {
			writeClaudeError(w, http.StatusInternalServerError, "Failed to get completion.")
			config.Logger.Warn("[claude_empty_retry] retry request failed", "surface", "claude.messages", "retry_attempt", attempts, "error", err)
			return
		}
		usagePrompt = shared.UsagePromptWithEmptyOutputRetry(stdReq.Standard.FinalPrompt, attempts)
		currentResp = nextResp
	}
}

func (h *Handler) executeClaudeRequest(ctx context.Context, a *auth.RequestAuth, payload map[string]any, pow string) *http.Response {
	resp, err := h.DS.CallCompletion(ctx, a, payload, pow, 3)
	if err != nil {
		return nil
	}
	return resp
}

func (h *Handler) collectClaudeNonStreamAttempt(w http.ResponseWriter, resp *http.Response, usagePrompt string, stdReq claudeNormalizedRequest) (claudeNonStreamResult, bool) {
	if resp == nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)
			writeClaudeError(w, resp.StatusCode, string(body))
		} else {
			writeClaudeError(w, http.StatusInternalServerError, "Failed to get completion.")
		}
		return claudeNonStreamResult{}, false
	}
	result := sse.CollectStream(resp, stdReq.Standard.Thinking, true)
	stripReferenceMarkers := h.compatStripReferenceMarkers()
	sanitizedThinking := shared.CleanVisibleOutput(result.Thinking, stripReferenceMarkers)
	toolDetectionThinking := shared.CleanVisibleOutput(result.ToolDetectionThinking, stripReferenceMarkers)
	sanitizedText := shared.CleanVisibleOutput(result.Text, stripReferenceMarkers)
	return claudeNonStreamResult{
		thinking:              sanitizedThinking,
		toolDetectionThinking: toolDetectionThinking,
		text:                  sanitizedText,
		contentFilter:         result.ContentFilter,
		responseMessageID:     result.ResponseMessageID,
	}, true
}

func (h *Handler) finishClaudeNonStreamResult(w http.ResponseWriter, r *http.Request, result claudeNonStreamResult, attempts int, stdReq claudeNormalizedRequest) {
	// 检查空输出错误
	if len(result.parsed.Calls) == 0 && shared.ShouldWriteUpstreamEmptyOutputError(result.text) {
		status, message, _ := shared.UpstreamEmptyOutputDetail(result.contentFilter, result.text, result.thinking)
		writeClaudeError(w, status, message)
		config.Logger.Info("[claude_empty_retry] terminal empty output", "surface", "claude.messages", "retry_attempts", attempts, "success_source", "none")
		return
	}

	// 构建 OpenAI 格式响应
	model := stdReq.Standard.ResponseModel
	respBody := openaifmt.BuildChatCompletionWithToolCalls(
		"claude_"+strings.ReplaceAll(uuid.NewString(), "-", ""),
		model,
		stdReq.Standard.FinalPrompt,
		result.thinking,
		result.text,
		result.parsed.Calls,
		stdReq.Standard.ToolsRaw,
	)

	// 转换为 Claude 格式
	rawReq := map[string]any{
		"model":    stdReq.Standard.RequestedModel,
		"stream":   false,
		"messages": stdReq.NormalizedMessages,
	}
	rawBytes, _ := json.Marshal(rawReq)
	respBytes, _ := json.Marshal(respBody)
	claudeResp := translatorcliproxy.FromOpenAINonStream(
		sdktranslator.FormatClaude,
		model,
		rawBytes,
		rawBytes,
		respBytes,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(claudeResp)

	source := "first_attempt"
	if attempts > 0 {
		source = "synthetic_retry"
	}
	config.Logger.Info("[claude_empty_retry] completed", "surface", "claude.messages", "retry_attempts", attempts, "success_source", source)
}

func shouldRetryClaudeNonStream(result claudeNonStreamResult, attempts int) bool {
	return shared.EmptyOutputRetryEnabled() &&
		attempts < shared.EmptyOutputRetryMaxAttempts() &&
		!result.contentFilter &&
		len(result.parsed.Calls) == 0 &&
		strings.TrimSpace(result.text) == ""
}

// detectClaudeToolCalls 检测 Claude 格式的工具调用
func detectClaudeToolCalls(text, thinking, toolDetectionThinking string, toolNames []string) toolcall.ToolCallParseResult {
	return shared.DetectAssistantToolCalls(text, thinking, toolDetectionThinking, toolNames)
}

// 流式重试处理
func (h *Handler) handleClaudeStreamWithRetry(w http.ResponseWriter, r *http.Request, a *auth.RequestAuth, stdReq claudeNormalizedRequest, payload map[string]any, pow string) {
	streamRuntime, initialType, ok := h.prepareClaudeStreamRuntime(w, stdReq)
	if !ok {
		return
	}
	attempts := 0
	currentResp := h.executeClaudeRequest(r.Context(), a, payload, pow)

	for {
		terminalWritten, retryable := h.consumeClaudeStreamAttempt(r, currentResp, streamRuntime, initialType, stdReq, attempts < shared.EmptyOutputRetryMaxAttempts())
		if terminalWritten {
			logClaudeStreamTerminal(streamRuntime, attempts)
			return
		}
		if !retryable || !shared.EmptyOutputRetryEnabled() || attempts >= shared.EmptyOutputRetryMaxAttempts() {
			// Rate limit 检测
			if strings.Contains(strings.ToLower(streamRuntime.finalErrorMessage), "limit") {
				h.Auth.BanCurrentAccount(a, 5*time.Minute)
				config.Logger.Warn("[claude_empty_retry] rate limit detected, banned account", "account", a.AccountID, "duration", "5m")
			}
			streamRuntime.finalize("stop")
			config.Logger.Info("[claude_empty_retry] terminal empty output", "surface", "claude.messages", "stream", true, "retry_attempts", attempts, "success_source", "none")
			return
		}
		attempts++
		config.Logger.Info("[claude_empty_retry] attempting synthetic retry", "surface", "claude.messages", "stream", true, "retry_attempt", attempts, "parent_message_id", streamRuntime.responseMessageID)

		retryPow, powErr := h.DS.GetPow(r.Context(), a, 3)
		if powErr != nil {
			config.Logger.Warn("[claude_empty_retry] retry PoW fetch failed, falling back to original PoW", "surface", "claude.messages", "retry_attempt", attempts, "error", powErr)
			retryPow = pow
		}
		nextResp, err := h.DS.CallCompletion(r.Context(), a, shared.ClonePayloadForEmptyOutputRetry(payload, streamRuntime.responseMessageID), retryPow, 3)
		if err != nil {
			streamRuntime.failStream(http.StatusInternalServerError, "Failed to get completion.", "error")
			config.Logger.Warn("[claude_empty_retry] retry request failed", "surface", "claude.messages", "retry_attempt", attempts, "error", err)
			return
		}
		if nextResp.StatusCode != http.StatusOK {
			defer func() { _ = nextResp.Body.Close() }()
			body, _ := io.ReadAll(nextResp.Body)
			streamRuntime.failStream(nextResp.StatusCode, string(body), "error")
			return
		}
		currentResp = nextResp
	}
}

func (h *Handler) prepareClaudeStreamRuntime(w http.ResponseWriter, stdReq claudeNormalizedRequest) (*claudeStreamRuntime, string, bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	_, canFlush := w.(http.Flusher)
	if !canFlush {
		config.Logger.Warn("[claude_stream] response writer does not support flush; streaming may be buffered")
	}

	initialType := "text"
	if stdReq.Standard.Thinking {
		initialType = "thinking"
	}

	streamRuntime := newClaudeStreamRuntime(
		w,
		rc,
		canFlush,
		stdReq.Standard.ResponseModel,
		stdReq.Standard.Messages,
		stdReq.Standard.Thinking,
		stdReq.Standard.Search,
		h.compatStripReferenceMarkers(),
		stdReq.Standard.ToolNames,
		stdReq.Standard.ToolsRaw,
	)
	streamRuntime.sendMessageStart()
	return streamRuntime, initialType, true
}

func (h *Handler) consumeClaudeStreamAttempt(r *http.Request, resp *http.Response, streamRuntime *claudeStreamRuntime, initialType string, stdReq claudeNormalizedRequest, allowDeferEmpty bool) (bool, bool) {
	defer func() { _ = resp.Body.Close() }()
	finalReason := streamengine.StopReason("stop")
	contextCancelled := false

	streamengine.ConsumeSSE(streamengine.ConsumeConfig{
		Context:             r.Context(),
		Body:                resp.Body,
		ThinkingEnabled:     stdReq.Standard.Thinking,
		InitialType:         initialType,
		KeepAliveInterval:   claudeStreamPingInterval,
		IdleTimeout:         claudeStreamIdleTimeout,
		MaxKeepAliveNoInput: claudeStreamMaxKeepaliveCnt,
	}, streamengine.ConsumeHooks{
		OnKeepAlive: streamRuntime.sendPing,
		OnParsed:    streamRuntime.onParsed,
		OnFinalize: func(reason streamengine.StopReason, err error) {
			if string(reason) == "content_filter" {
				finalReason = reason
			}
		},
		OnContextDone: func() {
			contextCancelled = true
		},
	})

	if contextCancelled {
		streamRuntime.failStream(http.StatusRequestTimeout, "Request context cancelled before stream completed.", string(streamengine.StopReasonContextCancelled))
		return true, false
	}

	terminalWritten := streamRuntime.finalizeWithResult(string(finalReason))
	return terminalWritten, !terminalWritten
}

func logClaudeStreamTerminal(streamRuntime *claudeStreamRuntime, attempts int) {
	source := "first_attempt"
	if attempts > 0 {
		source = "synthetic_retry"
	}
	if streamRuntime.failed {
		config.Logger.Info("[claude_empty_retry] terminal error", "surface", "claude.messages", "stream", true, "retry_attempts", attempts, "success_source", "none", "error_code", streamRuntime.finalErrorCode)
		return
	}
	config.Logger.Info("[claude_empty_retry] completed", "surface", "claude.messages", "stream", true, "retry_attempts", attempts, "success_source", source)
}
