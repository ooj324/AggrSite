package platform

import (
	"crypto/sha256"
	"fmt"
	"math/big"
)

type NewApiRunanytimeAdapter struct {
	NewApiAdapter
}

func init() {
	Register(&NewApiRunanytimeAdapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "newapi-runanytime"}}})
}

func solveRunAnyTimePOW(prefix string, difficulty int) string {
	diffBig := new(big.Int).SetInt64(1)
	diffBig.Lsh(diffBig, uint(256-difficulty))

	for i := 0; i < 100000000; i++ {
		nonce := fmt.Sprintf("%08x", i)
		text := prefix + nonce
		hash := sha256.Sum256([]byte(text))

		hashBig := new(big.Int).SetBytes(hash[:])
		if hashBig.Cmp(diffBig) < 0 {
			return nonce
		}
	}
	return ""
}

func (a *NewApiRunanytimeAdapter) executePowCheckin(baseURL string, headers map[string]string, cookie string, opt *RequestOption) (*CheckinResult, error) {
	challengeURL := fmt.Sprintf("%s/api/user/pow/challenge?action=checkin", baseURL)
	var challengeRes map[string]interface{}
	var err error

	if cookie != "" {
		_, err = FetchJSONWithCookieRetry(challengeURL, "GET", cookie, headers, nil, &challengeRes, opt)
	} else {
		err = a.FetchJSON(challengeURL, "GET", headers, nil, &challengeRes, opt)
	}

	if err != nil {
		return nil, err
	}

	if ok, _ := challengeRes["success"].(bool); !ok {
		return nil, fmt.Errorf("pow error: %s", ExtractMessage(challengeRes))
	}

	data, _ := challengeRes["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("pow error: empty data")
	}

	prefix, _ := data["prefix"].(string)
	diffFloat, _ := data["difficulty"].(float64)
	cid, _ := data["challenge_id"].(string)
	nonce := solveRunAnyTimePOW(prefix, int(diffFloat))

	checkinURL := AppendTurnstileParam(fmt.Sprintf("%s/api/user/checkin?pow_challenge=%s&pow_nonce=%s", baseURL, cid, nonce), opt)
	var checkinRes map[string]interface{}

	checkinHeaders := headers
	if cookie != "" {
		checkinHeaders = mergeMaps(headers, map[string]string{"X-Requested-With": "XMLHttpRequest"})
	}

	checkinBody := BuildCheckinBodyWithTurnstile(map[string]interface{}{}, opt)

	if cookie != "" {
		_, err = FetchJSONWithCookieRetry(checkinURL, "POST", cookie, checkinHeaders, checkinBody, &checkinRes, opt)
	} else {
		err = a.FetchJSON(checkinURL, "POST", checkinHeaders, checkinBody, &checkinRes, opt)
	}

	if err != nil {
		return nil, err
	}

	if ok, _ := checkinRes["success"].(bool); ok {
		msg := ExtractMessage(checkinRes)
		if msg == "" {
			msg = "checkin success"
		}
		reward := ""
		if data, ok := checkinRes["data"].(map[string]interface{}); ok {
			if r, ok := data["reward"]; ok {
				reward = fmt.Sprintf("%v", r)
			}
		}
		return &CheckinResult{Success: true, Message: msg, Reward: reward}, nil
	}

	return &CheckinResult{Success: false, Message: ExtractMessage(checkinRes)}, nil
}

func (a *NewApiRunanytimeAdapter) Checkin(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*CheckinResult, error) {
	resolvedUserID := a.discoverUserId(baseURL, accessToken, platformUserID, opt)
	var firstFailureMessage string

	// Step 1: Bearer token checkin
	if !IsCookieSessionToken(accessToken) {
		headers := AuthHeaders(accessToken, resolvedUserID)
		result, err := a.executePowCheckin(baseURL, headers, "", opt)
		if err == nil {
			if result.Success {
				return result, nil
			}
			firstFailureMessage = result.Message
		} else {
			firstFailureMessage = err.Error()
		}

		if firstFailureMessage != "" && !shouldFallbackToCookieCheckin(firstFailureMessage) {
			return &CheckinResult{Success: false, Message: firstFailureMessage}, nil
		}
	}

	// Step 2: Cookie-based checkin with resolved user id
	for _, cookie := range BuildCookieCandidates(accessToken) {
		headers := CookieUserIDHeaders(resolvedUserID)
		result, err := a.executePowCheckin(baseURL, headers, cookie, opt)
		if err == nil {
			if result.Success {
				return result, nil
			}
			if firstFailureMessage == "" {
				firstFailureMessage = result.Message
			}
			if !isCookieSessionFailureMessage(result.Message) {
				return result, nil
			}
		} else {
			if firstFailureMessage == "" {
				firstFailureMessage = err.Error()
			}
		}
	}

	// Step 3: Probe alternate user id via cookie and retry
	alternateUserID := a.probeAlternateCookieUserId(baseURL, accessToken, resolvedUserID, opt)
	if alternateUserID > 0 {
		for _, cookie := range BuildCookieCandidates(accessToken) {
			headers := CookieUserIDHeaders(alternateUserID)
			result, err := a.executePowCheckin(baseURL, headers, cookie, opt)
			if err == nil {
				if result.Success {
					return result, nil
				}
				if firstFailureMessage == "" {
					firstFailureMessage = result.Message
				}
				if !isCookieSessionFailureMessage(result.Message) {
					return result, nil
				}
			} else {
				if firstFailureMessage == "" {
					firstFailureMessage = err.Error()
				}
			}
		}
	}

	if firstFailureMessage == "" {
		firstFailureMessage = "checkin failed"
	}
	return &CheckinResult{Success: false, Message: firstFailureMessage}, nil
}
