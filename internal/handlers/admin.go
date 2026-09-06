package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// HandleAdminGetUsers lists all users with details for Admin Panel
func (ctx *AppContext) HandleAdminGetUsers(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	users, err := db.GetAllUsersAdmin(ctx.DB)
	if err != nil {
		log.Printf("Error fetching admin users: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch users"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, users)
}

// HandleAdminCreateUser creates a user account by email or phone
func (ctx *AppContext) HandleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Role     string `json:"role"`
		Status   string `json:"status"`
		StoreID  int    `json:"storeId"`
		StoreIDs []int  `json:"storeIds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User name is required"})
		return
	}

	user, err := db.CreateUserAdmin(ctx.DB, req.Name, req.Email, req.Phone, req.Role, req.Status, req.StoreID, req.StoreIDs)
	if err != nil {
		log.Printf("Error creating admin user: %v", err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, user)
}

// HandleAdminSetUserStatus activates or disables a user account
func (ctx *AppContext) HandleAdminSetUserStatus(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		UserID string `json:"userId"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User ID is required"})
		return
	}

	if req.Status != "Active" && req.Status != "Disabled" && req.Status != "Inactive" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Status must be Active or Disabled"})
		return
	}

	if err := db.SetUserStatusAdmin(ctx.DB, req.UserID, req.Status); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": fmt.Sprintf("User status updated to %s", req.Status)})
}

// HandleAdminResetPassword resets a user password (prompts user to set password on next login)
func (ctx *AppContext) HandleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
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

	// Reset password to empty (requires password creation on login)
	res, err := ctx.DB.Exec(`
		UPDATE users
		SET passwordHash = ''
		WHERE (LOWER(email) = LOWER($1) OR (phone IS NOT NULL AND phone != '' AND phone = $1))
	`, loginInput)

	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		middleware.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "User account not found"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Password reset successfully. User will set a new password on login."})
}

// HandleAdminGetPayments lists all payments
func (ctx *AppContext) HandleAdminGetPayments(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	payments, err := db.GetAllPayments(ctx.DB)
	if err != nil {
		log.Printf("Error fetching payments: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch payments"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, payments)
}

// HandleAdminCreatePayment records a subscription payment for 1m, 3m, 6m, 1y
func (ctx *AppContext) HandleAdminCreatePayment(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var p models.PaymentDB
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if strings.TrimSpace(p.UserID) == "" && strings.TrimSpace(p.UserEmail) == "" && strings.TrimSpace(p.UserPhone) == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User selection is required"})
		return
	}

	if strings.TrimSpace(p.PlanDuration) == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Plan duration (1 Month, 3 Months, 6 Months, 1 Year) is required"})
		return
	}

	payment, err := db.CreatePayment(ctx.DB, &p)
	if err != nil {
		log.Printf("Error recording payment: %v", err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, payment)
}

// HandleAdminSummary returns summary metrics for Admin Panel Dashboard
func (ctx *AppContext) HandleAdminSummary(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var totalUsers, activeUsers, disabledUsers int
	_ = ctx.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	_ = ctx.DB.QueryRow("SELECT COUNT(*) FROM users WHERE status = 'Active'").Scan(&activeUsers)
	_ = ctx.DB.QueryRow("SELECT COUNT(*) FROM users WHERE status IN ('Disabled', 'Inactive')").Scan(&disabledUsers)

	var totalRevenue float64
	_ = ctx.DB.QueryRow("SELECT COALESCE(SUM(amount), 0.0) FROM payments WHERE status = 'Paid'").Scan(&totalRevenue)

	var totalPayments int
	_ = ctx.DB.QueryRow("SELECT COUNT(*) FROM payments").Scan(&totalPayments)

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"totalUsers":    totalUsers,
		"activeUsers":   activeUsers,
		"disabledUsers": disabledUsers,
		"totalRevenue":  totalRevenue,
		"totalPayments": totalPayments,
	})
}

// HandleAdminUpdateUser updates existing user details
func (ctx *AppContext) HandleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var user models.UserDB
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil || user.ID == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User ID is required"})
		return
	}

	if err := db.UpdateUserAdmin(ctx.DB, &user); err != nil {
		log.Printf("Error updating user: %v", err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "User updated successfully"})
}

// HandleAdminDeleteUser deletes a user account by ID
func (ctx *AppContext) HandleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userId := r.URL.Query().Get("id")
	if userId == "" {
		var req struct {
			UserID string `json:"userId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		userId = req.UserID
	}

	if userId == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "User ID is required"})
		return
	}

	if err := db.DeleteUserAdmin(ctx.DB, userId); err != nil {
		log.Printf("Error deleting user %s: %v", userId, err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "User account deleted successfully"})
}

// HandleAdminDeletePayment deletes a payment record by ID
func (ctx *AppContext) HandleAdminDeletePayment(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payment ID"})
		return
	}

	if err := db.DeletePaymentByID(ctx.DB, id); err != nil {
		log.Printf("Error deleting payment %d: %v", id, err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Payment record deleted successfully"})
}
