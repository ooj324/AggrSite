package service

import (
	"encoding/json"
	"metapi/aggrsite/platform"
	"testing"
	"time"
)

func TestIsManagedSessionPlatform(t *testing.T) {
	for _, name := range []string{"sub2api", "SUB2API", " new-api-v1 "} {
		if !IsManagedSessionPlatform(name) {
			t.Fatalf("%q should be a managed session platform", name)
		}
	}
	for _, name := range []string{"", "new-api", "agentrouter"} {
		if IsManagedSessionPlatform(name) {
			t.Fatalf("%q should not be a managed session platform", name)
		}
	}
}

func TestGetManagedTokenExpiresAtPrefersCanonicalNodeAndFallsBack(t *testing.T) {
	cases := []struct {
		name        string
		extraConfig string
		want        int64
	}{
		{
			name:        "canonical managedAuth",
			extraConfig: `{"managedAuth":{"tokenExpiresAt":1700000000000}}`,
			want:        1700000000000,
		},
		{
			name:        "canonical wins over legacy",
			extraConfig: `{"managedAuth":{"tokenExpiresAt":1700000000000},"sub2apiAuth":{"tokenExpiresAt":1600000000000}}`,
			want:        1700000000000,
		},
		{
			name:        "managedAuth without expiry falls back to sub2apiAuth",
			extraConfig: `{"managedAuth":{"other":1},"sub2apiAuth":{"refreshToken":"r","tokenExpiresAt":1600000000000}}`,
			want:        1600000000000,
		},
		{
			name:        "legacy newApiV1Auth fallback",
			extraConfig: `{"newApiV1Auth":{"tokenExpiresAt":1600000000000}}`,
			want:        1600000000000,
		},
		{
			name:        "seconds are normalized to millis",
			extraConfig: `{"managedAuth":{"tokenExpiresAt":1700000000}}`,
			want:        1700000000000,
		},
		{
			name:        "string values are accepted",
			extraConfig: `{"managedAuth":{"tokenExpiresAt":"1700000000000"}}`,
			want:        1700000000000,
		},
		{
			name:        "missing expiry",
			extraConfig: `{"credentialMode":"session"}`,
			want:        0,
		},
		{
			name:        "invalid json",
			extraConfig: `not-json`,
			want:        0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extraConfig := tc.extraConfig
			if got := getManagedTokenExpiresAt(&extraConfig); got != tc.want {
				t.Fatalf("getManagedTokenExpiresAt = %d, want %d", got, tc.want)
			}
		})
	}

	if got := getManagedTokenExpiresAt(nil); got != 0 {
		t.Fatalf("nil extraConfig should yield 0, got %d", got)
	}
}

func TestIsManagedTokenDue(t *testing.T) {
	now := time.Now()
	if isManagedTokenDue(0, now) {
		t.Fatal("unknown expiry must not be due")
	}
	if isManagedTokenDue(now.Add(managedRefreshLead+time.Minute).UnixMilli(), now) {
		t.Fatal("token outside the refresh lead must not be due")
	}
	if !isManagedTokenDue(now.Add(managedRefreshLead-time.Minute).UnixMilli(), now) {
		t.Fatal("token inside the refresh lead must be due")
	}
	if !isManagedTokenDue(now.Add(-time.Hour).UnixMilli(), now) {
		t.Fatal("already expired token must be due")
	}
}

func TestApplyLoginManagedAuthPersistsRefreshCredential(t *testing.T) {
	cfg := map[string]interface{}{
		"credentialMode": "session",
		"sub2apiAuth":    map[string]interface{}{"tokenExpiresAt": float64(1)},
	}
	changed := ApplyLoginManagedAuth(cfg, &platform.LoginResult{
		Success:       true,
		AccessToken:   "jwt",
		RefreshCookie: " refresh-value ",
		ExpiresAt:     1700000000, // seconds
	})
	if !changed {
		t.Fatal("expected cfg to be modified")
	}

	authNode, _ := cfg[platform.NewApiV1AuthConfigKey].(map[string]interface{})
	if authNode == nil || authNode[platform.RefreshCookieKey] != "refresh-value" {
		t.Fatalf("refresh cookie not persisted (and trimmed): %#v", cfg)
	}
	managed, _ := cfg[platform.ManagedAuthConfigKey].(map[string]interface{})
	if managed == nil || managed[platform.TokenExpiresAtKey] != int64(1700000000000) {
		t.Fatalf("expiry not normalized into managedAuth: %#v", cfg)
	}
	if _, exists := cfg[platform.Sub2APIAuthConfigKey]; exists {
		t.Fatalf("legacy expiry copy must be dropped: %#v", cfg)
	}

	extraConfig := mustMarshalConfig(t, cfg)
	if got := getManagedTokenExpiresAt(&extraConfig); got != 1700000000000 {
		t.Fatalf("stored expiry is not readable back: %d", got)
	}
}

func TestApplyLoginManagedAuthAssumesShortTTLWhenExpiryMissing(t *testing.T) {
	cfg := map[string]interface{}{}
	before := time.Now().UnixMilli()
	if !ApplyLoginManagedAuth(cfg, &platform.LoginResult{RefreshCookie: "refresh-value"}) {
		t.Fatal("expected cfg to be modified")
	}
	managed, _ := cfg[platform.ManagedAuthConfigKey].(map[string]interface{})
	if managed == nil {
		t.Fatalf("managedAuth missing: %#v", cfg)
	}
	expiresAt, _ := managed[platform.TokenExpiresAtKey].(int64)
	if expiresAt <= before {
		t.Fatalf("expected a future fallback expiry, got %d", expiresAt)
	}
}

func TestApplyLoginManagedAuthIgnoresLoginsWithoutRefreshCredential(t *testing.T) {
	cfg := map[string]interface{}{}
	if ApplyLoginManagedAuth(cfg, &platform.LoginResult{Success: true, AccessToken: "jwt"}) {
		t.Fatal("login without refresh cookie must not modify cfg")
	}
	if len(cfg) != 0 {
		t.Fatalf("cfg should stay untouched: %#v", cfg)
	}
	if ApplyLoginManagedAuth(nil, &platform.LoginResult{RefreshCookie: "x"}) {
		t.Fatal("nil cfg must be a no-op")
	}
	if ApplyLoginManagedAuth(cfg, nil) {
		t.Fatal("nil login result must be a no-op")
	}
}

func mustMarshalConfig(t *testing.T, cfg map[string]interface{}) string {
	t.Helper()
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	return string(out)
}
