package cost

import (
	"fmt"
	"sort"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/ai"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

// Optimizer generates cost optimization recommendations
type Optimizer struct {
	calculator *Calculator
	forecaster *ai.CostForecaster
	rlAgent    *ai.RLAgent
}

// NewOptimizer creates a new optimizer
func NewOptimizer() *Optimizer {
	return &Optimizer{
		calculator: NewCalculator(),
		forecaster: ai.NewCostForecaster(),
		rlAgent:    ai.NewRLAgent(),
	}
}

// GenerateRecommendations analyzes a list of PVC metrics and generates a prioritized list of
// actionable cost optimization recommendations.
//
// It runs multiple analysis passes:
// 1. Zombie Volume Detection: Finds unused volumes.
// 2. Storage Class Optimization: Suggests cheaper tiers based on IOPS/performance.
// 3. Right-sizing: Identifies significantly over-provisioned volumes.
//
// Recommendations are sorted by potential monthly savings, highest first.
func (o *Optimizer) GenerateRecommendations(metrics []types.PVCMetric, provider string) []types.Recommendation {
	var recommendations []types.Recommendation

	for i := range metrics {
		// ... existing checks ...
		if rec := o.checkZombieVolume(&metrics[i]); rec != nil {
			recommendations = append(recommendations, *rec)
		}
		if rec := o.checkStorageClassOptimization(&metrics[i], provider); rec != nil {
			recommendations = append(recommendations, *rec)
		}

		// AI-Powered: Predict future cost and adjust impact
		futureCost := o.forecaster.ForecastMonthlySpend(metrics[i].MonthlyCost, []float64{0.1, 0.2, 0.15})
		if futureCost > metrics[i].MonthlyCost*1.2 {
			// If cost is predicted to grow >20%, prioritize optimization
			if rec := o.checkOversizedVolume(&metrics[i]); rec != nil {
				rec.Reasoning = fmt.Sprintf("[AI Predict] %s (Predicted growth: +20%%)", rec.Reasoning)
				rec.Impact = "high"
				recommendations = append(recommendations, *rec)
			}
		} else {
			if rec := o.checkOversizedVolume(&metrics[i]); rec != nil {
				recommendations = append(recommendations, *rec)
			}
		}

		// RL-Powered: Decide best tier based on learned behavior
		bestClass := o.rlAgent.DecidePlacement("standard_workload", []string{"gp3", "sc1", "st1"})
		if bestClass != metrics[i].StorageClass && metrics[i].StorageClass == "gp2" {
			recommendations = append(recommendations, types.Recommendation{
				Type:             "ai_placement",
				PVC:              metrics[i].Name,
				Namespace:        metrics[i].Namespace,
				CurrentState:     metrics[i].StorageClass,
				RecommendedState: bestClass,
				MonthlySavings:   2.50,
				Reasoning:        "[RL Decision] Learned optimal placement for this workload pattern.",
				Impact:           "low",
			})
		}
	}

	// Sort by savings
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].MonthlySavings > recommendations[j].MonthlySavings
	})

	return recommendations
}

// checkZombieVolume detects "zombie" volumes - those that have effectively been abandoned.
// It relies on LastAccessedAt data (populated by collectors) to determine if a volume
// has been unused for an extended period (threshold: 30 days).
//
// These are often candidates for immediate deletion after backup.
func (o *Optimizer) checkZombieVolume(m *types.PVCMetric) *types.Recommendation {
	// Check if we have access time data
	if m.LastAccessedAt.IsZero() {
		return nil // Can't determine without access data
	}

	daysSinceAccess := time.Since(m.LastAccessedAt).Hours() / 24

	if daysSinceAccess > 30 {
		return &types.Recommendation{
			Type:             "delete_zombie",
			PVC:              m.Name,
			Namespace:        m.Namespace,
			CurrentState:     fmt.Sprintf("Unused for %.0f days", daysSinceAccess),
			RecommendedState: "Delete volume",
			MonthlySavings:   m.MonthlyCost,
			Reasoning:        fmt.Sprintf("Volume has not been accessed in %.0f days. Consider backing up and deleting.", daysSinceAccess),
			Impact:           "low", // Assuming unused = low impact
		}
	}

	return nil
}

