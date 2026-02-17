package cost

import "fmt"

// PricingProvider defines the interface for retrieving cloud storage pricing.
// It abstracts the source of pricing data (static map, API, etc.).
type PricingProvider interface {
	// GetPrice returns the pricing for a given provider and storage class.
	// Returns a default pricing if the specific class is not found.
	GetPrice(provider, storageClass, region string) StorageClassPricing
}

// StaticPricingProvider implements PricingProvider using a hardcoded map.
// This serves as the baseline/fallback pricing source.
type StaticPricingProvider struct {
	pricingData map[string]StorageClassPricing
}

// NewStaticPricingProvider creates a new StaticPricingProvider with default data.
func NewStaticPricingProvider() *StaticPricingProvider {
	return &StaticPricingProvider{
		pricingData: initializePricing(),
	}
}

// GetPrice returns the pricing for a given provider and storage class from the static map.
func (p *StaticPricingProvider) GetPrice(provider, storageClass, region string) StorageClassPricing {
	// Key format: "provider-storageclass"
	// Region is currently ignored in the static map simplicity, but interface supports it for future API use.
	key := fmt.Sprintf("%s-%s", provider, storageClass)

	pricing, ok := p.pricingData[key]
	if !ok {
		// Try with generic storage class name
		key = fmt.Sprintf("%s-default", provider)
		pricing, ok = p.pricingData[key]
		if !ok {
			// Fall back to unknown provider default
			pricing = p.pricingData["unknown-default"]
		}
	}
	return pricing
}

// This function is moved from calculator.go and remains the data source for StaticPricingProvider
func initializePricing() map[string]StorageClassPricing {
	// Key format: "provider-storageclass"
	// Prices are approximate as of Feb 2026 and vary by region
	return map[string]StorageClassPricing{
		// AWS EBS pricing (us-east-1)
		"aws-gp3": {
			PerGBMonth:  0.08,
			PerIOPS:     0.005, // Above 3000 baseline IOPS
			Provisioned: true,
		},
		"aws-gp2": {
			PerGBMonth:  0.10,
			PerIOPS:     0,
			Provisioned: false,
		},
		"aws-io1": {
			PerGBMonth:  0.125,
			PerIOPS:     0.065,
			Provisioned: true,
		},
		"aws-io2": {
			PerGBMonth:  0.125,
			PerIOPS:     0.065,
			Provisioned: true,
		},
		"aws-st1": {
			PerGBMonth:  0.045, // Throughput-optimized HDD
			PerIOPS:     0,
			Provisioned: false,
		},
		"aws-sc1": {
			PerGBMonth:  0.025, // Cold HDD
			PerIOPS:     0,
			Provisioned: false,
		},

		// GCP Persistent Disk pricing
		"gcp-standard": {
			PerGBMonth:  0.04,
			PerIOPS:     0,
			Provisioned: false,
		},
		"gcp-pd-standard": {
			PerGBMonth:  0.04,
			PerIOPS:     0,
			Provisioned: false,
		},
		"gcp-balanced": {
			PerGBMonth:  0.10,
			PerIOPS:     0,
			Provisioned: false,
		},
		"gcp-pd-balanced": {
			PerGBMonth:  0.10,
			PerIOPS:     0,
			Provisioned: false,
		},
		"gcp-ssd": {
			PerGBMonth:  0.17,
			PerIOPS:     0,
			Provisioned: false,
		},
		"gcp-pd-ssd": {
			PerGBMonth:  0.17,
			PerIOPS:     0,
			Provisioned: false,
		},
		"gcp-pd-extreme": {
			PerGBMonth:  0.125,
			PerIOPS:     0.05, // Provisioned IOPS
			Provisioned: true,
		},

		// Azure Managed Disks pricing
		"azure-standard-hdd": {
			PerGBMonth:  0.045,
			PerIOPS:     0,
			Provisioned: false,
		},
		"azure-standard-ssd": {
			PerGBMonth:  0.075,
			PerIOPS:     0,
			Provisioned: false,
		},
		"azure-premium": {
			PerGBMonth:  0.12,
			PerIOPS:     0,
			Provisioned: false,
		},
		"azure-premium-v2": {
			PerGBMonth:  0.08,
			PerIOPS:     0.005, // Above baseline
			Provisioned: true,
		},
		"azure-ultra": {
			PerGBMonth:  0.15,
			PerIOPS:     0.10,
			Provisioned: true,
		},
		"azure-managed-premium": {
			PerGBMonth:  0.12,
			PerIOPS:     0,
			Provisioned: false,
		},

		// Unknown/Default pricing
		"unknown-default": {
			PerGBMonth:  0.10,
			PerIOPS:     0,
			Provisioned: false,
		},
	}
}
