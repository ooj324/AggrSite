package handler

import (
	"metapi/aggrsite/db"
	"metapi/aggrsite/service"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func CheckinAll(w http.ResponseWriter, r *http.Request) {
	results, err := service.CheckinAll()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, results)
}

func CheckinAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "accountId"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid accountId")
		return
	}

	result, err := service.CheckinAccount(accountID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, result)
}

func ListCheckinLogs(w http.ResponseWriter, r *http.Request) {
	accountID := queryInt64Ptr(r, "accountId")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	logs, total, err := db.ListCheckinLogs(accountID, limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    logs,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
