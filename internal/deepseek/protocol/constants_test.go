package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSharedConstantsLoaded(t *testing.T) {
	cfg := sharedConstants{}
	if err := json.Unmarshal(sharedConstantsJSON, &cfg); err != nil {
		t.Fatalf("failed to parse shared constants: %v", err)
	}
	client := normalizeClientConstants(cfg.Client)
	if ClientVersion != client.Version {
		t.Fatalf("unexpected client version=%q", ClientVersion)
	}
	wantUserAgent := client.Name + "/" + client.Version + " Android/" + client.AndroidAPILevel
	if BaseHeaders["User-Agent"] != wantUserAgent {
		t.Fatalf("unexpected user agent=%q", BaseHeaders["User-Agent"])
	}
	if BaseHeaders["x-client-platform"] != "android" {
		t.Fatalf("unexpected base header x-client-platform=%q", BaseHeaders["x-client-platform"])
	}
	if BaseHeaders["x-client-version"] != ClientVersion {
		t.Fatalf("unexpected base header x-client-version=%q", BaseHeaders["x-client-version"])
	}
	if BaseHeaders["Accept-Language"] != "zh-CN,zh;q=0.9" {
		t.Fatalf("unexpected base header Accept-Language=%q", BaseHeaders["Accept-Language"])
	}
	if BaseHeaders["x-client-timezone-offset"] != "28800" {
		t.Fatalf("unexpected base header x-client-timezone-offset=%q", BaseHeaders["x-client-timezone-offset"])
	}
	if !looksLikeRangersID(BaseHeaders["x-rangers-id"]) {
		t.Fatalf("unexpected base header x-rangers-id=%q", BaseHeaders["x-rangers-id"])
	}
	if BaseHeaders["Content-Type"] != "application/json" {
		t.Fatalf("unexpected base header Content-Type=%q", BaseHeaders["Content-Type"])
	}
	if BaseHeaders["x-client-bundle-id"] != "com.deepseek.chat" {
		t.Fatalf("unexpected base header x-client-bundle-id=%q", BaseHeaders["x-client-bundle-id"])
	}
	if len(SkipContainsPatterns) == 0 {
		t.Fatal("expected skip contains patterns to be loaded")
	}
	if _, ok := SkipExactPathSet["response/search_status"]; !ok {
		t.Fatal("expected response/search_status in exact skip path set")
	}
}

func TestClientHeadersDerivedFromSharedVersion(t *testing.T) {
	client := normalizeClientConstants(clientConstants{
		Name:            "DeepSeek",
		Platform:        "android",
		Version:         "9.8.7",
		AndroidAPILevel: "35",
		Locale:          "zh_CN",
	})
	headers := buildBaseHeaders(client, map[string]string{
		"User-Agent":       "stale",
		"x-client-version": "stale",
	})
	if headers["User-Agent"] != "DeepSeek/9.8.7 Android/35" {
		t.Fatalf("unexpected derived user agent=%q", headers["User-Agent"])
	}
	if headers["x-client-version"] != "9.8.7" {
		t.Fatalf("unexpected derived client version=%q", headers["x-client-version"])
	}
	if headers["Accept-Language"] != "zh-CN,zh;q=0.9" {
		t.Fatalf("unexpected accept language=%q", headers["Accept-Language"])
	}
	if headers["x-client-timezone-offset"] != "28800" {
		t.Fatalf("unexpected timezone offset=%q", headers["x-client-timezone-offset"])
	}
}

func TestAcceptLanguageFromLocale(t *testing.T) {
	tests := []struct {
		locale string
		want   string
	}{
		{locale: "zh_CN", want: "zh-CN,zh;q=0.9"},
		{locale: "en_US", want: "en-US,en;q=0.9"},
		{locale: "ja", want: "ja"},
		{locale: "", want: ""},
	}
	for _, tc := range tests {
		if got := acceptLanguageFromLocale(tc.locale); got != tc.want {
			t.Fatalf("acceptLanguageFromLocale(%q)=%q want %q", tc.locale, got, tc.want)
		}
	}
}

