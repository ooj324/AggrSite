package platform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestDiscoverUserIDCacheReusesResolvedID verifies that a cookie-session account
// does not re-run the user-id probe matrix on every adapter call. Once Checkin
// resolves the id, the follow-up GetBalance must reuse it instead of hammering
// /api/user/self again.
func TestDiscoverUserIDCacheReusesResolvedID(t *testing.T) {
	// Use a cookie token that cannot decode a JWT id, forcing the probe path.
	const cookieToken = "session=abcdefghijklmnopqrstuvwxyz012345"

	var selfProbes int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			atomic.AddInt64(&selfProbes, 1)
			// Only succeed when the New-Api-User header matches the real id (7),
			// mirroring sites that require the user-id header.
			if r.Header.Get("New-Api-User") == "7" {
				_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"quota":500000,"used_quota":0}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":false,"message":"unauthorized"}`))
		case "/api/user/checkin", "/api/user/sign_in":
			if r.Header.Get("New-Api-User") == "7" {
				_, _ = w.Write([]byte(`{"success":true,"message":"checkin success","data":{"reward":10}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":false,"message":"unauthorized"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Ensure a clean cache for this base URL.
	resolvedUserIDCacheMu.Lock()
	resolvedUserIDCache = map[string]resolvedUserIDEntry{}
	resolvedUserIDCacheMu.Unlock()

	adapter := &NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}}

	res, err := adapter.Checkin(server.URL, cookieToken, 0, nil)
	if err != nil {
		t.Fatalf("Checkin returned error: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("expected successful checkin, got %+v", res)
	}
	afterCheckin := atomic.LoadInt64(&selfProbes)
	if afterCheckin == 0 {
		t.Fatalf("expected checkin to probe /api/user/self at least once")
	}

	// The cached id must satisfy the follow-up balance refresh with a single
	// authoritative /api/user/self call (no re-probing).
	balance, err := adapter.GetBalance(server.URL, cookieToken, 0, nil)
	if err != nil {
		t.Fatalf("GetBalance returned error: %v", err)
	}
	if balance == nil {
		t.Fatalf("expected balance info")
	}
	afterBalance := atomic.LoadInt64(&selfProbes)

	extraProbes := afterBalance - afterCheckin
	if extraProbes > 1 {
		t.Fatalf("GetBalance re-probed user id %d times; expected reuse of cached id (<=1)", extraProbes)
	}

	cached, ok := lookupResolvedUserID(server.URL, cookieToken)
	if !ok || cached != 7 {
		t.Fatalf("expected cached user id 7, got %d (ok=%v)", cached, ok)
	}
}

// TestProbeUserIDRespectsBudget ensures the probe matrix is bounded so a token
// that never resolves cannot fan out into the full cookie x candidate product.
func TestProbeUserIDRespectsBudget(t *testing.T) {
	const cookieToken = "session=neverresolves0123456789abcdef"

	var selfProbes int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/user/self" {
			atomic.AddInt64(&selfProbes, 1)
			_, _ = w.Write([]byte(`{"success":false,"message":"unauthorized"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolvedUserIDCacheMu.Lock()
	resolvedUserIDCache = map[string]resolvedUserIDEntry{}
	resolvedUserIDCacheMu.Unlock()

	adapter := &NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}}
	id := adapter.probeUserId(server.URL, cookieToken, nil)
	if id != 0 {
		t.Fatalf("expected unresolved id 0, got %d", id)
	}

	// Step 3 (cookie probe, no id header) plus the bounded step-4 matrix. Every
	// candidate id is probed once with the primary cookie, and extra cookie
	// variants share maxCookieVariantsPerProbe. The worst-case fan-out must not
	// exceed that bound.
	cookieCandidates := int64(len(BuildCookieCandidates(cookieToken)))
	idCandidates := int64(len(BuildUserIDProbeCandidates(cookieToken)))
	got := atomic.LoadInt64(&selfProbes)
	// step 3: one probe per cookie candidate.
	// step 4: one probe per candidate id (primary cookie) + bounded extra variants.
	maxExpected := cookieCandidates + idCandidates + int64(maxCookieVariantsPerProbe)
	if got > maxExpected {
		t.Fatalf("probe issued %d /api/user/self requests; budget cap expected <= %d", got, maxExpected)
	}
	_ = fmt.Sprint(got)
}
