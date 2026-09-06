package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// HandleGetSuppliers returns all suppliers at /api/suppliers
func (ctx *AppContext) HandleGetSuppliers(w http.ResponseWriter, r *http.Request) {
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
	if storeIDStr == "" {
		storeIDStr = r.Header.Get("X-Store-ID")
	}
	storeID, _ := strconv.Atoi(storeIDStr)

	suppliers, err := db.GetSuppliersByStore(ctx.DB, storeID)
	if err != nil {
		log.Printf("Error fetching suppliers: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch suppliers"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, suppliers)
}

// HandleUpdateSupplier updates or inserts a supplier at /api/suppliers/update
func (ctx *AppContext) HandleUpdateSupplier(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var s models.Supplier
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		log.Printf("Error decoding supplier payload: %v", err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid supplier payload format"})
		return
	}

	if s.StoreID <= 0 {
		storeIDStr := r.URL.Query().Get("store_id")
		if storeIDStr == "" {
			storeIDStr = r.URL.Query().Get("storeId")
		}
		if storeIDStr == "" {
			storeIDStr = r.Header.Get("X-Store-ID")
		}
		storeID, _ := strconv.Atoi(storeIDStr)
		if storeID > 0 {
			s.StoreID = storeID
		}
	}

	id, err := db.UpsertSupplier(ctx.DB, s)
	if err != nil {
		log.Printf("Error upserting supplier: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save supplier"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Supplier saved successfully",
		"id":      id,
	})
}

// HandleDeleteSupplier removes a supplier at /api/suppliers/delete
func (ctx *AppContext) HandleDeleteSupplier(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Valid ID parameter is required"})
		return
	}

	if err := db.DeleteSupplierByID(ctx.DB, id); err != nil {
		log.Printf("Error deleting supplier ID %d: %v", id, err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete supplier"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}
