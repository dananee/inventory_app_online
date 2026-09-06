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

// HandleGetStores lists all stores/businesses
func (ctx *AppContext) HandleGetStores(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	stores, err := db.GetAllStores(ctx.DB)
	if err != nil {
		log.Printf("Error fetching stores: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch stores"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, stores)
}

// HandleUpdateStore creates or updates a store/business
func (ctx *AppContext) HandleUpdateStore(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var store models.StoreDB
	if err := json.NewDecoder(r.Body).Decode(&store); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if strings.TrimSpace(store.Name) == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Store name is required"})
		return
	}

	if err := db.CreateOrUpdateStore(ctx.DB, &store); err != nil {
		log.Printf("Error saving store: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to save store: %v", err)})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, store)
}

// HandleDeleteStore deletes a store/business
func (ctx *AppContext) HandleDeleteStore(w http.ResponseWriter, r *http.Request) {
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
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid store ID"})
		return
	}

	if err := db.DeleteStoreByID(ctx.DB, id); err != nil {
		log.Printf("Error deleting store %d: %v", id, err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Store deleted successfully"})
}
