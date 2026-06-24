package handler

import (
	"metapi/aggrsite/db"
	"net/http"
)

func ListSites(w http.ResponseWriter, r *http.Request) {
	sites, err := db.ListSites()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, sites)
}

func GetSite(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	site, err := db.GetSite(id)
	if err != nil {
		fail(w, http.StatusNotFound, "site not found")
		return
	}
	ok(w, site)
}

func CreateSite(w http.ResponseWriter, r *http.Request) {
	var input db.CreateSiteInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.Name == "" || input.URL == "" || input.Platform == "" {
		fail(w, http.StatusBadRequest, "name, url, platform are required")
		return
	}

	id, err := db.CreateSite(input)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	site, _ := db.GetSite(id)
	created(w, site)
}

func UpdateSite(w http.ResponseWriter, r *http.Request) {
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

	// Prevent updating the id
	delete(fields, "id")
	delete(fields, "created_at")

	if err := db.UpdateSite(id, fields); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	site, _ := db.GetSite(id)
	ok(w, site)
}

func DeleteSite(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := db.DeleteSite(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"deleted": true})
}
