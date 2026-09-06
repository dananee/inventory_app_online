package handlers

import (
	"encoding/json"
	"net/http"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// HandleGetActivities returns log feed at /api/activities
func (ctx *AppContext) HandleGetActivities(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	logs, err := db.GetAllActivities(ctx.DB)
	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch activities"})
		return
	}
	middleware.WriteJSON(w, http.StatusOK, logs)
}

// HandleLogActivity logs a new activity event at /api/activities/log
func (ctx *AppContext) HandleLogActivity(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	var act models.ActivityLogDB
	if err := json.NewDecoder(r.Body).Decode(&act); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid log payload"})
		return
	}

	if err := db.LogActivity(ctx.DB, act); err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to record activity log"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, act)
}
