package claude

import (
	"context"
	"net/http"
	"time"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
)

type AuthResolver interface {
	Determine(req *http.Request) (*auth.RequestAuth, error)
	Release(a *auth.RequestAuth)
	BanCurrentAccount(a *auth.RequestAuth, duration time.Duration)
}

type DeepSeekCaller interface {
	CreateSession(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error)
	GetPow(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error)
	CallCompletion(ctx context.Context, a *auth.RequestAuth, payload map[string]any, powResp string, maxAttempts int) (*http.Response, error)
}

type ConfigReader interface {
	ModelAliases() map[string]string
	CompatStripReferenceMarkers() bool
}

type OpenAIChatRunner interface {
	ChatCompletions(w http.ResponseWriter, r *http.Request)
}

var _ AuthResolver = (*auth.Resolver)(nil)
var _ DeepSeekCaller = (*dsclient.Client)(nil)
var _ ConfigReader = (*config.Store)(nil)
