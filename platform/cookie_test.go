package platform

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
