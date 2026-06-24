package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// ---- Cookie helpers ----

// BuildCookieCandidates generates cookie header candidates from an access token.
func BuildCookieCandidates(accessToken string) []string {
	raw := strings.TrimSpace(accessToken)
	if raw == "" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "Bearer ")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	seen := map[string]bool{}
	var candidates []string
	add := func(c string) {
		if !seen[c] {
			seen[c] = true
			candidates = append(candidates, c)
		}
	}

	if strings.Contains(raw, "=") {
		add(raw)
	}
	add("session=" + raw)
	add("token=" + raw)

	return candidates
}

// IsCookieSessionToken returns true if the access token looks like a cookie.
func IsCookieSessionToken(accessToken string) bool {
	raw := strings.TrimSpace(accessToken)
	raw = strings.TrimPrefix(raw, "Bearer ")
	raw = strings.TrimSpace(raw)
	return strings.Contains(raw, "=")
}

// CookieUserIDHeaders builds extra headers with user-id for cookie-based requests.
func CookieUserIDHeaders(platformUserID int64) map[string]string {
	if platformUserID <= 0 {
		return nil
	}
	uid := fmt.Sprintf("%d", platformUserID)
	return map[string]string{
		"New-Api-User": uid,
		"Veloera-User": uid,
		"User-id":      uid,
	}
}

// ---- Shield Challenge Bypass ----

func parseChallengeArg1(html string) string {
	re := regexp.MustCompile(`var\s+arg1\s*=\s*['"]([0-9a-fA-F]+)['"]`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}
	return ""
}

