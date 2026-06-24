package handler

import (
	"metapi/aggrsite/db"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func ListAccounts(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt64Ptr(r, "siteId")
	accounts, err := db.ListAccounts(siteID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, accounts)
}

func GetAccount(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	account, err := db.GetAccount(id)
	if err != nil {
		fail(w, http.StatusNotFound, "account not found")
		return
	}
	ok(w, account)
}

func CreateAccount(w http.ResponseWriter, r *http.Request) {
	var input db.CreateAccountInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.SiteID == 0 || input.AccessToken == "" {
		fail(w, http.StatusBadRequest, "site_id and access_token are required")
		return
	}

	id, err := db.CreateAccount(input)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	account, _ := db.GetAccount(id)
	created(w, account)
}

func UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	var fields map[string]interface{}
	if err := parseBody(r, &fields); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	delete(fields, "id")
	delete(fields, "created_at")

	if err := db.UpdateAccount(id, fields); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	account, _ := db.GetAccount(id)
	ok(w, account)
}

func DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := db.DeleteAccount(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"deleted": true})
}

// ---- Account Tokens ----

func ListAccountTokens(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid account id")
		return
	}

	tokens, err := db.ListAccountTokens(accountID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, tokens)
}

type createTokenInput struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

func CreateAccountToken(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid account id")
		return
	}

	var input createTokenInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.Name == "" || input.Token == "" {
		fail(w, http.StatusBadRequest, "name and token are required")
		return
	}

	id, err := db.CreateAccountToken(accountID, input.Name, input.Token)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"id": id})
}

func DeleteAccountToken(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := db.DeleteAccountToken(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"deleted": true})
}
