package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/collector"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

func TestNewServer(t *testing.T) {
	server := NewServer(nil, nil, "aws", true)

	if server == nil {
		t.Fatal("NewServer() returned nil")
	}

	if server.provider != "aws" {
		t.Errorf("Expected provider 'aws', got '%s'", server.provider)
	}

	if !server.mock {
		t.Error("Expected mock mode to be true")
	}
}

func TestHandlePVCs_MockMode(t *testing.T) {
	server := NewServer(nil, nil, "aws", true)
	server.reconcile()

	req := httptest.NewRequest("GET", "/api/pvc", nil)
	w := httptest.NewRecorder()

	server.handlePVCs(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Decode response
	var metrics []types.PVCMetric
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Mock collector should return some metrics
	if len(metrics) == 0 {
		t.Error("Expected at least one metric from mock collector")
	}

	// Verify costs are calculated
	for _, m := range metrics {
		if m.MonthlyCost == 0 {
			t.Error("Expected MonthlyCost to be calculated")
		}
	}
}

func TestHandleCost_MockMode(t *testing.T) {
	server := NewServer(nil, nil, "aws", true)
	server.reconcile()

	req := httptest.NewRequest("GET", "/api/cost", nil)
	w := httptest.NewRecorder()

	server.handleCost(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Decode response
	var summary types.CostSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify summary has data
	if summary.TotalMonthlyCost == 0 {
		t.Error("Expected TotalMonthlyCost to be non-zero")
	}

	if len(summary.ByNamespace) == 0 {
		t.Error("Expected ByNamespace to have data")
	}

	if len(summary.ByStorageClass) == 0 {
		t.Error("Expected ByStorageClass to have data")
	}
}

func TestHandleRecommendations_MockMode(t *testing.T) {
	server := NewServer(nil, nil, "aws", true)
	server.reconcile()

	req := httptest.NewRequest("GET", "/api/recommendations", nil)
	w := httptest.NewRecorder()

	server.handleRecommendations(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Decode response
	var recommendations []types.Recommendation
	if err := json.NewDecoder(resp.Body).Decode(&recommendations); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Mock data should generate some recommendations
	if len(recommendations) == 0 {
		t.Log("Warning: No recommendations generated from mock data")
	}

	// Verify recommendation structure
	for _, rec := range recommendations {
		if rec.Type == "" {
			t.Error("Recommendation Type should not be empty")
		}
		if rec.PVC == "" {
			t.Error("Recommendation PVC should not be empty")
		}
		if rec.Namespace == "" {
			t.Error("Recommendation Namespace should not be empty")
		}
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{
		"key": "value",
	}

	writeJSON(w, data)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected 'value', got '%s'", result["key"])
	}
}

func TestMockCollectorIntegration(t *testing.T) {
	// Test that mock collector works correctly
	mockCollector := collector.NewMockPVCCollector()

	ctx := context.Background()
	metrics, err := mockCollector.CollectAll(ctx)

	if err != nil {
		t.Fatalf("Mock collector failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected mock metrics, got empty slice")
	}

	// Verify mock data structure
	for _, m := range metrics {
		if m.Name == "" {
			t.Error("Mock metric Name should not be empty")
		}
		if m.Namespace == "" {
			t.Error("Mock metric Namespace should not be empty")
		}
		if m.SizeBytes == 0 {
			t.Error("Mock metric SizeBytes should not be zero")
		}
	}
}
