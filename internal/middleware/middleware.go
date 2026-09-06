package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// statusResponseWriter wraps http.ResponseWriter to capture the HTTP status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestLogger middleware logs HTTP requests with endpoint, status code, duration, and ANSI colors
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// ANSI Color codes
		const reset = "\033[0m"
		const cyan = "\033[36m"
		const yellow = "\033[33m"
		const purple = "\033[35m"
		const bold = "\033[1m"
		green := "\033[32m"
		red := "\033[31m"

		statusColor := green
		if rw.statusCode >= 400 && rw.statusCode < 500 {
			statusColor = red
		} else if rw.statusCode >= 500 {
			statusColor = yellow
		} else if rw.statusCode >= 300 && rw.statusCode < 400 {
			statusColor = cyan
		}

		timestamp := start.Format("15:04:05")
		endpoint := r.URL.Path
		queryParamsStr := ""
		if r.URL.RawQuery != "" {
			endpoint += "?" + purple + r.URL.RawQuery + reset
			// Format query parameters into key=value map format
			params := r.URL.Query()
			var paramPairs []string
			for key, vals := range params {
				paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", key, strings.Join(vals, ",")))
			}
			queryParamsStr = fmt.Sprintf(" %sparams=[%s]%s", purple, strings.Join(paramPairs, " "), reset)
		}

		log.Printf("%s[%s]%s %s%s%s %s%s -> %s%d%s (%v)",
			cyan, timestamp, reset,
			bold, r.Method, reset,
			endpoint, queryParamsStr,
			statusColor, rw.statusCode, reset,
			duration.Round(time.Microsecond),
		)
	})
}

// EnableCORS sets standard CORS headers for cross-origin local network clients (e.g. Flutter mobile apps, external browsers)
func EnableCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}

	return false
}

// WriteJSON returns formatted JSON payloads with appropriate headers
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}
