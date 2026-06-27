package handler

import (
	"fmt"
	"metapi/aggrsite/db"
	"net/http"
)

// BatchAccounts handles bulk operations on accounts.
func BatchAccounts(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}

	if len(input.IDs) == 0 {
		fail(w, http.StatusBadRequest, "ids is required")
		return
	}

	var successIDs []int64
	var failedItems []map[string]interface{}

	for _, id := range input.IDs {
		var err error
		switch input.Action {
		case "enable":
			err = db.UpdateAccount(id, map[string]interface{}{"status": "active"})
		case "disable":
			err = db.UpdateAccount(id, map[string]interface{}{"status": "disabled"})
		case "delete":
			err = db.DeleteAccount(id)
		case "checkin":
			// Handled async or individually elsewhere, but we can do it synchronous for now or just call service
			// Since we want to keep it simple, we will return an error that this should be done via checkin all
			err = fmt.Errorf("use checkin all endpoint")
		case "refreshBalance":
			err = fmt.Errorf("use refresh balance endpoint")
		default:
			fail(w, http.StatusBadRequest, "unsupported action: "+input.Action)
			return
		}

		if err != nil {
			failedItems = append(failedItems, map[string]interface{}{
				"id":      id,
				"message": err.Error(),
			})
		} else {
			successIDs = append(successIDs, id)
		}
	}

	ok(w, map[string]interface{}{
		"successIds":  successIDs,
		"failedItems": failedItems,
	})
}
