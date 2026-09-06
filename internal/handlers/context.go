package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// AppContext maintains references to system-wide resources like our active WAL database
type AppContext struct {
	DB   *sql.DB
	Port int
}

// AuthenticateRequest extracts the session token, validates it against SQLite, and returns the authenticated user.
func (ctx *AppContext) AuthenticateRequest(w http.ResponseWriter, r *http.Request) (*models.UserDB, error) {
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	if token == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error": "Authentication required. Missing authorization token.",
		})
		return nil, fmt.Errorf("missing token")
	}

	user, err := db.ValidateSessionToken(ctx.DB, token)
	if err != nil {
		middleware.WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error": "Invalid or expired session token. Please log in again.",
		})
		return nil, err
	}

	if user.Status != "Active" {
		middleware.WriteJSON(w, http.StatusForbidden, map[string]interface{}{
			"error": "User account is inactive or suspended.",
		})
		return nil, fmt.Errorf("user inactive")
	}

	return user, nil
}

// RequirePermission wraps an HTTP handler function with RBAC permission validation.
func (ctx *AppContext) RequirePermission(perm models.Permission, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if middleware.EnableCORS(w, r) {
			return
		}

		user, err := ctx.AuthenticateRequest(w, r)
		if err != nil {
			return
		}

		if !models.HasPermission(user.Role, perm) {
			fmt.Printf("[RBAC Audit] Permission DENIED for user %s (%s) requesting %s (Requires %s)\n", user.Email, user.Role, r.URL.Path, perm)
			middleware.WriteJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":              fmt.Sprintf("Access denied: Permission '%s' required for role '%s'", perm, user.Role),
				"requiredPermission": string(perm),
				"userRole":           user.Role,
			})
			return
		}

		handler(w, r)
	}
}