func parseChallengeMapping(html string) []int {
	re := regexp.MustCompile(`for\(var m=\[([^\]]+)\],p=L\(0x115\)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil
	}
	raws := strings.Split(matches[1], ",")
	var values []int
	for _, raw := range raws {
		val := strings.TrimSpace(strings.ToLower(raw))
		if val == "" {
			return nil
		}
		if strings.HasPrefix(val, "0x") {
			parsed, err := strconv.ParseInt(val[2:], 16, 64)
			if err != nil {
				return nil
			}
			values = append(values, int(parsed))
		} else {
			parsed, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil
			}
			values = append(values, int(parsed))
		}
	}
	return values
}

func parseChallengeXorSeed(html string) string {
	fnStart := strings.Index(html, "function a0i()")
	bStart := strings.Index(html, "function b(")
	rotateStart := strings.Index(html, "(function(a,c){")
	if fnStart < 0 || bStart < 0 || bStart <= fnStart || rotateStart < 0 {
		return ""
	}
	rotateEnd := strings.Index(html[rotateStart:], "),!(function")
	if rotateEnd < 0 {
		return ""
	}
	rotateEnd += rotateStart

	helperCode := html[fnStart:bStart]
	rotateCode := html[rotateStart:rotateEnd+1] + ")"

	vm := goja.New()
	vm.Set("decodeURIComponent", func(s string) string {
		res, err := url.QueryUnescape(s)
		if err != nil {
			return s
		}
		return res
	})

	if _, err := vm.RunString(helperCode); err != nil {
		return ""
	}
	if _, err := vm.RunString(rotateCode); err != nil {
		return ""
	}

	a0jVal := vm.Get("a0j")
	if a0jVal == nil {
		return ""
	}

	var a0j func(int) string
	if err := vm.ExportTo(a0jVal, &a0j); err != nil {
		return ""
	}

	seed := a0j(0x115)
	if matched, _ := regexp.MatchString("(?i)^[0-9a-f]+$", seed); !matched {
		return ""
	}
	return seed
}

func solveNewApiAcwScV2(html string) string {
	arg1 := parseChallengeArg1(html)
	mapping := parseChallengeMapping(html)
	xorSeed := parseChallengeXorSeed(html)

	if arg1 == "" || mapping == nil || xorSeed == "" {
		return ""
	}

	reordered := make([]byte, len(mapping))
	for i := 0; i < len(arg1); i++ {
		ch := arg1[i]
		for j := 0; j < len(mapping); j++ {
			if mapping[j] == i+1 {
				reordered[j] = ch
			}
		}
	}

	source := string(reordered)
	var out strings.Builder
	for i := 0; i < len(source) && i < len(xorSeed); i += 2 {
		if i+2 > len(source) || i+2 > len(xorSeed) {
			break
		}
		left, err1 := strconv.ParseInt(source[i:i+2], 16, 64)
		right, err2 := strconv.ParseInt(xorSeed[i:i+2], 16, 64)
		if err1 != nil || err2 != nil {
			return ""
		}
		out.WriteString(fmt.Sprintf("%02x", left^right))
	}
	return out.String()
}

func isShieldChallenge(contentType, text string) bool {
	normalizedType := strings.ToLower(contentType)
	if strings.Contains(normalizedType, "text/html") {
		re := regexp.MustCompile(`(?i)var\s+arg1\s*=|acw_sc__v2|cdn_sec_tc|<script`)
		if re.MatchString(text) {
			return true
		}
	}
	re2 := regexp.MustCompile(`var\s+arg1\s*=`)
	return re2.MatchString(text)
}

func upsertCookie(cookieHeader, name, value string) string {
	parts := strings.Split(cookieHeader, ";")
	var next []string
	replaced := false

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.Index(part, "=")
		if eq < 0 {
			next = append(next, part)
			continue
		}
		k := strings.TrimSpace(part[:eq])
		if k == name {
			replaced = true
			next = append(next, fmt.Sprintf("%s=%s", name, value))
		} else {
			next = append(next, part)
		}
	}
	if !replaced {
		next = append(next, fmt.Sprintf("%s=%s", name, value))
	}
	return strings.Join(next, "; ")
}

func mergeSetCookiePairs(cookieHeader string, setCookies []string) string {
	merged := cookieHeader
	for _, raw := range setCookies {
		if raw == "" {
			continue
		}
		firstPair := strings.TrimSpace(strings.Split(raw, ";")[0])
		if firstPair == "" {
			continue
		}
		eq := strings.Index(firstPair, "=")
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(firstPair[:eq])
		value := firstPair[eq+1:]
		merged = upsertCookie(merged, name, value)
	}
	return merged
}

// ---- Fetch Retry Loop ----

type FetchCookieResult struct {
	CookieHeader string
}

// FetchJSONWithCookieRetry makes an HTTP request, merging Set-Cookie headers
// and solving the shield challenge (acw_sc__v2) automatically.
func FetchJSONWithCookieRetry(reqURL, method string, cookie string, extraHeaders map[string]string, body interface{}, out interface{}, opt *RequestOption) (*FetchCookieResult, error) {
	currentCookie := cookie

	for attempt := 0; attempt < 3; attempt++ {
		var bodyReader io.Reader
		if body != nil {
			bs, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(bs)
		}

		req, err := http.NewRequest(method, reqURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36")
		if currentCookie != "" {
			req.Header.Set("Cookie", currentCookie)
		}
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
		applyCustomHeaders(req, opt)

		client := &http.Client{
			Timeout:   30 * time.Second,
			Transport: buildTransport(opt),
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		// Collect Set-Cookie headers
		setCookies := resp.Header.Values("Set-Cookie")
		currentCookie = mergeSetCookiePairs(currentCookie, setCookies)

		// Try to parse JSON first
		if jsonErr := json.Unmarshal(respBody, out); jsonErr == nil {
			return &FetchCookieResult{CookieHeader: currentCookie}, nil
		}

		// If not valid JSON, check for shield challenge
		if !isShieldChallenge(resp.Header.Get("Content-Type"), string(respBody)) {
			// Not a challenge, maybe just a server error
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
			}
			return &FetchCookieResult{CookieHeader: currentCookie}, fmt.Errorf("invalid json response")
		}

		if currentCookie == "" {
			return nil, fmt.Errorf("shield challenge encountered but no cookie available")
		}

		acwScV2 := solveNewApiAcwScV2(string(respBody))
		if acwScV2 == "" {
			return nil, fmt.Errorf("failed to solve shield challenge")
		}

		currentCookie = upsertCookie(currentCookie, "acw_sc__v2", acwScV2)
		// continue to next attempt
	}

	return nil, fmt.Errorf("exceeded max shield bypass attempts")
}

// FetchJSONWithCookie retains the simple signature for places that don't need retry logic.
func FetchJSONWithCookie(reqURL, method string, cookie string, extraHeaders map[string]string, body interface{}, out interface{}, opt *RequestOption) error {
	_, err := FetchJSONWithCookieRetry(reqURL, method, cookie, extraHeaders, body, out, opt)
	return err
}
