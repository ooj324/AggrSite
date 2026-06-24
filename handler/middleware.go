package handler

import (
	"crypto/subtle"
	"metapi/aggrsite/config"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	failedAuthAttempts = make(map[string]int)
	lockoutTime        = make(map[string]time.Time)
	authMu             sync.Mutex
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if colon := strings.LastIndex(ip, ":"); colon != -1 {
			ip = ip[:colon]
		}

		authMu.Lock()
		if lockout, exists := lockoutTime[ip]; exists {
			if time.Now().Before(lockout) {
				authMu.Unlock()
				fail(w, http.StatusTooManyRequests, "Too many failed attempts. Please try again later.")
				return
			}
			// Lockout expired, reset attempts
			delete(lockoutTime, ip)
			delete(failedAuthAttempts, ip)
		}
		authMu.Unlock()

		token := r.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
		token = strings.TrimSpace(token)

		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(config.C.AuthToken)) != 1 {
			authMu.Lock()
			failedAuthAttempts[ip]++
			if failedAuthAttempts[ip] >= 5 {
				lockoutTime[ip] = time.Now().Add(10 * time.Minute)
			}
			authMu.Unlock()

			fail(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		authMu.Lock()
		if failedAuthAttempts[ip] > 0 {
			delete(failedAuthAttempts, ip)
			delete(lockoutTime, ip)
		}
		authMu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
