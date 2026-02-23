package ai

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	aiServiceURL     string
	aiServiceHealthy bool
	healthCheckMutex sync.RWMutex
	lastHealthCheck  time.Time
)

func init() {
	aiServiceURL = os.Getenv("CLOUDVAULT_AI_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:5005"
	}
	// Start background health checker
	go healthCheckLoop()
}

// getAIServiceURL returns the base URL for the recursive neural network inference service.
func getAIServiceURL() string {
	return aiServiceURL
}

// IsAIServiceHealthy returns whether the AI service is currently reachable
func IsAIServiceHealthy() bool {
	healthCheckMutex.RLock()
	defer healthCheckMutex.RUnlock()
	return aiServiceHealthy
}

// CheckAIServiceHealth performs a health check on the AI service
func CheckAIServiceHealth() error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(aiServiceURL + "/health")
	if err != nil {
		return fmt.Errorf("AI service unreachable: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AI service unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// healthCheckLoop runs periodic health checks on the AI service
func healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial check
	updateHealthStatus()

	for range ticker.C {
		updateHealthStatus()
	}
}

func updateHealthStatus() {
	err := CheckAIServiceHealth()

	healthCheckMutex.Lock()
	defer healthCheckMutex.Unlock()

	lastHealthCheck = time.Now()
	if err != nil {
		if aiServiceHealthy {
			slog.Warn("AI service became unhealthy", "error", err)
		}
		aiServiceHealthy = false
	} else {
		if !aiServiceHealthy {
			slog.Info("AI service is now healthy")
		}
		aiServiceHealthy = true
	}
}

// GetAIServiceStatus returns detailed status information
func GetAIServiceStatus() AIServiceStatus {
	healthCheckMutex.RLock()
	defer healthCheckMutex.RUnlock()

	return AIServiceStatus{
		URL:             aiServiceURL,
		Healthy:         aiServiceHealthy,
		LastHealthCheck: lastHealthCheck,
	}
}

// AIServiceStatus represents the current status of the AI service
type AIServiceStatus struct {
	URL             string
	Healthy         bool
	LastHealthCheck time.Time
}
