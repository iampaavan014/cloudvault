package cost

import (
	"fmt"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

// Calculator handles cost calculations for different cloud providers
type Calculator struct {
	pricingProvider PricingProvider
}

// StorageClassPricing represents pricing information for a storage class
type StorageClassPricing struct {
	PerGBMonth  float64 // Price per GB per month
	PerIOPS     float64 // Price per IOPS (if provisioned)
	Provisioned bool    // Whether IOPS are provisioned
}

// NewCalculator creates a new cost calculator with default static pricing.
func NewCalculator() *Calculator {
	return &Calculator{
		pricingProvider: NewStaticPricingProvider(),
	}
}

// NewCalculatorWithProvider creates a new cost calculator with a specific pricing provider.
// Useful for injection of real-time API providers or mocks.
func NewCalculatorWithProvider(provider PricingProvider) *Calculator {
	return &Calculator{
		pricingProvider: provider,
	}
}

// CalculatePVCCost calculates the estimated monthly cost for a PVC based on its size,
// storage class, and provider. It handles both straightforward per-GB pricing and
// complex provisioned IOPS pricing models (like AWS io1/io2).
//
// If the specific storage class is not found in the pricing data, it attempts to fallback
// to a generic default for the provider, and finally to a global default if necessary.
func (c *Calculator) CalculatePVCCost(metric *types.PVCMetric, provider string) float64 {
	// Use the provider to get pricing. We pass "us-east-1" as default region for now
	// until region support is fully plumbed through from the CLI/Agent.
	pricing := c.pricingProvider.GetPrice(provider, metric.StorageClass, "us-east-1")

	sizeGB := float64(metric.SizeBytes) / (1024 * 1024 * 1024)
	storageCost := sizeGB * pricing.PerGBMonth

	// Add IOPS cost if provisioned
	iopsCost := 0.0
	if pricing.Provisioned && metric.TotalIOPS() > 3000 {
		extraIOPS := metric.TotalIOPS() - 3000
		iopsCost = extraIOPS * pricing.PerIOPS
	}

	cost := storageCost + iopsCost
	metric.MonthlyCost = cost
	return cost
}

// GenerateSummary creates a comprehensive cost summary for a list of PVC metrics.
// It aggregates costs by namespace and storage class, identifies top expensive volumes,
// and flags potential zombie volumes for review.
//
// The returned CostSummary provides a high-level view of storage spend, suitable for
// reporting and dashboarding.
func (c *Calculator) GenerateSummary(metrics []types.PVCMetric, provider string) *types.CostSummary {
	summary := &types.CostSummary{
		ByNamespace:    make(map[string]float64),
		ByStorageClass: make(map[string]float64),
		ByProvider:     make(map[string]float64),
		ByCluster:      make(map[string]float64),
		ZombieVolumes:  make([]types.PVCMetric, 0),
		ActiveAlerts:   []string{},
	}

	// Calculate costs and aggregate
	for i := range metrics {
		cost := c.CalculatePVCCost(&metrics[i], provider)
		metrics[i].MonthlyCost = cost
		metrics[i].HourlyCost = cost / (24 * 30)

		summary.TotalMonthlyCost += cost
		summary.ByNamespace[metrics[i].Namespace] += cost
		summary.ByStorageClass[metrics[i].StorageClass] += cost

		// Aggregate by provider and cluster (Phase 10 optimization)
		p := metrics[i].Provider
		if p == "" {
			p = provider
		}
		summary.ByProvider[p] += cost

		cid := metrics[i].ClusterID
		if cid == "" {
			cid = "default-cluster"
		}
		summary.ByCluster[cid] += cost

		// Check if zombie
		if metrics[i].IsZombie() {
			summary.ZombieVolumes = append(summary.ZombieVolumes, metrics[i])
		}
	}

	// Governance check (Budget)
	// In production, this is handled by CostPolicy controllers and the Admission Webhook

	// Sort and get top expensive (simple approach - just take first 10)
	// TODO: Implement proper sorting in Phase 2
	topCount := 10
	if len(metrics) < topCount {
		topCount = len(metrics)
	}

	// Find most expensive by iterating
	for i := 0; i < topCount && i < len(metrics); i++ {
		maxIdx := i
		for j := i + 1; j < len(metrics); j++ {
			if metrics[j].MonthlyCost > metrics[maxIdx].MonthlyCost {
				maxIdx = j
			}
		}
		// Swap
		metrics[i], metrics[maxIdx] = metrics[maxIdx], metrics[i]
		summary.TopExpensive = append(summary.TopExpensive, metrics[i])
	}

	return summary
}

// GetPricing returns pricing info for a storage class
func (c *Calculator) GetPricing(provider, storageClass string) *StorageClassPricing {
	// We assume us-east-1 for now
	pricing := c.pricingProvider.GetPrice(provider, storageClass, "us-east-1")
	return &pricing
}

// EstimateSavings calculates the potential monthly savings if a PVC were migrated
// to a different storage class. It compares the current cost against the estimated
// cost of the target class, assuming the same size and IOPS requirements.
func (c *Calculator) EstimateSavings(metric *types.PVCMetric, provider, newStorageClass string) float64 {
	currentCost := c.CalculatePVCCost(metric, provider)

	// Create a copy with new storage class
	testMetric := *metric
	testMetric.StorageClass = newStorageClass
	newCost := c.CalculatePVCCost(&testMetric, provider)

	return currentCost - newCost
}

// CalculateEgressCost estimates the cost of moving a given amount of data between two regions/clouds.
// It uses a simplified model based on standard cloud egress fees (e.g., $0.09/GB for external egress).
func (c *Calculator) CalculateEgressCost(bytes int64, srcCloud, srcRegion, dstCloud, dstRegion string) float64 {
	gb := float64(bytes) / (1024 * 1024 * 1024)

	// Same region, same cloud -> Free
	if srcCloud == dstCloud && srcRegion == dstRegion {
		return 0
	}

	// Same cloud, different region (inter-region fee)
	if srcCloud == dstCloud {
		return gb * 0.02 // Approx $0.02/GB
	}

	// Different clouds (external egress fee)
	// AWS/GCP typically charge ~$0.09/GB for internet egress
	return gb * 0.09
}

// FormatCost formats a cost value as a string
func FormatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

// FormatCostPerMonth formats a monthly cost
func FormatCostPerMonth(cost float64) string {
	return fmt.Sprintf("$%.2f/mo", cost)
}

// FormatCostPerYear formats an annual cost
func FormatCostPerYear(cost float64) string {
	return fmt.Sprintf("$%.2f/yr", cost*12)
}
