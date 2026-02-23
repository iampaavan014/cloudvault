package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/ai"
)

// HealthResponse represents the health status of the system
type HealthResponse struct {
	Status     string                     `json:"status"`
	Version    string                     `json:"version"`
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents the health of an individual component
type ComponentHealth struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	LastCheck time.Time `json:"last_check"`
}

// HandleHealth returns the overall health status
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	components := make(map[string]ComponentHealth)

	// Check Kubernetes connectivity
	if s.client != nil {
		_, err := s.client.GetClusterInfo(ctx)
		if err != nil {
			components["kubernetes"] = ComponentHealth{
				Status:    "unhealthy",
				Message:   err.Error(),
				LastCheck: time.Now(),
			}
		} else {
			components["kubernetes"] = ComponentHealth{
				Status:    "healthy",
				LastCheck: time.Now(),
			}
		}
	}

	// Check Prometheus connectivity
	if s.promClient != nil {
		_, err := s.promClient.GetAllPVCMetrics(ctx)
		if err != nil {
			components["prometheus"] = ComponentHealth{
				Status:    "unhealthy",
				Message:   "Prometheus query failed",
				LastCheck: time.Now(),
			}
		} else {
			components["prometheus"] = ComponentHealth{
				Status:    "healthy",
				LastCheck: time.Now(),
			}
		}
	}

	// Check Agent self-health
	components["agent"] = ComponentHealth{
		Status:    "healthy",
		LastCheck: time.Now(),
	}

	// Check AI service health
	aiStatus := ai.GetAIServiceStatus()
	if aiStatus.Healthy {
		components["ai_service"] = ComponentHealth{
			Status:    "healthy",
			LastCheck: aiStatus.LastHealthCheck,
		}
	} else {
		components["ai_service"] = ComponentHealth{
			Status:    "unhealthy",
			Message:   "AI service unreachable",
			LastCheck: aiStatus.LastHealthCheck,
		}
	}

	// Determine overall status
	overallStatus := "healthy"

	// Ensure all standard components exist at least as unknown/checking
	standardComponents := []string{"agent", "kubernetes", "prometheus", "ai_service"}
	for _, c := range standardComponents {
		if _, exists := components[c]; !exists {
			components[c] = ComponentHealth{
				Status:    "unknown",
				Message:   "Component not initialized",
				LastCheck: time.Now(),
			}
		}
	}

	for _, comp := range components {
		if comp.Status == "unhealthy" {
			overallStatus = "degraded"
			break
		}
	}

	response := HealthResponse{
		Status:     overallStatus,
		Version:    "v1.0.0",
		Timestamp:  time.Now(),
		Components: components,
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(response)
}

// HandleReadiness returns the readiness status (for Kubernetes probes)
func (s *Server) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Check if we can query Kubernetes
	if s.client != nil {
		if _, err := s.client.GetClusterInfo(ctx); err != nil {
			http.Error(w, "Not ready: Kubernetes unreachable", http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "Ready")
}

// HandleLiveness returns the liveness status (for Kubernetes probes)
func (s *Server) HandleLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "Alive")
}

// RegisterHealthEndpoints registers health check endpoints
func (s *Server) RegisterHealthEndpoints() {
	http.HandleFunc("/health", s.HandleHealth)
	http.HandleFunc("/healthz", s.HandleLiveness)
	http.HandleFunc("/readyz", s.HandleReadiness)
	slog.Info("Registered health check endpoints", "endpoints", []string{"/health", "/healthz", "/readyz"})
}
