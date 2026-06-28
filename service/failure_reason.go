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

	if strings.Contains(msgLower, "site disabled") {
		return &FailureReason{
			Code:       "SITE_DISABLED",
			Category:   "site",
			Title:      "站点已禁用",
			ActionHint: "启用站点后再试",
			DetailHint: "该账号所属站点处于禁用状态，任务会自动跳过。",
		}
	}

	if strings.Contains(msgLower, "already checked in") ||
		strings.Contains(msgLower, "already signed") ||
		strings.Contains(msgLower, "今天已经签到") ||
		strings.Contains(msgLower, "今日已签到") ||
		strings.Contains(msgLower, "已经签到") {
		return &FailureReason{
			Code:       "ALREADY_CHECKED_IN",
			Category:   "state",
			Title:      "今日已签到",
			ActionHint: "无需重复执行",
			DetailHint: "该账号当天签到已完成，重复请求会被站点拒绝或跳过。",
		}
	}

	if strings.Contains(msgLower, "turnstile") {
		return &FailureReason{
			Code:       "TURNSTILE_REQUIRED",
			Category:   "verification",
			Title:      "需要人工验证",
			ActionHint: "浏览器先人工签到一次",
			DetailHint: "站点开启了 Turnstile 人机验证，自动签到无法直接通过。",
		}
	}

	if strings.Contains(msgLower, "token expired") ||
		strings.Contains(msgLower, "invalid token") ||
		strings.Contains(msgLower, "login failed") ||
		strings.Contains(msgLower, "unauthorized") ||
		strings.Contains(msgLower, "forbidden") ||
		strings.Contains(msgLower, "not login") ||
		strings.Contains(msgLower, "not logged") {
		return &FailureReason{
			Code:       "TOKEN_EXPIRED",
			Category:   "auth",
			Title:      "令牌失效",
			ActionHint: "重新登录或同步新令牌",
			DetailHint: "账号访问令牌可能过期或无效，需更新认证信息。",
		}
	}

	if strings.Contains(msgLower, "cloudflare tunnel error") ||
		strings.Contains(msgLower, "error 1033") ||
		strings.Contains(msgLower, "unable to resolve it") {
		return &FailureReason{
			Code:       "CLOUDFLARE_TUNNEL_UNAVAILABLE",
			Category:   "network",
			Title:      "站点隧道不可用",
			ActionHint: "稍后重试或联系站点方",
			DetailHint: "Cloudflare Tunnel 当前不可达，通常是站点侧网络或隧道进程问题。",
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
			Code:       "CLOUDFLARE_CHALLENGE",
			Category:   "verification",
			Title:      "触发 Cloudflare 验证",
			ActionHint: "降低频率并稍后重试",
			DetailHint: "请求触发了防护挑战，建议稍后再试或更换稳定站点。",
		}
	}

	if strings.Contains(msgLower, "不支持") ||
		strings.Contains(msgLower, "unsupported") ||
		strings.Contains(msgLower, "no checkin method") ||
		strings.Contains(msgLower, "not support checkin") ||
		strings.Contains(msgLower, "check-in is not supported") ||
		strings.Contains(msgLower, "checkin is not supported") ||
		strings.Contains(msgLower, "does not support checkin") ||
		strings.Contains(msgLower, "checkin endpoint not found") ||
		strings.Contains(msgLower, "invalid url (post /api/user/checkin)") {
		return &FailureReason{
			Code:       "CHECKIN_NOT_SUPPORTED",
			Category:   "site",
			Title:      "站点未开启签到",
			ActionHint: "无需重试（非故障）",
			DetailHint: "该站点未提供签到端点，账号会被自动跳过。",
		}
	}

	if strings.Contains(msgLower, "timeout") ||
		strings.Contains(msgLower, "timed out") ||
		strings.Contains(msgLower, "请求超时") ||
		strings.Contains(msgLower, "connection refused") ||
		strings.Contains(msgLower, "network") ||
		strings.Contains(msgLower, "malformed http response") {
		return &FailureReason{
			Code:       "NETWORK_ERROR",
			Category:   "network",
			Title:      "网络错误",
			ActionHint: "稍后重试并检查网络/代理",
			DetailHint: "请求未正常完成，可能是网络波动、代理配置错误或站点响应异常。",
		}
	}

	if strings.Contains(msgLower, "http 5") ||
		strings.Contains(msgLower, "upstream") ||
		strings.Contains(msgLower, "internal server error") {
		return &FailureReason{
			Code:       "UPSTREAM_ERROR",
			Category:   "site",
			Title:      "上游站点错误",
			ActionHint: "稍后重试",
			DetailHint: "站点返回服务端错误，通常需要站点恢复后才可成功。",
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
