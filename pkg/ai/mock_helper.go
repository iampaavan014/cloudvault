package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// StartMockAIService starts a local HTTP server that mimics the Python AI service
func StartMockAIService(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/predict":
			var reqBody struct {
				History []float64 `json:"history"`
			}
			json.NewDecoder(r.Body).Decode(&reqBody)

			prediction := 0.1 // Default growth
			if len(reqBody.History) > 0 {
				sum := 0.0
				for _, h := range reqBody.History {
					sum += h
				}
				prediction = sum / float64(len(reqBody.History))
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"prediction": prediction,
				"accuracy":   0.99,
				"latency_ms": 1.2,
			})
		case "/decide":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"action_index": 1,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Point the AI package to our mock server
	oldURL := getAIServiceURL()
	SetAIServiceURL(server.URL)
	updateHealthStatus() // Update parity

	t.Cleanup(func() {
		server.Close()
		SetAIServiceURL(oldURL)
	})

	return server
}
