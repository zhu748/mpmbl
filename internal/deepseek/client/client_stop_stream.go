package client

import (
	"context"
	dsprotocol "ds2api/internal/deepseek/protocol"
	"errors"
	"net/http"
	"strings"

	"ds2api/internal/auth"
	"ds2api/internal/config"
)

// StopStream mirrors the Android client's explicit pause request.
// It is best-effort for downstream disconnects; callers should log failures
// but must not fail an already-finished client response because of them.
func (c *Client) StopStream(ctx context.Context, a *auth.RequestAuth, sessionID string, messageID int, maxAttempts int) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || messageID <= 0 {
		return errors.New("missing stop stream identifiers")
	}
	if maxAttempts <= 0 {
		maxAttempts = c.maxRetries
	}
	clients := c.requestClientsForAuth(ctx, a)
	payload := map[string]any{
		"chat_session_id": sessionID,
		"message_id":      messageID,
	}
	for attempts := 0; attempts < maxAttempts; attempts++ {
		headers := c.authHeadersForAuth(a)
		resp, status, err := c.postJSONWithStatus(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekStopStreamURL, headers, payload)
		if err != nil {
			config.Logger.Warn("[stop_stream] request error", "error", err, "account", accountHeaderSeed(a), "session_id", sessionID, "message_id", messageID)
			continue
		}
		code, bizCode, msg, bizMsg := extractResponseStatus(resp)
		if status == http.StatusOK && code == 0 && bizCode == 0 {
			return nil
		}
		config.Logger.Warn("[stop_stream] failed", "status", status, "code", code, "biz_code", bizCode, "msg", msg, "biz_msg", bizMsg, "account", accountHeaderSeed(a), "session_id", sessionID, "message_id", messageID)
	}
	return errors.New("stop stream failed")
}
