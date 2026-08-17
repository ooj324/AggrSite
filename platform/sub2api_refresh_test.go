package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestSub2ApiRefreshAuthUsesOnlyTheRefreshTokenOnce(t *testing.T) {
	calls := 0
	customHeaders := `{"Authorization":"Bearer site-wide-token","Idempotency-Key":"must-not-replay","X-Trace":"kept"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("refresh endpoint must not receive the short-lived access token: %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "" {
			t.Fatalf("one-shot refresh must not be marked replayable: %q", got)
		}
		if got := r.Header.Get("X-Trace"); got != "kept" {
			t.Fatalf("ordinary custom header was lost: %q", got)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["refresh_token"] != "refresh-old" {
			t.Fatalf("refresh token = %q", body["refresh_token"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600}}`))
	}))
	defer server.Close()

	adapter := &Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}}
	res, err := adapter.RefreshAuth(server.URL, "access-old", `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`, &RequestOption{CustomHeaders: &customHeaders})
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || !res.Success || res.AccessToken != "access-new" {
		t.Fatalf("unexpected refresh result: %+v", res)
	}
	if calls != 1 {
		t.Fatalf("one-time refresh credential was submitted %d times, want 1", calls)
	}
}

func TestSub2ApiRefreshAuthClassifiesPermanentAndRateLimitErrors(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		reason         string
		retryAfter     string
		wantDead       bool
		wantRetryAfter time.Duration
	}{
		{
			name:     "invalid refresh token",
			status:   http.StatusUnauthorized,
			reason:   "REFRESH_TOKEN_INVALID",
			wantDead: true,
		},
		{
			name:           "rate limited",
			status:         http.StatusTooManyRequests,
			reason:         "RATE_LIMITED",
			retryAfter:     "9",
			wantRetryAfter: 9 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"code":` + strconv.Itoa(tc.status) + `,"reason":"` + tc.reason + `","message":"refresh failed"}`))
			}))
			defer server.Close()

			adapter := &Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}}
			res, err := adapter.RefreshAuth(server.URL, "access-old", `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`, nil)
			if err != nil {
				t.Fatalf("RefreshAuth returned error: %v", err)
			}
			if res == nil || res.Success {
				t.Fatalf("expected a classified failure, got %+v", res)
			}
			if res.CredentialDead != tc.wantDead {
				t.Fatalf("CredentialDead = %v, want %v", res.CredentialDead, tc.wantDead)
			}
			if res.RetryAfter != tc.wantRetryAfter {
				t.Fatalf("RetryAfter = %s, want %s", res.RetryAfter, tc.wantRetryAfter)
			}
			if calls != 1 {
				t.Fatalf("refresh endpoint called %d times, want 1", calls)
			}
		})
	}
}

func TestSub2ApiRefreshAuthClassifiesEnvelopeReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":401,"reason":"SESSION_BINDING_MISMATCH","message":"session changed"}`))
	}))
	defer server.Close()

	adapter := &Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}}
	res, err := adapter.RefreshAuth(server.URL, "access-old", `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || !res.CredentialDead {
		t.Fatalf("expected SESSION_BINDING_MISMATCH to be permanent, got %+v", res)
	}
}

func TestSub2ApiRefreshAuthDoesNotReplayOnShieldResponse(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><script>var arg1='challenge';</script></html>`))
	}))
	defer server.Close()

	adapter := &Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}}
	res, err := adapter.RefreshAuth(server.URL, "access-old", `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("expected shield failure, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("one-time refresh request was replayed %d times", calls)
	}
}

func TestSub2ApiRefreshAuthDoesNotReplayOnRedirect(t *testing.T) {
	refreshCalls := 0
	redirectCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			refreshCalls++
			http.Redirect(w, r, "/redirect-target", http.StatusTemporaryRedirect)
		case "/redirect-target":
			redirectCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}}
	res, err := adapter.RefreshAuth(server.URL, "access-old", `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success || !res.CredentialDead {
		t.Fatalf("a redirect must stop the one-shot exchange, got %+v", res)
	}
	if refreshCalls != 1 || redirectCalls != 0 {
		t.Fatalf("one-time request followed a redirect: refresh=%d target=%d", refreshCalls, redirectCalls)
	}
}