func TestBaseHeadersPreserveExplicitAcceptLanguage(t *testing.T) {
	client := normalizeClientConstants(clientConstants{
		Version: "1.2.3",
		Locale:  "zh_CN",
	})
	headers := buildBaseHeaders(client, map[string]string{
		"Accept-Language": "zh-CN",
	})
	if got := headers["Accept-Language"]; got != "zh-CN" {
		t.Fatalf("Accept-Language=%q want explicit override", got)
	}
}

func TestEnvClientOverrides(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_CLIENT_NAME", "DeepSeekTest")
	t.Setenv("DS2API_DEEPSEEK_CLIENT_PLATFORM", "android")
	t.Setenv("DS2API_DEEPSEEK_CLIENT_VERSION", "8.7.6")
	t.Setenv("DS2API_DEEPSEEK_ANDROID_API_LEVEL", "34")
	t.Setenv("DS2API_DEEPSEEK_CLIENT_LOCALE", "en_US")
	t.Setenv("DS2API_DEEPSEEK_CLIENT_TIMEZONE_OFFSET", "0")
	t.Setenv("DS2API_DEEPSEEK_RANGERS_ID", "7659744735539498504")

	client := applyEnvClientOverrides(normalizeClientConstants(clientConstants{}))
	headers := buildBaseHeaders(client, nil)
	if got := headers["User-Agent"]; got != "DeepSeekTest/8.7.6 Android/34" {
		t.Fatalf("User-Agent=%q want overridden Android UA", got)
	}
	if got := headers["x-client-version"]; got != "8.7.6" {
		t.Fatalf("x-client-version=%q want 8.7.6", got)
	}
	if got := headers["x-client-locale"]; got != "en_US" {
		t.Fatalf("x-client-locale=%q want en_US", got)
	}
	if got := headers["Accept-Language"]; got != "en-US,en;q=0.9" {
		t.Fatalf("Accept-Language=%q want en-US,en;q=0.9", got)
	}
	if got := headers["x-client-timezone-offset"]; got != "0" {
		t.Fatalf("x-client-timezone-offset=%q want 0", got)
	}
	if got := headers["x-rangers-id"]; got != "7659744735539498504" {
		t.Fatalf("x-rangers-id=%q want override", got)
	}
}

func TestDerivedDefaultRangersIDShape(t *testing.T) {
	got := deriveDefaultRangersID()
	if !looksLikeRangersID(got) {
		t.Fatalf("deriveDefaultRangersID()=%q, want 19 digit id", got)
	}
	if got2 := deriveDefaultRangersID(); got2 != got {
		t.Fatalf("deriveDefaultRangersID() not stable: %q then %q", got, got2)
	}
}

func TestBaseHeadersForRangersSeedDerivesStableAccountID(t *testing.T) {
	oldExplicit := explicitRangersID
	explicitRangersID = ""
	t.Cleanup(func() { explicitRangersID = oldExplicit })

	headersA := BaseHeadersForRangersSeed("user@example.com")
	headersB := BaseHeadersForRangersSeed("user@example.com")
	headersC := BaseHeadersForRangersSeed("other@example.com")
	if headersA["x-rangers-id"] != headersB["x-rangers-id"] {
		t.Fatalf("expected same seed to produce same x-rangers-id: %q vs %q", headersA["x-rangers-id"], headersB["x-rangers-id"])
	}
	if headersA["x-rangers-id"] == headersC["x-rangers-id"] {
		t.Fatalf("expected different seeds to produce different x-rangers-id: %q", headersA["x-rangers-id"])
	}
	if !looksLikeRangersID(headersA["x-rangers-id"]) {
		t.Fatalf("unexpected account x-rangers-id=%q", headersA["x-rangers-id"])
	}
}

func looksLikeRangersID(v string) bool {
	if len(v) != 19 {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return strings.HasPrefix(v, "7")
}
