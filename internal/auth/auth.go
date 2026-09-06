package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenerateActivationKey computes the SHA-256 activation key for a given UTC timestamp and secret phrase.
// It reproduces the exact algorithm from generate_activationkey.py:
// 1. Formats date "YYYY-MM-DD" and time "HH:MM" (UTC 24-hour format)
// 2. Computes SHA-256 of "YYYY-MM-DD_HH:MM_phrase"
// 3. Returns uppercase 16-hex characters in 4 blocks of 4: "XXXX-XXXX-XXXX-XXXX"
func GenerateActivationKey(t time.Time, phrase string) string {
	if phrase == "" {
		phrase = "GO-INVENTORY"
	}
	utcNow := t.UTC()
	dateStr := utcNow.Format("2006-01-02")
	hourMinStr := utcNow.Format("15:04")
	rawInput := fmt.Sprintf("%s_%s_%s", dateStr, hourMinStr, phrase)

	hash := sha256.Sum256([]byte(rawInput))
	hashHex := strings.ToUpper(hex.EncodeToString(hash[:]))

	return fmt.Sprintf("%s-%s-%s-%s", hashHex[0:4], hashHex[4:8], hashHex[8:12], hashHex[12:16])
}

// VerifyActivationKey validates an activation key against UTC rolling time windows (default 60 minutes)
func VerifyActivationKey(inputKey string, windowMinutes int) (bool, string) {
	cleanKey := strings.TrimSpace(strings.ToUpper(inputKey))
	if cleanKey == "" {
		return false, ""
	}

	if windowMinutes <= 0 {
		windowMinutes = 60
	}

	now := time.Now().UTC()

	// 1. Check SHA-256 16-char format (XXXX-XXXX-XXXX-XXXX) for rolling UTC minutes
	for i := 0; i <= windowMinutes; i++ {
		tPast := now.Add(-time.Duration(i) * time.Minute)
		keyPast := GenerateActivationKey(tPast, "GO-INVENTORY")
		if cleanKey == keyPast {
			return true, keyPast
		}

		if i > 0 && i <= 10 {
			tFut := now.Add(time.Duration(i) * time.Minute)
			keyFut := GenerateActivationKey(tFut, "GO-INVENTORY")
			if cleanKey == keyFut {
				return true, keyFut
			}
		}
	}

	// 2. Check legacy GO-INVENTORY-MM-DD-YY-HH-MM format (with typo tolerance)
	normalized := cleanKey
	if strings.HasPrefix(normalized, "GO-INVETORY-") {
		normalized = "GO-INVENTORY-" + strings.TrimPrefix(normalized, "GO-INVETORY-")
	} else if strings.HasPrefix(normalized, "GOINVENTORY-") {
		normalized = "GO-INVENTORY-" + strings.TrimPrefix(normalized, "GOINVENTORY-")
	}

	if strings.HasPrefix(normalized, "GO-INVENTORY-") {
		timePart := strings.TrimPrefix(normalized, "GO-INVENTORY-")
		parsedTime, err := time.Parse("01-02-06-15-04", timePart)
		if err == nil {
			diff := time.Since(parsedTime)
			if diff < 0 {
				diff = -diff
			}
			if diff <= time.Duration(windowMinutes)*time.Minute {
				return true, normalized
			}
		}
	}

	return false, ""
}
