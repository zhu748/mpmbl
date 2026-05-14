package transport

import (
	"testing"

	utls "github.com/refraction-networking/utls"
)

func TestJoinDialErrorsIncludesAllProfiles(t *testing.T) {
	err := joinDialErrors("example.com:443", []error{
		assertErr("android failed"),
		assertErr("chrome failed"),
	})
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	got := err.Error()
	for _, want := range []string{
		"tls fingerprint dial failed for example.com:443",
		"android failed",
		"chrome failed",
	} {
		if !contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestDefaultTLSFingerprintProfilesPrefersAndroid(t *testing.T) {
	if len(defaultTLSFingerprintProfiles) == 0 {
		t.Fatal("expected default fingerprint profiles")
	}
	first := defaultTLSFingerprintProfiles[0]
	if first.name != "android-okhttp" {
		t.Fatalf("expected android profile first, got %q", first.name)
	}
	if first.helloID != utls.HelloAndroid_11_OkHttp {
		t.Fatalf("expected HelloAndroid_11_OkHttp first, got %#v", first.helloID)
	}
	if len(first.alpnProtocols) != 2 || first.alpnProtocols[0] != "h2" || first.alpnProtocols[1] != "http/1.1" {
		t.Fatalf("expected android profile to prefer h2 then http/1.1, got %#v", first.alpnProtocols)
	}
}

func TestAllowedNegotiatedALPN(t *testing.T) {
	cases := []struct {
		name       string
		negotiated string
		allowed    []string
		want       bool
	}{
		{name: "empty ok", negotiated: "", allowed: []string{"h2", "http/1.1"}, want: true},
		{name: "h2 ok", negotiated: "h2", allowed: []string{"h2", "http/1.1"}, want: true},
		{name: "http11 ok", negotiated: "http/1.1", allowed: []string{"h2", "http/1.1"}, want: true},
		{name: "other denied", negotiated: "spdy/3", allowed: []string{"h2", "http/1.1"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allowedNegotiatedALPN(tc.negotiated, tc.allowed); got != tc.want {
				t.Fatalf("allowedNegotiatedALPN(%q, %#v)=%v want %v", tc.negotiated, tc.allowed, got, tc.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(msg string) error { return staticErr(msg) }