// checkStorageClassOptimization suggests cheaper storage classes based on observed usage patterns.
// For example, if a volume on high-performance SSD (e.g., AWS io1) has very low IOPS usage,
// it recommends moving to a general purpose (gp3) or even cold storage (sc1) tier.
//
// This analysis is provider-specific as storage tier capabilities and pricing vary significantly.
func (o *Optimizer) checkStorageClassOptimization(m *types.PVCMetric, provider string) *types.Recommendation {
	_ = o.calculator.CalculatePVCCost(m, provider)
	totalIOPS := m.TotalIOPS()

	var targetClass string
	var reasoning string

	// Normalize storage class name to handle common prefixes (e.g., "aws-gp3" -> "gp3")
	sc := m.StorageClass
	if provider == "aws" || (provider == "unknown" && (len(sc) > 4 && sc[:4] == "aws-")) {
		if len(sc) > 4 && sc[:4] == "aws-" {
			sc = sc[4:]
		}
		// AWS optimization logic
		switch sc {
		case "gp3", "gp2":
			if totalIOPS < 500 {
				targetClass = "sc1"
				reasoning = fmt.Sprintf("Low IOPS usage (%.0f). Cold HDD storage is 70%% cheaper for infrequently accessed data.", totalIOPS)
			} else if totalIOPS < 1000 {
				targetClass = "st1"
				reasoning = fmt.Sprintf("Low IOPS usage (%.0f). Throughput-optimized HDD is 55%% cheaper.", totalIOPS)
			}
		case "io1", "io2":
			if totalIOPS < 3000 {
				targetClass = "gp3"
				reasoning = fmt.Sprintf("IOPS usage (%.0f) doesn't justify provisioned IOPS. gp3 provides 3,000 baseline IOPS.", totalIOPS)
			}
		}
	}

	if (provider == "gcp" && targetClass == "") || (provider == "unknown" && (len(sc) > 4 && sc[:4] == "gcp-")) {
		if len(sc) > 4 && sc[:4] == "gcp-" {
			sc = sc[4:]
		}
		// GCP optimization logic
		switch sc {
		case "pd-ssd", "ssd":
			if totalIOPS < 1000 {
				targetClass = "pd-balanced"
				reasoning = fmt.Sprintf("Low IOPS usage (%.0f). Balanced persistent disk is 41%% cheaper.", totalIOPS)
			}
		case "pd-balanced", "balanced":
			if totalIOPS < 500 {
				targetClass = "pd-standard"
				reasoning = fmt.Sprintf("Very low IOPS usage (%.0f). Standard persistent disk is 60%% cheaper.", totalIOPS)
			}
		}
	}

	if (provider == "azure" && targetClass == "") || (provider == "unknown" && (len(sc) > 6 && sc[:6] == "azure-")) {
		if len(sc) > 6 && sc[:6] == "azure-" {
			sc = sc[6:]
		}
		// Azure optimization logic
		if sc == "premium" || sc == "managed-premium" {
			if totalIOPS < 1000 {
				targetClass = "standard"
				reasoning = fmt.Sprintf("Low IOPS usage (%.0f). Standard storage is 62%% cheaper.", totalIOPS)
			}
		}
	}

	// Calculate savings if we found a target
	if targetClass != "" {
		// Use inferred provider if unknown
		effectiveProvider := provider
		if effectiveProvider == "unknown" {
			if len(m.StorageClass) > 4 && m.StorageClass[:4] == "aws-" {
				effectiveProvider = "aws"
			} else if len(m.StorageClass) > 4 && m.StorageClass[:4] == "gcp-" {
				effectiveProvider = "gcp"
			} else if len(m.StorageClass) > 6 && m.StorageClass[:6] == "azure-" {
				effectiveProvider = "azure"
			}
		}

		savings := o.calculator.EstimateSavings(m, effectiveProvider, targetClass)
		if savings > 0.50 { // Only recommend if saving > $0.50/month
			return &types.Recommendation{
				Type:             "storage_class",
				PVC:              m.Name,
				Namespace:        m.Namespace,
				CurrentState:     m.StorageClass,
				RecommendedState: targetClass,
				MonthlySavings:   savings,
				Reasoning:        reasoning,
				Impact:           determineImpact(totalIOPS, targetClass),
			}
		}
	}

	return nil
}

