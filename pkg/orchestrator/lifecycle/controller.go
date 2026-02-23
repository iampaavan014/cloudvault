package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/collector"
	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/graph"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

// LifecycleController manages autonomous storage optimization cycles.
type LifecycleController struct {
	client       *collector.KubernetesClient
	pvcCollector *collector.PVCCollector
	manager      *MigrationManager
	recommender  *IntelligentRecommender
	sig          *graph.SIG
	timescale    *graph.TimescaleDB
	optimizer    *cost.Optimizer
	interval     time.Duration
	policies     []v1alpha1.StorageLifecyclePolicy
}

// NewLifecycleController creates a new autonomous controller with full SIG integration.
func NewLifecycleController(
	interval time.Duration,
	client *collector.KubernetesClient,
	recommender *IntelligentRecommender,
	sig *graph.SIG,
	timescale *graph.TimescaleDB,
) *LifecycleController {
	mgr := NewMigrationManager(client)
	return &LifecycleController{
		client:      client,
		manager:     mgr,
		recommender: recommender,
		sig:         sig,
		timescale:   timescale,
		optimizer:   cost.NewOptimizer(),
		interval:    interval,
	}
}

// SetPolicies updates the controller's policy set.
func (c *LifecycleController) SetPolicies(policies []v1alpha1.StorageLifecyclePolicy) {
	c.policies = policies
	slog.Info("Updated lifecycle policies", "count", len(policies))
}

// SetPVCCollector sets the PVC collector for metrics gathering
func (c *LifecycleController) SetPVCCollector(collector *collector.PVCCollector) {
	c.pvcCollector = collector
}

// Start begins the infinite loop of storage intelligence and action.
func (c *LifecycleController) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	slog.Info("Starting autonomous lifecycle controller with SIG integration", "interval", c.interval)

	// Run first optimization immediately
	if err := c.processOptimization(ctx); err != nil {
		slog.Error("Initial optimization pass failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Lifecycle controller shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := c.processOptimization(ctx); err != nil {
				slog.Error("Optimization pass failed", "error", err)
			}
		}
	}
}

func (c *LifecycleController) processOptimization(ctx context.Context) error {
	slog.Info("Starting optimization pass...")

	// 1. Collect current PVC metrics
	if c.pvcCollector == nil {
		slog.Warn("PVC collector not set, skipping optimization pass")
		return nil
	}

	metrics, err := c.pvcCollector.CollectAll(ctx)
	if err != nil {
		return err
	}
	slog.Info("Collected PVC metrics", "count", len(metrics))

	// 2. Sync metrics to Storage Intelligence Graph (SIG)
	if c.sig != nil {
		if err := c.sig.SyncPVCs(ctx, metrics); err != nil {
			slog.Error("Failed to sync PVCs to SIG", "error", err)
		} else {
			slog.Info("Synced PVCs to Storage Intelligence Graph")
		}

		// 3. Map Pod-to-PVC relationships for data gravity analysis
		if err := c.syncPodRelationships(ctx, metrics); err != nil {
			slog.Error("Failed to sync pod relationships", "error", err)
		}

		// 4. Detect cross-region gravity issues
		crossRegionPVCs, err := c.sig.GetCrossRegionGravity(ctx)
		if err != nil {
			slog.Error("Failed to detect cross-region gravity", "error", err)
		} else if len(crossRegionPVCs) > 0 {
			slog.Warn("Detected cross-region data gravity issues", "count", len(crossRegionPVCs), "pvcs", crossRegionPVCs)
		}
	}

	// 5. Record metrics to TimescaleDB for historical analysis
	if c.timescale != nil {
		if err := c.timescale.RecordMetrics(ctx, metrics); err != nil {
			slog.Error("Failed to record metrics to TimescaleDB", "error", err)
		} else {
			slog.Debug("Recorded metrics to TimescaleDB for AI training")
		}
	}

	// 6. Generate cost optimization recommendations
	recommendations := c.optimizer.GenerateRecommendations(metrics, "aws")
	slog.Info("Generated recommendations", "count", len(recommendations))

	if len(recommendations) > 0 {
		totalSavings := c.optimizer.CalculateTotalSavings(recommendations)
		slog.Info("Total potential savings", "monthly", totalSavings)
	}

	// 7. Evaluate lifecycle policies
	policyEngine := NewPolicyEngine(c.policies)

	migrationCount := 0
	for i := range metrics {
		policy := policyEngine.Match(metrics[i])
		if policy != nil {
			targetTier, err := policyEngine.Evaluate(metrics[i], policy)
			if err != nil {
				slog.Error("Failed to evaluate policy", "pvc", metrics[i].Name, "error", err)
				continue
			}

			if targetTier != nil && targetTier.StorageClass != metrics[i].StorageClass {
				slog.Info("PVC eligible for lifecycle migration",
					"pvc", metrics[i].Name,
					"namespace", metrics[i].Namespace,
					"current", metrics[i].StorageClass,
					"target", targetTier.StorageClass,
					"policy", policy.Name,
				)

				// Execute migration (in future, check if auto-execute is enabled)
				if c.manager != nil {
					if err := c.manager.ExecuteMigration(ctx, metrics[i].Namespace, metrics[i].Name, targetTier.StorageClass); err != nil {
						slog.Error("Migration failed", "pvc", metrics[i].Name, "error", err)
					} else {
						migrationCount++
					}
				}
			}
		}
	}

	if migrationCount > 0 {
		slog.Info("Completed optimization pass", "migrations_executed", migrationCount)
	} else {
		slog.Info("Completed optimization pass - no migrations needed")
	}

	return nil
}

// syncPodRelationships creates Pod-to-PVC relationships in the SIG for data gravity analysis
func (c *LifecycleController) syncPodRelationships(ctx context.Context, metrics []types.PVCMetric) error {
	if c.sig == nil {
		return nil
	}

	relationshipCount := 0
	for _, metric := range metrics {
		for _, podName := range metric.MountedPods {
			if err := c.sig.MapPodToPVC(ctx, podName, metric.Namespace, metric.Name); err != nil {
				slog.Error("Failed to map pod to PVC", "pod", podName, "pvc", metric.Name, "error", err)
			} else {
				relationshipCount++
			}
		}
	}

	if relationshipCount > 0 {
		slog.Info("Synced pod-to-PVC relationships to SIG", "count", relationshipCount)
	}

	return nil
}

// GetStatus returns the current status of the lifecycle controller
func (c *LifecycleController) GetStatus() LifecycleStatus {
	return LifecycleStatus{
		ActivePolicies:   len(c.policies),
		SIGEnabled:       c.sig != nil,
		TimescaleEnabled: c.timescale != nil,
	}
}

// LifecycleStatus represents the current state of the controller
type LifecycleStatus struct {
	ActivePolicies   int
	SIGEnabled       bool
	TimescaleEnabled bool
}
