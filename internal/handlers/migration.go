package handlers

import (
	"encoding/json"
	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
	"net/http"
	"strconv"
)

func (ctx *AppContext) HandleMigrationDimensions(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req models.DimensionsMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload format"})
		return
	}

	storeID, _ := strconv.Atoi(r.URL.Query().Get("storeId"))

	res, err := db.ProcessBulkDimensions(ctx.DB, storeID, req)
	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, res)
}

func (ctx *AppContext) HandleMigrationProducts(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req models.ProductsMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload format"})
		return
	}

	storeID, _ := strconv.Atoi(r.URL.Query().Get("storeId"))

	res, err := db.ProcessBulkProducts(ctx.DB, storeID, req)
	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, res)
}

func (ctx *AppContext) HandleMigrationReceipts(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req models.ReceiptsMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload format"})
		return
	}

	storeID, _ := strconv.Atoi(r.URL.Query().Get("storeId"))

	if err := db.ProcessBulkReceipts(ctx.DB, storeID, req); err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Receipts migrated successfully"})
}
