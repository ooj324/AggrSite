package handler

import (
	"metapi/aggrsite/platform"
	"metapi/aggrsite/service"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func MountRoutes(r chi.Router) {
	r.Use(CORSMiddleware)

	// Public ping endpoint
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)

		r.Get("/api/platforms", func(w http.ResponseWriter, r *http.Request) {
			ok(w, platform.AllPlatformNames())
		})

		// Sites
		r.Get("/api/sites", ListSites)
		r.Post("/api/sites", CreateSite)
		r.Get("/api/sites/{id}", GetSite)
		r.Put("/api/sites/{id}", UpdateSite)
		r.Delete("/api/sites/{id}", DeleteSite)

		// Accounts
		r.Get("/api/accounts", ListAccounts)
		r.Post("/api/accounts", CreateAccount)
		r.Post("/api/accounts/login", LoginAccount)
		r.Get("/api/accounts/{id}", GetAccount)
		r.Put("/api/accounts/{id}", UpdateAccount)
		r.Delete("/api/accounts/{id}", DeleteAccount)

		// Account Tokens
		r.Get("/api/accounts/{id}/tokens", ListAccountTokens)
		r.Post("/api/accounts/{id}/tokens", CreateAccountToken)
		r.Delete("/api/account-tokens/{id}", DeleteAccountToken)

		// Checkin
		r.Post("/api/checkin/all", CheckinAll)
		r.Post("/api/checkin/{accountId}", CheckinAccount)
		r.Get("/api/checkin/logs", ListCheckinLogs)

		// Balance
		r.Post("/api/balance/refresh/all", RefreshAllBalances)
		r.Post("/api/balance/refresh/{accountId}", RefreshBalance)

		// Events
		r.Get("/api/events", ListEvents)
		r.Put("/api/events/read-all", MarkAllEventsRead)

		r.Get("/api/backup/export", ExportBackup)
		r.Post("/api/backup/import", ImportBackup)

		r.Get("/api/settings/{key}", GetSetting)
		r.Put("/api/settings/{key}", UpdateSetting)

		// Stats
		r.Get("/api/stats/dashboard", GetDashboardStats)

		// Scheduler
		r.Get("/api/scheduler/status", func(w http.ResponseWriter, r *http.Request) {
			ok(w, service.GetSchedulerStatus())
		})
	})
}
