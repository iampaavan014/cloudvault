package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/cloudvault-io/cloudvault/pkg/collector"
	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/governance"
	"github.com/cloudvault-io/cloudvault/pkg/orchestrator/lifecycle"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

//go:embed dist/*
var staticFiles embed.FS

// MetricsStore holds the latest cached data
type MetricsStore struct {
	sync.RWMutex
	Metrics         []types.PVCMetric
	Summary         types.CostSummary
	Recommendations []types.Recommendation
	Policies        []v1alpha1.StorageLifecyclePolicy
	LastUpdate      time.Time
}

// Server handles the HTTP server for the dashboard
type Server struct {
	client       *collector.KubernetesClient
	promClient   *integrations.PrometheusClient
	orchestrator *lifecycle.LifecycleController
	governance   *governance.AdmissionController
	provider     string // Cloud provider (aws, gcp, azure)
	mock         bool
	store        *MetricsStore
}

// NewServer creates a new dashboard server
func NewServer(client *collector.KubernetesClient, promClient *integrations.PrometheusClient, provider string, mock bool) *Server {
	// Initialize intelligence layer for dashboard visibility
	recommender := lifecycle.NewIntelligentRecommender(nil) // Dashboard usually read-only for metrics
	return &Server{
		client:       client,
		promClient:   promClient,
		orchestrator: lifecycle.NewLifecycleController(60*time.Second, nil, recommender),
		governance:   governance.NewAdmissionController(),
		provider:     provider,
		mock:         mock,
		store:        &MetricsStore{},
	}
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	// Start background reconciler (Phase 3 ARCHITECTURE MOVE)
	// We refresh metrics every 30 seconds to provide real-time updates without taxing the API
	go s.runReconciler(30 * time.Second)

	// Start Autonomous Orchestrator (Phase 4 ACTIVE ORCHESTRATOR)
	go s.orchestrator.Start(context.Background(), func() []types.PVCMetric {
		s.store.RLock()
		defer s.store.RUnlock()
		return s.store.Metrics
	})

	// Create a new router/mux to avoid polluting default serve mux
	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/api/login", LoginHandler) // Phase 16 Auth
	mux.HandleFunc("/api/pvc", s.handlePVCs)
	mux.HandleFunc("/api/cost", s.handleCost)
	mux.HandleFunc("/api/recommendations", s.handleRecommendations)
	mux.HandleFunc("/api/policies", s.handlePolicies)

	// Admission Webhook (Phase 6 Hardening)
	mux.Handle("/validate", s.governance)

	// Health Checks (Phase 9 optimization)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Readiness check - verify cluster connectivity
		if s.client == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("READY"))
	})

	// Internal Metrics (CNCF Observability)
	mux.Handle("/metrics", promhttp.Handler())

	// Static Files (Frontend)
	// We need to strip the "dist" prefix since the files are embedded in "dist/..."
	// Also use http.FS to adapt fs.FS to http.FileSystem
	distFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return fmt.Errorf("failed to load static files: %w", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", fileServer)

	// Wrap mux with RBAC Middleware (Phase 16 Hardening)
	// Pass the mock flag to dynamically bypass auth
	handler := AuthMiddleware(mux)

	addr := fmt.Sprintf(":%d", port)
	slog.Info("Dashboard starting", "url", fmt.Sprintf("http://localhost%s", addr))
	return http.ListenAndServe(addr, handler)
}

// Helper to write JSON response
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("JSON encoding error", "error", err)
	}
}

// runReconciler periodically fetches cluster data and updates the cache
func (s *Server) runReconciler(interval time.Duration) {
	slog.Info("Background reconciler started", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial sync
	s.reconcile()

	for range ticker.C {
		s.reconcile()
	}
}

func (s *Server) reconcile() {
	start := time.Now()
	ctx := context.Background()
	var metrics []types.PVCMetric
	var err error

	var pvcCollector collector.Collector
	if s.mock {
		pvcCollector = collector.NewMockPVCCollector()
	} else {
		pvcCollector = collector.NewPVCCollector(s.client, s.promClient)
	}
	metrics, err = pvcCollector.CollectAll(ctx)

	duration := time.Since(start)
	integrations.CollectionDuration.Observe(duration.Seconds())

	if err != nil {
		slog.Error("Reconciliation error", "error", err)
		integrations.CollectionTotal.WithLabelValues("error").Inc()
		return
	}

	integrations.CollectionTotal.WithLabelValues("success").Inc()
	integrations.PVCCount.Set(float64(len(metrics)))

	// Enrich with costs (unified cost engine)
	calculator := cost.NewCalculator()
	for i := range metrics {
		metrics[i].MonthlyCost = calculator.CalculatePVCCost(&metrics[i], s.provider)
	}

	summary := calculator.GenerateSummary(metrics, s.provider)

	optimizer := cost.NewOptimizer()
	recommendations := optimizer.GenerateRecommendations(metrics, s.provider)

	var policies []v1alpha1.StorageLifecyclePolicy
	// Fetch real policies if not in mock mode
	// Fetch real policies
	if s.client != nil {
		p, err := s.client.ListStoragePolicies(ctx)
		if err != nil {
			slog.Error("Failed to fetch storage policies", "error", err)
		} else {
			policies = p
			slog.Info("Policies fetched successfully", "count", len(policies))
		}
	}

	var costPolicies []v1alpha1.CostPolicy
	if s.client != nil {
		cp, err := s.client.ListCostPolicies(ctx)
		if err != nil {
			slog.Error("Failed to fetch cost policies", "error", err)
		} else {
			costPolicies = cp
			slog.Info("Cost policies fetched successfully", "count", len(costPolicies))
			s.governance.SetPolicies(costPolicies)
		}
	}

	// Initial Mock Policy for Phase 4 Demo
	// Update with real policies
	s.store.Policies = policies
	s.orchestrator.SetPolicies(policies)

	// Update cache
	s.store.Lock()
	s.store.Metrics = metrics
	s.store.Summary = *summary
	s.store.Recommendations = recommendations
	s.store.LastUpdate = time.Now()
	s.store.Unlock()
}

// GET /api/pvc
func (s *Server) handlePVCs(w http.ResponseWriter, r *http.Request) {
	s.store.RLock()
	metrics := s.store.Metrics
	s.store.RUnlock()

	if metrics == nil {
		metrics = []types.PVCMetric{}
	}
	writeJSON(w, metrics)
}

// GET /api/cost
func (s *Server) handleCost(w http.ResponseWriter, r *http.Request) {
	s.store.RLock()
	summary := s.store.Summary
	s.store.RUnlock()

	writeJSON(w, summary)
}

// GET /api/recommendations
func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	s.store.RLock()
	recommendations := s.store.Recommendations
	s.store.RUnlock()

	if recommendations == nil {
		recommendations = []types.Recommendation{}
	}
	writeJSON(w, recommendations)
}

// GET /api/policies
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	s.store.RLock()
	policies := s.store.Policies
	s.store.RUnlock()

	if policies == nil {
		policies = []v1alpha1.StorageLifecyclePolicy{}
	}
	writeJSON(w, policies)
}
