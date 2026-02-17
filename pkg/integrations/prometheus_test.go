package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewPrometheusClient(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		expectErr bool
	}{
		{
			name:      "Valid URL",
			url:       "http://localhost:9090",
			expectErr: false,
		},
		{
			name:      "Empty URL",
			url:       "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewPrometheusClient(tt.url)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if client == nil {
				t.Error("Expected client, got nil")
				return
			}

			if client.baseURL != tt.url {
				t.Errorf("Expected baseURL '%s', got '%s'", tt.url, client.baseURL)
			}
		})
	}
}

func TestQueryScalar_Success(t *testing.T) {
	// Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameter
		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("Expected query parameter")
		}

		// Return mock response
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{},
						"value":  []interface{}{float64(time.Now().Unix()), "12345.67"},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewPrometheusClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	value, err := client.queryScalar(ctx, "test_metric")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := 12345.67
	if value != expected {
		t.Errorf("Expected value %v, got %v", expected, value)
	}
}

func TestQueryScalar_NoData(t *testing.T) {
	// Create mock server returning empty result
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result":     []map[string]interface{}{},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewPrometheusClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	_, err = client.queryScalar(ctx, "missing_metric")

	if err == nil {
		t.Error("Expected error for empty result, got nil")
	}
}

func TestQueryScalar_HTTPError(t *testing.T) {
	// Create mock server returning HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewPrometheusClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	_, err = client.queryScalar(ctx, "test_metric")

	if err == nil {
		t.Error("Expected error for HTTP 500, got nil")
	}
}

func TestGetPVCMetrics_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{},
						"value":  []interface{}{float64(time.Now().Unix()), "53687091200"}, // 50GB
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewPrometheusClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	metrics, err := client.GetPVCMetrics(ctx, "test-pvc", "default")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}

	expectedBytes := int64(53687091200)
	if metrics.UsedBytes != expectedBytes {
		t.Errorf("Expected UsedBytes %d, got %d", expectedBytes, metrics.UsedBytes)
	}

	// Should have set LastActivity since metric exists
	if metrics.LastActivity.IsZero() {
		t.Error("Expected LastActivity to be set")
	}
}

func TestGetPVCMetrics_NoData(t *testing.T) {
	// Create mock server returning empty result
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result":     []map[string]interface{}{},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewPrometheusClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	metrics, err := client.GetPVCMetrics(ctx, "missing-pvc", "default")

	// Should not error, but metrics should have zero values
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if metrics.UsedBytes != 0 {
		t.Errorf("Expected UsedBytes 0, got %d", metrics.UsedBytes)
	}

	// LastActivity should be zero since no data
	if !metrics.LastActivity.IsZero() {
		t.Error("Expected LastActivity to be zero")
	}
}

func TestQueryScalar_InvalidJSON(t *testing.T) {
	// Create mock server returning invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, err := NewPrometheusClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	_, err = client.queryScalar(ctx, "test_metric")

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
