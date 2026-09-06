package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// HandleGetIPs returns the server's local IP addresses at /api/server/ip
func (ctx *AppContext) HandleGetIPs(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Could not discover local interfaces: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to discover local interfaces"})
		return
	}

	var ips []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ips":  ips,
		"port": ctx.Port,
	})
}

// HandleLogin authenticates user credentials at /api/auth/login
func (ctx *AppContext) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	loginInput := strings.TrimSpace(req.Email)
	if loginInput == "" {
		loginInput = strings.TrimSpace(req.Phone)
	}

	if loginInput == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Email or phone number is required"})
		return
	}

	user, token, err := db.AuthenticateUser(ctx.DB, loginInput, req.Password)
	if err != nil {
		if err.Error() == "PASSWORD_NOT_CREATED" {
			middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
				"status":             "PASSWORD_NOT_CREATED",
				"mustChangePassword": true,
				"user":               user,
				"message":            "Password is not created yet. Please create a new password.",
			})
			return
		}
		middleware.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("User authenticated: %s (%s)", user.Name, user.Email)
	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// HandleSetPassword sets/creates a new password for a user identified by email or phone.
func (ctx *AppContext) HandleSetPassword(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		NewPassword string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	loginInput := strings.TrimSpace(req.Email)
	if loginInput == "" {
		loginInput = strings.TrimSpace(req.Phone)
	}

	if loginInput == "" || req.NewPassword == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Email or phone and new password required"})
		return
	}

	if len(req.NewPassword) < 6 {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "New password must be at least 6 characters"})
		return
	}

	if err := db.SetUserPassword(ctx.DB, loginInput, req.NewPassword); err != nil {
		log.Printf("Error setting password for %s: %v", loginInput, err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Password created successfully. Please log in with your new password.",
	})
}

// HandleGetMe returns currently authenticated user at /api/auth/me
func (ctx *AppContext) HandleGetMe(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	user, err := db.ValidateSessionToken(ctx.DB, token)
	if err != nil {
		middleware.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized or session expired"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, user)
}

// HandleChangePassword handles password changes at /api/auth/change-password
func (ctx *AppContext) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		UserID      string `json:"userId"`
		FullName    string `json:"fullName"`
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewPassword == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "New password is required"})
		return
	}

	if len(req.NewPassword) < 6 {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "New password must be at least 6 characters"})
		return
	}

	if req.NewPassword == "admin123" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "New password cannot be default 'admin123'"})
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	user, err := db.ValidateSessionToken(ctx.DB, token)
	if err != nil {
		middleware.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized session"})
		return
	}

	targetID := user.ID
	if req.UserID != "" {
		targetID = req.UserID
	}

	if err := db.ChangeUserPassword(ctx.DB, targetID, req.NewPassword, req.FullName); err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update password"})
		return
	}

	log.Printf("Password updated for user ID: %s", targetID)
	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Password successfully updated",
	})
}

// HandleGetSettings returns application configuration synced with store data at /api/settings.
func (ctx *AppContext) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	storeIDStr := r.URL.Query().Get("store_id")
	if storeIDStr == "" {
		storeIDStr = r.URL.Query().Get("storeId")
	}
	storeID := 0
	fmt.Sscanf(storeIDStr, "%d", &storeID)

	// If no explicit store_id query param was passed, check user session token
	if storeID <= 0 {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != "" {
			if user, err := db.ValidateSessionToken(ctx.DB, token); err == nil && user != nil && user.StoreID > 0 {
				storeID = user.StoreID
			}
		}
	}

	if storeID <= 0 {
		storeID = 1
	}

	settings, err := db.GetSettingsForStore(ctx.DB, storeID)
	if err != nil {
		log.Printf("Error fetching settings for store %d: %v", storeID, err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch application settings"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, settings)
}

// HandleUpdateSettings updates application configuration at /api/settings/update
func (ctx *AppContext) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var s models.Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		log.Printf("Error decoding settings update: %v", err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid settings payload format"})
		return
	}

	if s.StoreID <= 0 {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != "" {
			if user, err := db.ValidateSessionToken(ctx.DB, token); err == nil && user != nil && user.StoreID > 0 {
				s.StoreID = user.StoreID
			}
		}
	}

	if s.StoreID <= 0 {
		s.StoreID = 1
	}

	if err := db.UpdateSettings(ctx.DB, s); err != nil {
		log.Printf("Error updating settings: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update application settings"})
		return
	}

	log.Printf("Application settings successfully updated.")
	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Settings updated successfully",
	})
}