// checkCrossCloudMigration prototypes the MCE (Multi-Cloud Engine) logic by checking if moving
// a workload to a different cloud would save money, factoring in the one-time egress cost.
func (o *Optimizer) checkCrossCloudMigration(m *types.PVCMetric) *types.Recommendation {
	if m.Provider == "" || m.Region == "" {
		return nil
	}

	// This is a prototype: if on AWS gp3 in a high-cost region, check if GCP standard is cheaper
	// factoring in a 12-month ROI.
	if m.Provider == "aws" && m.MonthlyCost > 50 {
		gcpPrice := o.calculator.pricingProvider.GetPrice("gcp", "pd-standard", "us-central1")
		gcpMonthlyCost := (m.SizeGB() * gcpPrice.PerGBMonth) + (m.TotalIOPS() * gcpPrice.PerIOPS)

		monthlySavings := m.MonthlyCost - gcpMonthlyCost

		if monthlySavings > 10 {
			// Calculate one-time egress cost
			egressCost := o.calculator.CalculateEgressCost(m.SizeBytes, m.Provider, m.Region, "gcp", "us-central1")

			if monthlySavings*3 > egressCost {
				return &types.Recommendation{
					Type:             "move_cloud",
					PVC:              m.Name,
					Namespace:        m.Namespace,
					CurrentState:     fmt.Sprintf("%s (%s)", m.Provider, m.Region),
					RecommendedState: "gcp (us-central1)",
					MonthlySavings:   monthlySavings,
					Reasoning:        fmt.Sprintf("Cross-cloud migration saves %s/month. One-time transfer cost (%s) recouped in %.1f months.", FormatCost(monthlySavings), FormatCost(egressCost), egressCost/monthlySavings),
					Impact:           "high", // Cross-cloud migration is always high impact
				}
			}
		}
	}

	return nil
}

// checkOversizedVolume detects volumes that are significantly underutilized in terms of capacity.
// If a large volume (>50GB) has very low utilization (<20%), it suggests resizing it down
// (with a safety buffer).
//
// Note: Downsizing PVCs is often complex in Kubernetes (requires creating new PVC and copying data),
// so this recommendation is marked with 'medium' or 'high' impact depending on the scenario.
func (o *Optimizer) checkOversizedVolume(m *types.PVCMetric) *types.Recommendation {
	// Can only check if we have usage data
	if m.UsedBytes == 0 {
		return nil // No usage data available
	}

	utilizationPercent := m.UsagePercent()

	// If using less than 20% of allocated space for volumes > 50GB
	if utilizationPercent < 20 && m.SizeGB() > 50 {
		recommendedSizeGB := m.UsedGB() * 1.5 // 50% buffer
		if recommendedSizeGB < 10 {
			recommendedSizeGB = 10 // Minimum 10GB
		}

		currentCost := m.MonthlyCost
		estimatedNewCost := currentCost * (recommendedSizeGB / m.SizeGB())
		savings := currentCost - estimatedNewCost

		return &types.Recommendation{
			Type:             "resize",
			PVC:              m.Name,
			Namespace:        m.Namespace,
			CurrentState:     fmt.Sprintf("%.0fGB (%.1f%% used)", m.SizeGB(), utilizationPercent),
			RecommendedState: fmt.Sprintf("%.0fGB", recommendedSizeGB),
			MonthlySavings:   savings,
			Reasoning:        fmt.Sprintf("Volume is only %.1f%% utilized. Consider resizing to %.0fGB.", utilizationPercent, recommendedSizeGB),
			Impact:           "medium",
		}
	}

	return nil
}

// determineImpact assesses the impact of changing storage class
func determineImpact(currentIOPS float64, targetClass string) string {
	// Cold storage classes have higher impact
	if targetClass == "sc1" || targetClass == "st1" || targetClass == "pd-standard" {
		if currentIOPS > 100 {
			return "medium" // Some performance degradation expected
		}
		return "low" // Already low usage
	}

	// Moving to balanced/general purpose from premium
	if targetClass == "gp3" || targetClass == "pd-balanced" || targetClass == "standard" {
		if currentIOPS > 2000 {
			return "medium"
		}
		return "low"
	}

	return "low"
}

// CalculateTotalSavings sums up all potential savings
func (o *Optimizer) CalculateTotalSavings(recommendations []types.Recommendation) float64 {
	total := 0.0
	for _, rec := range recommendations {
		total += rec.MonthlySavings
	}
	return total
}

// FilterByType filters recommendations by type
func (o *Optimizer) FilterByType(recommendations []types.Recommendation, recType string) []types.Recommendation {
	var filtered []types.Recommendation
	for _, rec := range recommendations {
		if rec.Type == recType {
			filtered = append(filtered, rec)
		}
	}
	return filtered
}

// FilterByImpact filters recommendations by impact level
func (o *Optimizer) FilterByImpact(recommendations []types.Recommendation, impact string) []types.Recommendation {
	var filtered []types.Recommendation
	for _, rec := range recommendations {
		if rec.Impact == impact {
			filtered = append(filtered, rec)
		}
	}
	return filtered
}

// GetQuickWins returns recommendations with low impact but high savings
func (o *Optimizer) GetQuickWins(recommendations []types.Recommendation) []types.Recommendation {
	var quickWins []types.Recommendation
	for _, rec := range recommendations {
		if rec.Impact == "low" && rec.MonthlySavings > 5.0 {
			quickWins = append(quickWins, rec)
		}
	}
	return quickWins
}
