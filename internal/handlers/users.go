package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// HandleGetUsers returns all registered users at /api/users
func (ctx *AppContext) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	users, err := db.GetAllUsers(ctx.DB)
	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch users"})
		return
	}
	middleware.WriteJSON(w, http.StatusOK, users)
}

// HandleAddUser creates or updates a user account at /api/users/add
func (ctx *AppContext) HandleAddUser(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	var req struct {
		models.UserDB
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid user payload"})
		return
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("usr_%d", time.Now().UnixNano()/1e6)
	}
	if req.CreatedAt == "" {
		req.CreatedAt = time.Now().Format("2006-01-02")
	}
	if req.Status == "" {
		req.Status = "Active"
	}

	if err := db.UpsertUser(ctx.DB, req.UserDB, req.Password); err != nil {
		log.Printf("Error adding user: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save user"})
		return
	}

	log.Printf("User account created/updated: %s (%s)", req.Name, req.Email)
	middleware.WriteJSON(w, http.StatusOK, req.UserDB)
}

// HandleUpdateUserStatus updates status at /api/users/status
func (ctx *AppContext) HandleUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User ID and status required"})
		return
	}

	if err := db.UpdateUserStatus(ctx.DB, req.ID, req.Status); err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update user status"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": req.ID, "userStatus": req.Status})
}

// HandleDeleteUser removes a user at /api/users/delete
func (ctx *AppContext) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		var payload struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		id = payload.ID
	}

	if id == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User ID required"})
		return
	}

	if err := db.DeleteUserByID(ctx.DB, id); err != nil {
		middleware.WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}
