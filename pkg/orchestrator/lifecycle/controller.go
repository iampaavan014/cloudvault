package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

// LifecycleController manages autonomous storage tiering
type LifecycleController struct {
	engine   *PolicyEngine
	manager  MigrationManager
	interval time.Duration
}

// NewLifecycleController creates a new autonomous controller
func NewLifecycleController(interval time.Duration, manager MigrationManager) *LifecycleController {
	return &LifecycleController{
		manager:  manager,
		interval: interval,
	}
}

// Start starts the background orchestration loop
func (c *LifecycleController) Start(ctx context.Context, getMetrics func() []types.PVCMetric) {
	slog.Info("Autonomous Lifecycle Controller started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reconcile(getMetrics())
		}
	}
}

// SetPolicies updates the controller's policy set
func (c *LifecycleController) SetPolicies(policies []v1alpha1.StorageLifecyclePolicy) {
	c.engine = NewPolicyEngine(policies)
}

func (c *LifecycleController) reconcile(metrics []types.PVCMetric) {
	if c.engine == nil {
		return
	}

	slog.Debug("Reconciling storage lifecycle policies", "pvc_count", len(metrics))

	for _, pvc := range metrics {
		policy := c.engine.Match(pvc)
		if policy == nil {
			continue
		}

		targetTier, err := c.engine.Evaluate(pvc, policy)
		if err != nil {
			slog.Error("Policy evaluation failed", "pvc", pvc.Name, "policy", policy.Name, "error", err)
			continue
		}

		if targetTier != nil {
			c.executeTransition(pvc, policy, targetTier)
		}
	}
}

func (c *LifecycleController) executeTransition(pvc types.PVCMetric, policy *v1alpha1.StorageLifecyclePolicy, tier *v1alpha1.StorageTier) {
	slog.Info("⚠️ AUTONOMOUS ACTION REQUIRED",
		"pvc", pvc.Name,
		"namespace", pvc.Namespace,
		"current_class", pvc.StorageClass,
		"target_tier", tier.Name,
		"target_class", tier.StorageClass,
		"policy", policy.Name)

	// In Phase 4, we simulate the action or log it as a "Triggered" state.
	// Actual migration logic (MCE/ASO) will be integrated here in the final stage.

	if c.manager != nil {
		slog.Info("🚀 TRIGGERING REAL MIGRATION", "pvc", pvc.Name, "target", tier.StorageClass)
		workflowName, err := c.manager.TriggerMigration(context.Background(), pvc, tier.StorageClass)
		if err != nil {
			slog.Error("Failed to trigger migration", "pvc", pvc.Name, "error", err)
		} else {
			slog.Info("Migration workflow submitted", "pvc", pvc.Name, "workflow", workflowName)
		}
	}
}
