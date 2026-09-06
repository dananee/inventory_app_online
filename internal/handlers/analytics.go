package handlers

import (
	"log"
	"net/http"
	"strconv"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
)

// HandleAnalyticsQuery serves aggregate metrics and time-series points to Chart.js at /api/analytics/dashboard
func (ctx *AppContext) HandleAnalyticsQuery(w http.ResponseWriter, r *http.Request) {
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

	analytics, err := db.GetDashboardAnalyticsForStore(ctx.DB, storeID)
	if err != nil {
		log.Printf("Error computing dashboard analytics: %v", err)
		analytics.DatabaseConnected = false
		middleware.WriteJSON(w, http.StatusInternalServerError, analytics)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, analytics)
}
