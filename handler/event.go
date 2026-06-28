package handler

import (
	"metapi/aggrsite/db"
	"metapi/aggrsite/service"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func ListEvents(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(queryInt(r, "limit", 50), 10, 200)
	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	filter := db.EventFilter{
		Level: r.URL.Query().Get("level"),
		Type:  r.URL.Query().Get("type"),
		Start: r.URL.Query().Get("start"),
		End:   r.URL.Query().Get("end"),
	}

	events, total, err := db.ListEvents(filter, limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    events,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func MarkAllEventsRead(w http.ResponseWriter, r *http.Request) {
	if err := db.MarkAllEventsRead(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]interface{}{"updated": true})
}

// ---- Settings ----

func GetSetting(w http.ResponseWriter, r *http.Request) {
	key := db.NormalizeSettingKey(chi.URLParam(r, "key"))
	if key == "" {
		fail(w, http.StatusBadRequest, "key is required")
		return
	}

	setting, err := db.GetSetting(key)
	if err != nil {
		fail(w, http.StatusNotFound, "setting not found")
		return
	}
	if setting.Value != nil && isSchedulerSettingKey(key) {
		value := service.SettingStringValue(*setting.Value)
		setting.Value = &value
	}
	ok(w, setting)
}

type updateSettingInput struct {
	Value string `json:"value"`
}

func UpdateSetting(w http.ResponseWriter, r *http.Request) {
	key := db.NormalizeSettingKey(chi.URLParam(r, "key"))
	if key == "" {
		fail(w, http.StatusBadRequest, "key is required")
		return
	}

	var input updateSettingInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}

	input.Value = strings.TrimSpace(input.Value)
	if isSchedulerSettingKey(key) {
		if err := service.ValidateCronExpr(input.Value); err != nil {
			fail(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
			return
		}
	}

	if err := db.UpsertSetting(key, input.Value); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	if isSchedulerSettingKey(key) {
		service.ReloadScheduler()
	}

	ok(w, map[string]interface{}{"key": key, "value": input.Value})
}

func isSchedulerSettingKey(key string) bool {
	return key == "checkin_cron" || key == "balance_refresh_cron"
}
