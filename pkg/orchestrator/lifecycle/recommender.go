package lifecycle

import (
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/ai"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

// IntelligentRecommender combines multiple AI models to provide optimization advice
type IntelligentRecommender struct {
	rlAgent       *ai.RLAgent
	forecaster    *ai.CostForecaster
	anomalyEngine *ai.AnomalyEngine
}

// OptimizationRecommendation contains suggested actions for a PVC
type OptimizationRecommendation struct {
	TargetClass string
	TargetTier  string
	TargetSize  string
	Reason      string
	Confidence  float64
}

func NewIntelligentRecommender() *IntelligentRecommender {
	return &IntelligentRecommender{
		rlAgent:       ai.NewRLAgent(),
		forecaster:    ai.NewCostForecaster(),
		anomalyEngine: ai.NewAnomalyEngine(0.05), // 5% contamination
	}
}

// Recommend finds the most impactful optimization for a PVC
func (r *IntelligentRecommender) Recommend(pvc types.PVCMetric, policy *v1alpha1.StorageLifecyclePolicy) *OptimizationRecommendation {
	// 1. Right-Sizing Analysis
	// If usage is consistently low (< 30%), recommend shrinking
	usageRatio := 0.0
	if pvc.SizeBytes > 0 {
		usageRatio = float64(pvc.UsedBytes) / float64(pvc.SizeBytes)
	}

	// 2. Intelligent Placement (RL)
	// Suggest the best class based on workload profile
	availableClasses := []string{"standard", "sc1", "gp3", "io2"}
	workloadType := r.detectWorkloadType(pvc)
	optimizedClass := r.rlAgent.DecidePlacement(workloadType, availableClasses)

	// 3. Construct Recommendation
	rec := &OptimizationRecommendation{
		TargetClass: optimizedClass,
		TargetSize:  FormatQuantity(pvc.SizeBytes),
		TargetTier:  "hot",
		Confidence:  0.85,
	}

	if usageRatio < 0.3 && pvc.SizeBytes > 10*1024*1024*1024 { // > 10GB and < 30% utilized
		suggestedSize := int64(float64(pvc.UsedBytes) * 1.5)

		// Ensure a minimum size floor of 1GB for valid provisioning
		minSize := int64(1024 * 1024 * 1024)
		if suggestedSize < minSize {
			suggestedSize = minSize
		}

		rec.TargetSize = FormatQuantity(suggestedSize)
		rec.Reason = "Right-sizing: Workload is over-provisioned (under 30% utilization)"
	} else if usageRatio < 0.05 {
		// Use Anomaly Engine to flag as Zombie
		if r.anomalyEngine.IsZombie([]float64{usageRatio}) {
			rec.Reason = "Optimization: Anomalous Zombie Volume identified (under 5% recurring usage)"
			rec.Confidence = 0.95
			rec.TargetTier = "cold"
		}
	} else if optimizedClass != pvc.StorageClass {
		rec.TargetTier = "warm" // RL usually suggests cost-optimized tiers
		rec.Reason = "Placement: Better performance/cost balance found via RL agent"
	} else {
		return nil // No impactful recommendation
	}

	return rec
}

func FormatQuantity(bytes int64) string {
	if bytes >= 1024*1024*1024 {
		return fmt.Sprintf("%dGi", bytes/(1024*1024*1024))
	}
	if bytes >= 1024*1024 {
		return fmt.Sprintf("%dMi", bytes/(1024*1024))
	}
	return fmt.Sprintf("%d", bytes)
}

func (r *IntelligentRecommender) detectWorkloadType(pvc types.PVCMetric) string {
	if pvc.EgressBytes > 1024*1024*100 { // > 100MB egress
		return "high-egress"
	}
	return "standard"
}
