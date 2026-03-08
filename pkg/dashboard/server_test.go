package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/governance"
	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/lifecycle"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestHandlePVCs(t *testing.T) {
	s := &Server{
		store: &MetricsStore{
			Metrics: []types.PVCMetric{
				{Name: "pvc-1", Namespace: "default"},
			},
		},
	}

	req, _ := http.NewRequest("GET", "/api/pvc", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.handlePVCs)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var metrics []types.PVCMetric
	json.Unmarshal(rr.Body.Bytes(), &metrics)
	assert.Equal(t, 1, len(metrics))
	assert.Equal(t, "pvc-1", metrics[0].Name)
}

func TestHandleCost(t *testing.T) {
	s := &Server{
		store: &MetricsStore{
			Summary: types.CostSummary{
				TotalMonthlyCost: 150.50,
			},
		},
	}

	req, _ := http.NewRequest("GET", "/api/cost", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.handleCost)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var summary types.CostSummary
	json.Unmarshal(rr.Body.Bytes(), &summary)
	assert.Equal(t, 150.50, summary.TotalMonthlyCost)
}

func TestHandleBudget(t *testing.T) {
	s := &Server{
		store: &MetricsStore{
			BudgetLimit: 500,
		},
	}

	// Test GET
	req, _ := http.NewRequest("GET", "/api/budget", nil)
	rr := httptest.NewRecorder()
	s.handleBudget(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Test POST
	// Note: We'd need a body for POST, skipping for brevity in this snippet
}

func TestHandleRecommendations(t *testing.T) {
	s := &Server{
		store: &MetricsStore{
			Recommendations: []types.Recommendation{
				{PVC: "pvc-1", Type: "resize"},
			},
		},
	}

	req, _ := http.NewRequest("GET", "/api/recommendations", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(s.handleRecommendations)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var recs []types.Recommendation
	json.Unmarshal(rr.Body.Bytes(), &recs)
	assert.Equal(t, 1, len(recs))
}

func TestHandleNetwork(t *testing.T) {
	s := &Server{
		store: &MetricsStore{},
	}

	req, _ := http.NewRequest("GET", "/api/network", nil)
	rr := httptest.NewRecorder()
	s.handleNetwork(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleGovernanceStatus(t *testing.T) {
	s := &Server{
		orchestrator: lifecycle.NewLifecycleController(0, nil, nil, nil, nil),
		governance:   governance.NewAdmissionController(),
		store:        &MetricsStore{},
	}

	req, _ := http.NewRequest("GET", "/api/governance/status", nil)
	rr := httptest.NewRecorder()
	s.handleGovernanceStatus(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
