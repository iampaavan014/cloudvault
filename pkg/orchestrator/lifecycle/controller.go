package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

// LifecycleController manages autonomous storage tiering
type LifecycleController struct {
	engine      *PolicyEngine
	recommender *IntelligentRecommender
	manager     MigrationManager
	interval    time.Duration
}

// NewLifecycleController creates a new autonomous controller
func NewLifecycleController(interval time.Duration, manager MigrationManager, recommender *IntelligentRecommender) *LifecycleController {
	return &LifecycleController{
		manager:     manager,
		recommender: recommender,
		interval:    interval,
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

		// 1. Check for AI Intelligence (Right-sizing/Placement)
		recommendation := c.recommender.Recommend(pvc, policy)
		if recommendation != nil {
			slog.Info("🧠 AI RECOMMENDATION IDENTIFIED",
				"pvc", pvc.Name,
				"reason", recommendation.Reason,
				"target_size", recommendation.TargetSize)
			c.executeTransition(pvc, policy, recommendation)
			continue
		}

		// 2. Fallback to Rule-based Tiering
		targetTier, err := c.engine.Evaluate(pvc, policy)
		if err != nil {
			slog.Error("Policy evaluation failed", "pvc", pvc.Name, "policy", policy.Name, "error", err)
			continue
		}

		if targetTier != nil {
			c.executeTransition(pvc, policy, &OptimizationRecommendation{
				TargetClass: targetTier.StorageClass,
				TargetSize:  FormatQuantity(pvc.SizeBytes),
				TargetTier:  "warm",
				Reason:      fmt.Sprintf("Rule-based Tiering: Policy %s triggered duration threshold", policy.Name),
			})
		}
	}
}

func (c *LifecycleController) executeTransition(pvc types.PVCMetric, policy *v1alpha1.StorageLifecyclePolicy, rec *OptimizationRecommendation) {
	slog.Info("⚠️ AUTONOMOUS ACTION REQUIRED",
		"pvc", pvc.Name,
		"namespace", pvc.Namespace,
		"current_class", pvc.StorageClass,
		"target_class", rec.TargetClass,
		"target_size", rec.TargetSize,
		"reason", rec.Reason)

	if c.manager != nil {
		slog.Info("🚀 TRIGGERING INTELLIGENT MIGRATION",
			"pvc", pvc.Name,
			"target", rec.TargetClass,
			"size", rec.TargetSize)

		workflowName, err := c.manager.TriggerMigration(context.Background(), pvc, rec.TargetClass, rec.TargetSize)
		if err != nil {
			slog.Error("Failed to trigger migration", "pvc", pvc.Name, "error", err)
		} else {
			slog.Info("Migration workflow submitted", "pvc", pvc.Name, "workflow", workflowName)
		}
	}
}
