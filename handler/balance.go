package handler

import (
	"metapi/aggrsite/service"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func RefreshAllBalances(w http.ResponseWriter, r *http.Request) {
	results, err := service.RefreshAllBalances()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, results)
}

func RefreshBalance(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "accountId"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid accountId")
		return
	}

	result, err := service.RefreshBalance(accountID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, result)
}
