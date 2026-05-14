package config

import (
	"os"
	"testing"
)

func TestContainerDefaultConfigPath(t *testing.T) {
	t.Run("fallback to /app when /data is missing", func(t *testing.T) {
		// This test environment does not guarantee a writable/mounted /data.
		// If /data is absent we must keep /app fallback to avoid persistence failures.
		if _, err := os.Stat("/data"); err == nil {
			t.Skip("/data exists in this environment; cannot validate missing-/data fallback")
		}
		if got := containerDefaultConfigPath(); got != "/app/config.json" {
			t.Fatalf("containerDefaultConfigPath() = %q, want %q", got, "/app/config.json")
		}
	})

	t.Run("prefer /data when /data directory exists", func(t *testing.T) {
		if _, err := os.Stat("/data"); err != nil {
			t.Skip("/data does not exist in this environment")
		}
		if got := containerDefaultConfigPath(); got != "/data/config.json" {
			t.Fatalf("containerDefaultConfigPath() = %q, want %q", got, "/data/config.json")
		}
	})
}

func TestChatHistoryPathUsesTmpOnVercelByDefault(t *testing.T) {
	t.Setenv("VERCEL", "1")
	t.Setenv("NOW_REGION", "")
	t.Setenv("DS2API_CHAT_HISTORY_PATH", "")

	if got := ChatHistoryPath(); got != "/tmp/ds2api/chat_history.json" {
		t.Fatalf("ChatHistoryPath() = %q, want %q", got, "/tmp/ds2api/chat_history.json")
	}
}

func TestChatHistoryPathPrefersExplicitEnvOnVercel(t *testing.T) {
	t.Setenv("VERCEL", "1")
	t.Setenv("NOW_REGION", "")
	t.Setenv("DS2API_CHAT_HISTORY_PATH", `C:\custom\chat_history.json`)

	if got := ChatHistoryPath(); got != `C:\custom\chat_history.json` {
		t.Fatalf("ChatHistoryPath() = %q, want %q", got, `C:\custom\chat_history.json`)
	}
}
