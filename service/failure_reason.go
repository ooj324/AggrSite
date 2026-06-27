package service

import "strings"

type FailureReason struct {
	Code       string `json:"code"`
	Category   string `json:"category"`
	Title      string `json:"title"`
	ActionHint string `json:"actionHint"`
	DetailHint string `json:"detailHint"`
}

func AnalyzeCheckinFailure(message string) *FailureReason {
	msgLower := strings.ToLower(message)

	if strings.Contains(msgLower, "token expired") || 
		strings.Contains(msgLower, "invalid token") || 
		strings.Contains(msgLower, "login failed") || 
		strings.Contains(msgLower, "unauthorized") {
		return &FailureReason{
			Code:       "TOKEN_EXPIRED",
			Category:   "credential",
			Title:      "令牌过期",
			ActionHint: "重新登录",
			DetailHint: "账号会话已过期，需要重新登录或换绑",
		}
	}

	if strings.Contains(msgLower, "acw_sc__v2") || 
		strings.Contains(msgLower, "var arg1") || 
		strings.Contains(msgLower, "captcha") || 
		strings.Contains(msgLower, "challenge") || 
		strings.Contains(msgLower, "shield") ||
		strings.Contains(msgLower, "cloudflare") ||
		strings.Contains(msgLower, "invalid character") {
		return &FailureReason{
			Code:       "SHIELD_BLOCKED",
			Category:   "network",
			Title:      "防爬拦截",
			ActionHint: "使用 API Key",
			DetailHint: "请求被站点防火墙拦截，建议改用 API Key 模式",
		}
	}

	if strings.Contains(msgLower, "turnstile") {
		return &FailureReason{
			Code:       "TURNSTILE_REQUIRED",
			Category:   "manual",
			Title:      "需要人机验证",
			ActionHint: "手动签到",
			DetailHint: "站点开启了 Turnstile 校验，无法自动签到",
		}
	}

	if strings.Contains(msgLower, "不支持") || 
		strings.Contains(msgLower, "unsupported") ||
		strings.Contains(msgLower, "no checkin method") {
		return &FailureReason{
			Code:       "UNSUPPORTED",
			Category:   "site",
			Title:      "不支持签到",
			ActionHint: "忽略",
			DetailHint: "该站点不支持自动签到功能",
		}
	}

	if strings.Contains(msgLower, "timeout") || 
		strings.Contains(msgLower, "connection refused") || 
		strings.Contains(msgLower, "network") {
		return &FailureReason{
			Code:       "NETWORK_ERROR",
			Category:   "network",
			Title:      "网络错误",
			ActionHint: "重试",
			DetailHint: "连接站点服务器失败",
		}
	}

	return &FailureReason{
		Code:       "UNKNOWN",
		Category:   "unknown",
		Title:      "未知错误",
		ActionHint: "查看日志",
		DetailHint: message,
	}
}
