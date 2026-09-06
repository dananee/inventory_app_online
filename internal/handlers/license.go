package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"inventory_app/internal/auth"
	"inventory_app/internal/middleware"
)

// HandleActivateLicense processes the dynamic SHA-256/GO-INVENTORY key and generates an 8-digit PIN at /api/license/activate
func (ctx *AppContext) HandleActivateLicense(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
		return
	}

	rawKey := strings.TrimSpace(strings.ToUpper(req.Key))
	if rawKey == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Activation key is required"})
		return
	}

	isValid, matchedKey := auth.VerifyActivationKey(rawKey, 60)
	if !isValid {
		currentKey := auth.GenerateActivationKey(time.Now(), "GO-INVENTORY")
		currentLegacy := fmt.Sprintf("GO-INVENTORY-%s", time.Now().Format("01-02-06-15-04"))
		log.Printf("[License Activation Failed] Key: '%s'. Current valid SHA-256 key: '%s' (Legacy: '%s')", req.Key, currentKey, currentLegacy)

		middleware.WriteJSON(w, http.StatusForbidden, map[string]interface{}{
			"error":           fmt.Sprintf("Invalid or expired activation key '%s'.", req.Key),
			"inputKey":        req.Key,
			"validKeySample":  currentKey,
			"legacyKeySample": currentLegacy,
			"serverTimeUTC":   time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		})
		return
	}

	// Generate 8-digit activation PIN
	pin := fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)

	_, err := ctx.DB.Exec("UPDATE settings SET activationPin = $1, licenseActivated = 1 WHERE id = 1", pin)
	if err != nil {
		log.Printf("Error saving activation pin: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save activation pin"})
		return
	}

	log.Printf("[License Activated] Successfully activated with key '%s' -> PIN: %s", matchedKey, pin)

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "success",
		"pin":          pin,
		"activatedKey": matchedKey,
		"timestampUTC": time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleVerifyLicense checks the 8-digit PIN for mobile activation at /api/license/verify
func (ctx *AppContext) HandleVerifyLicense(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	pin := r.URL.Query().Get("pin")
	if pin == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "PIN is required"})
		return
	}

	var storedPin string
	var activated int
	err := ctx.DB.QueryRow("SELECT activationPin, licenseActivated FROM settings WHERE id = 1").Scan(&storedPin, &activated)
	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}

	if activated == 1 && storedPin == pin {
		middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
	} else {
		middleware.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "Invalid or inactive PIN"})
	}
}
