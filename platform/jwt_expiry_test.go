package platform

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// A real QuantumNous new-api dashboard access token: 15 minute TTL, user id in sub.
const sampleNewApiAccessToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
	"eyJ0b2tlbl91c2UiOiJhY2Nlc3MiLCJzaWQiOiI4YzQzYjU2Mi01NzNmLTQxYzgtYTUwMC0wOWNjM2Y3YjQyMGIiLCJ1diI6MSwic3YiOjEs" +
	"ImlzcyI6Im5ldy1hcGkiLCJzdWIiOiIxODkxIiwiYXVkIjpbIm5ldy1hcGktZGFzaGJvYXJkIl0sImV4cCI6MTc4NjM0NTkyMSwibmJmIjox" +
	"Nzg2MzQ1MDE2LCJpYXQiOjE3ODYzNDUwMjEsImp0aSI6ImU1NzM0NDg2LTNhNDItNDJlOS04ZDY2LWVhMjViYTc1OTdjNyJ9." +
	"gCjHgKEWlGiBFJOWQ5-R9cbUOdMEJOcDCN_pftc5VXw"

func TestJwtExpiresAtMillisReadsExpClaim(t *testing.T) {
	const wantMillis = int64(1786345921) * 1000

	if got := JwtExpiresAtMillis(sampleNewApiAccessToken); got != wantMillis {
		t.Fatalf("JwtExpiresAtMillis = %d, want %d", got, wantMillis)
	}
	if got := JwtExpiresAtMillis("Bearer " + sampleNewApiAccessToken); got != wantMillis {
		t.Fatalf("bearer prefix not handled: %d", got)
	}
	// The same token also carries the platform user id in sub.
	if got := TryDecodeJwtUserID(sampleNewApiAccessToken); got != 1891 {
		t.Fatalf("TryDecodeJwtUserID = %d, want 1891", got)
	}
}

func TestJwtExpiresAtMillisIgnoresNonJwtCredentials(t *testing.T) {
	for _, credential := range []string{
		"",
		"sk-plain-api-key",
		"session=abc; new_api_refresh=refresh-value",
		"not.a.jwt",
	} {
		if got := JwtExpiresAtMillis(credential); got != 0 {
			t.Fatalf("JwtExpiresAtMillis(%q) = %d, want 0", credential, got)
		}
	}

	// exp is present but unusable.
	if got := JwtExpiresAtMillis(testJwt(t, map[string]interface{}{"exp": 0})); got != 0 {
		t.Fatalf("zero exp should be ignored, got %d", got)
	}
	if got := JwtExpiresAtMillis(testJwt(t, map[string]interface{}{"sub": "1"})); got != 0 {
		t.Fatalf("missing exp should yield 0, got %d", got)
	}
	// Seconds and string encodings both normalize to millis.
	if got := JwtExpiresAtMillis(testJwt(t, map[string]interface{}{"exp": "1786345921"})); got != 1786345921000 {
		t.Fatalf("string exp = %d", got)
	}
}

func TestJwtLifetimeMillis(t *testing.T) {
	// The sample dashboard token was issued for 15 minutes (exp - iat).
	if got := JwtLifetimeMillis(sampleNewApiAccessToken); got != int64(15*time.Minute/time.Millisecond) {
		t.Fatalf("JwtLifetimeMillis = %d, want %d", got, int64(15*time.Minute/time.Millisecond))
	}
	for _, credential := range []string{
		"",
		"session=abc; new_api_refresh=refresh-value",
		testJwt(t, map[string]interface{}{"exp": 1786345921}),                    // no iat
		testJwt(t, map[string]interface{}{"iat": 1786345021}),                    // no exp
		testJwt(t, map[string]interface{}{"iat": 1786345921, "exp": 1786345021}), // inverted
	} {
		if got := JwtLifetimeMillis(credential); got != 0 {
			t.Fatalf("JwtLifetimeMillis(%q) = %d, want 0", credential, got)
		}
	}
}

func testJwt(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
