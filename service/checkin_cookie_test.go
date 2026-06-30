package service

import "testing"

func TestBuildCookieAuthHeaderValueAvoidsDoublePrefix(t *testing.T) {
	cases := []struct {
		name       string
		credential string
		prefix     string
		want       string
	}{
		{
			name:       "raw token",
			credential: "abc",
			prefix:     "auth_token=",
			want:       "auth_token=abc",
		},
		{
			name:       "same cookie name",
			credential: "auth_token=abc; Path=/; HttpOnly",
			prefix:     "auth_token=",
			want:       "auth_token=abc",
		},
		{
			name:       "session cookie value reused",
			credential: "session=abc; Path=/; HttpOnly",
			prefix:     "auth_token=",
			want:       "auth_token=abc",
		},
		{
			name:       "skip shield cookie",
			credential: "acw_sc__v2=shield; session=abc",
			prefix:     "auth_token=",
			want:       "auth_token=abc",
		},
		{
			name:       "no prefix preserves cookie header",
			credential: "session=abc; Path=/; HttpOnly",
			prefix:     "",
			want:       "session=abc",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildCookieAuthHeaderValue(tc.credential, tc.prefix)
			if got != tc.want {
				t.Fatalf("unexpected cookie auth value:\nwant %q\n got %q", tc.want, got)
			}
		})
	}
}
