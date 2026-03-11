package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func prometheusVectorResponse(metrics []map[string]string, values []string) string {
	type result struct {
		Metric map[string]string `json:"metric"`
		Value  []interface{}     `json:"value"`
	}
	results := make([]result, len(metrics))
	for i, m := range metrics {
		results[i] = result{Metric: m, Value: []interface{}{1234567890.0, values[i]}}
	}
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result":     results,
		},
	})
	return string(data)
}

func prometheusScalarResponse(value string) string {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result": []map[string]interface{}{
				{"value": []interface{}{1234567890.0, value}},
			},
		},
	})
	return string(data)
}

func TestQueryScalar_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(prometheusScalarResponse("42.5")))
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	val, err := client.queryScalar(context.Background(), `sum(kubelet_volume_stats_used_bytes)`)
	require.NoError(t, err)
	assert.InDelta(t, 42.5, val, 0.001)
}

func TestQueryScalar_NoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"data":   map[string]interface{}{"resultType": "vector", "result": []interface{}{}},
		})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	_, err := client.queryScalar(context.Background(), `absent(missing_metric)`)
	assert.Error(t, err)
}

func TestQueryScalar_FailStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	_, err := client.queryScalar(context.Background(), `up`)
	assert.Error(t, err)
}

func TestQueryVector_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	_, err := client.queryVector(context.Background(), `up`)
	assert.Error(t, err)
}

func TestQueryVector_FailStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	_, err := client.queryVector(context.Background(), `up`)
	assert.Error(t, err)
}

func TestQueryVector_StatusNotSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := json.Marshal(map[string]interface{}{"status": "error", "error": "bad query"})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	_, err := client.queryVector(context.Background(), `up`)
	assert.Error(t, err)
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

func TestGetAllPVCMetrics_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(prometheusVectorResponse(
			[]map[string]string{
				{"persistentvolumeclaim": "pvc-1", "namespace": "default"},
				{"persistentvolumeclaim": "pvc-2", "namespace": "kube-system"},
			},
			[]string{"1073741824", "2147483648"},
		)))
	}))
	defer srv.Close()

	client, err := NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	metrics, err := client.GetAllPVCMetrics(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1073741824), metrics["default"]["pvc-1"].UsedBytes)
	assert.Equal(t, int64(2147483648), metrics["kube-system"]["pvc-2"].UsedBytes)
}

func TestGetAllPVCMetrics_EmptyLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Result with missing labels — should be skipped
		_, _ = w.Write([]byte(prometheusVectorResponse(
			[]map[string]string{{}},
			[]string{"100"},
		)))
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	metrics, err := client.GetAllPVCMetrics(context.Background())
	require.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestGetAllPVCMetrics_PrometheusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	_, err := client.GetAllPVCMetrics(context.Background())
	assert.Error(t, err)
}

func TestGetPVCMetrics_WithActivity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(prometheusScalarResponse("536870912")))
	}))
	defer srv.Close()

	client, _ := NewPrometheusClient(srv.URL)
	m, err := client.GetPVCMetrics(context.Background(), "pvc-1", "default")
	require.NoError(t, err)
	assert.Equal(t, int64(536870912), m.UsedBytes)
	assert.False(t, m.LastActivity.IsZero())
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
