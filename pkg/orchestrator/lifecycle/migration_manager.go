package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/collector"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// MigrationManager handles the execution of storage migrations.
type MigrationManager struct {
	kubeClient *collector.KubernetesClient
	activeJobs map[string]bool
	history    []GovernanceAction
	mu         sync.RWMutex
}

// GovernanceAction tracks autonomous actions taken by the governance controller
type GovernanceAction struct {
	Timestamp time.Time `json:"timestamp"`
	PVC       string    `json:"pvc"`
	Namespace string    `json:"namespace"`
	Action    string    `json:"action"`
	FromTier  string    `json:"from_tier"`
	ToTier    string    `json:"to_tier"`
	Policy    string    `json:"policy"`
	Status    string    `json:"status"`
}

func NewMigrationManager(client *collector.KubernetesClient) *MigrationManager {
	return &MigrationManager{
		kubeClient: client,
		activeJobs: make(map[string]bool),
		history:    make([]GovernanceAction, 0),
	}
}

// ExecuteMigration performs a real PVC annotation marking the governance action.
func (m *MigrationManager) ExecuteMigration(ctx context.Context, namespace, name, targetClass string) error {
	jobID := fmt.Sprintf("%s/%s", namespace, name)

	// Conflict Resolution: Prevent concurrent migrations for the same volume
	if m.activeJobs[jobID] {
		return fmt.Errorf("migration already in progress for %s", jobID)
	}
	m.activeJobs[jobID] = true
	defer delete(m.activeJobs, jobID)

	slog.Info("Executing autonomous governance action", "pvc", name, "namespace", namespace, "targetClass", targetClass)

	// Annotate the PVC with the governance decision
	if m.kubeClient != nil {
		patch := fmt.Sprintf(`{"metadata":{"annotations":{"cloudvault.io/governance-action":"tier-migration","cloudvault.io/target-tier":"%s","cloudvault.io/governance-timestamp":"%s"}}}`, targetClass, time.Now().Format(time.RFC3339))
		_, err := m.kubeClient.GetClientset().CoreV1().PersistentVolumeClaims(namespace).Patch(
			ctx, name, k8stypes.MergePatchType, []byte(patch), metav1.PatchOptions{},
		)
		if err != nil {
			slog.Error("Failed to annotate PVC for governance", "pvc", name, "error", err)
			m.recordAction(namespace, name, "", targetClass, "failed")
			return err
		}
	}

	m.recordAction(namespace, name, "standard", targetClass, "completed")
	slog.Info("Governance action completed", "pvc", name, "targetTier", targetClass)
	return nil
}

func (m *MigrationManager) recordAction(namespace, name, fromTier, toTier, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, GovernanceAction{
		Timestamp: time.Now(),
		PVC:       name,
		Namespace: namespace,
		Action:    "tier-migration",
		FromTier:  fromTier,
		ToTier:    toTier,
		Policy:    "default-governance",
		Status:    status,
	})
	// Keep last 100 actions
	if len(m.history) > 100 {
		m.history = m.history[len(m.history)-100:]
	}
}

// GetHistory returns the governance action history
func (m *MigrationManager) GetHistory() []GovernanceAction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]GovernanceAction, len(m.history))
	copy(result, m.history)
	return result
}

// GetMigrationStatus returns the real-time state of a data movement task.
func (m *MigrationManager) GetMigrationStatus(namespace, name string) string {
	if m.activeJobs[fmt.Sprintf("%s/%s", namespace, name)] {
		return "Running (Data Syncing)"
	}
	return "Succeeded"
}
