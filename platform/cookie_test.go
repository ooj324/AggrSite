package platform

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeCookieHeaderStripsSetCookieAttributes(t *testing.T) {
	got := NormalizeCookieHeader("Set-Cookie: session=abc; Path=/; HttpOnly; SameSite=Lax; Expires=Wed, 21 Oct 2025 07:28:00 GMT")
	if got != "session=abc" {
		t.Fatalf("unexpected normalized cookie: %q", got)
	}
}

func TestNormalizeCookieHeaderHandlesMultipleSetCookieLines(t *testing.T) {
	got := NormalizeCookieHeader("Set-Cookie: session=abc; Path=/; HttpOnly\nSet-Cookie: token=def; Path=/; SameSite=Lax")
	if got != "session=abc; token=def" {
		t.Fatalf("unexpected normalized cookie: %q", got)
	}
}

func TestMergeCookieHeadersOverridesByNameAndStripsAttributes(t *testing.T) {
	got := mergeCookieHeaders("session=old; acw=1", "session=new; Path=/; HttpOnly; token=t")
	want := "session=new; acw=1; token=t"
	if got != want {
		t.Fatalf("unexpected merged cookie:\nwant %q\n got %q", want, got)
	}
}

func TestBuildCookieCandidatesNormalizesCookieInputs(t *testing.T) {
	got := BuildCookieCandidates("Bearer session=abc; Path=/; HttpOnly")
	if len(got) != 1 || got[0] != "session=abc" {
		t.Fatalf("unexpected cookie candidates: %#v", got)
	}

	got = BuildCookieCandidates("raw-token")
	want := []string{"session=raw-token", "token=raw-token"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected token candidates:\nwant %#v\n got %#v", want, got)
	}
}

func TestBuildCookieCandidatesTreatsPaddedRawSessionAsValue(t *testing.T) {
	payload := strings.Repeat("payload", 20)
	raw := base64.StdEncoding.EncodeToString([]byte("1782783060|" + payload + "|signature"))
	if !strings.HasSuffix(raw, "=") {
		t.Fatalf("test token should have base64 padding: %q", raw)
	}

	got := BuildCookieCandidates(raw)
	want := []string{"session=" + raw, "token=" + raw}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected padded token candidates:\nwant %#v\n got %#v", want, got)
	}
	if !IsCookieSessionToken(raw) {
		t.Fatal("expected signed session value to be treated as cookie session credential")
	}
}

func TestBuildCookieCandidatesAddsSessionOnlyFallbackForFullCookie(t *testing.T) {
	got := BuildCookieCandidates("_ga=ga; acw_tc=stale; session=abc; _ga_x=ga2")
	want := []string{"_ga=ga; acw_tc=stale; session=abc; _ga_x=ga2", "session=abc"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected full cookie candidates:\nwant %#v\n got %#v", want, got)
	}
}

func TestFetchJSONAppliesCustomHeadersWithoutOverridingExplicitAuth(t *testing.T) {
	customHeaders := `{"Authorization":"Bearer site","Cookie":"session=site; Path=/; HttpOnly; cf=1","X-Trace":123}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer account" {
			t.Fatalf("unexpected Authorization: %q", got)
		}
		if got := r.Header.Get("Cookie"); got != "session=account; cf=1; token=abc" {
			t.Fatalf("unexpected Cookie: %q", got)
		}
		if got := r.Header.Get("X-Trace"); got != "123" {
			t.Fatalf("unexpected X-Trace: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	var res map[string]interface{}
	base := &BaseAdapter{Name: "test"}
	err := base.FetchJSON(server.URL, "GET", map[string]string{
		"Authorization": "Bearer account",
		"Cookie":        "session=account; token=abc",
	}, nil, &res, &RequestOption{CustomHeaders: &customHeaders})
	if err != nil {
		t.Fatalf("FetchJSON returned error: %v", err)
	}
}

func TestRequestRouteFingerprintMatchesTransportFallbacks(t *testing.T) {
	invalidProxy := "http://[invalid"
	if got := RequestRouteFingerprint(&RequestOption{ProxyURL: &invalidProxy}); got != "environment" {
		t.Fatalf("invalid proxy route = %q, want environment fallback", got)
	}

	direct := false
	if got := RequestRouteFingerprint(&RequestOption{UseSystemProxy: &direct}); got != "direct" {
		t.Fatalf("explicit direct route = %q", got)
	}

	proxyURL := "http://127.0.0.1:7890"
	if got := RequestRouteFingerprint(&RequestOption{ProxyURL: &proxyURL}); got != "proxy:"+proxyURL {
		t.Fatalf("explicit proxy route = %q", got)
	}
}

func TestFetchJSONWithCookieRetryReturnsHTTPErrorForNon2xxJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"success":false,"message":"missing Origin"}`))
	}))
	defer server.Close()

	var res map[string]interface{}
	_, err := FetchJSONWithCookieRetry(server.URL, "POST", "session=abc", nil, map[string]interface{}{}, &res, nil)
	if err == nil {
		t.Fatal("expected HTTP error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 403: missing Origin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchJSONWithCookieRetryPreservesRetryAfterAndSetCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "13")
		w.Header().Add("Set-Cookie", "rotating=value-next; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"success":false,"message":"slow down"}`))
	}))
	defer server.Close()

	var res map[string]interface{}
	cookieResult, err := FetchJSONWithCookieRetry(server.URL, "POST", "rotating=value-old", nil, nil, &res, nil)
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	if got := RetryAfterFromError(err); got != 13*time.Second {
		t.Fatalf("Retry-After = %s, want 13s", got)
	}
	if cookieResult == nil || len(cookieResult.SetCookies) != 1 || !strings.Contains(cookieResult.SetCookies[0], "rotating=value-next") {
		t.Fatalf("rotating Set-Cookie was not preserved: %#v", cookieResult)
	}
}

func TestParseRetryAfterAcceptsHTTPDate(t *testing.T) {
	now := time.Date(2026, time.August, 17, 1, 2, 3, 0, time.UTC)
	value := now.Add(45 * time.Second).Format(http.TimeFormat)
	if got := parseRetryAfter(value, now); got != 45*time.Second {
		t.Fatalf("HTTP-date Retry-After = %s, want 45s", got)
	}
}

func TestFetchJSONWithCookieRetryKeepsRedirectSetCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "redirect-session", Path: "/"})
			http.Redirect(w, r, "/done", http.StatusFound)
		case "/done":
			if got := r.Header.Get("Cookie"); got != "session=redirect-session" {
				t.Fatalf("redirect request lost cookie: %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var res map[string]interface{}
	result, err := FetchJSONWithCookieRetry(server.URL+"/login", "POST", "", nil, map[string]interface{}{}, &res, nil)
	if err != nil {
		t.Fatalf("FetchJSONWithCookieRetry returned error: %v", err)
	}
	if result == nil || result.CookieHeader != "session=redirect-session" {
		t.Fatalf("unexpected cookie result: %#v", result)
	}
}

func TestFetchJSONWithCookieRetryRetriesShieldWithNewSetCookie(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.SetCookie(w, &http.Cookie{Name: "acw_tc", Value: "fresh", Path: "/"})
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><script>window._shield=1</script></html>`))
			return
		}
		if got := r.Header.Get("Cookie"); got != "session=abc; acw_tc=fresh" {
			t.Fatalf("retry lost fresh shield cookie: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	var res map[string]interface{}
	_, err := FetchJSONWithCookieRetry(server.URL, "GET", "session=abc", nil, nil, &res, nil)
	if err != nil {
		t.Fatalf("FetchJSONWithCookieRetry returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
