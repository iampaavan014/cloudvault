package cost

import (
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

type mockPricingProvider struct{}

func (m *mockPricingProvider) GetPrice(provider, storageClass, region string) StorageClassPricing {
	if provider == "aws" && storageClass == "gp3" {
		return StorageClassPricing{PerGBMonth: 0.08, PerIOPS: 0.005, Provisioned: true}
	}
	return StorageClassPricing{PerGBMonth: 0.10, PerIOPS: 0, Provisioned: false}
}

func TestCalculator_CalculatePVCCost(t *testing.T) {
	calc := NewCalculatorWithProvider(&mockPricingProvider{})

	tests := []struct {
		name         string
		metric       types.PVCMetric
		provider     string
		expectedCost float64
	}{
		{
			name: "aws-gp3-100GB-low-iops",
			metric: types.PVCMetric{
				SizeBytes:    100 * 1024 * 1024 * 1024,
				StorageClass: "gp3",
				ReadIOPS:     100,
				WriteIOPS:    100,
			},
			provider:     "aws",
			expectedCost: 8.0, // 100 * 0.08
		},
		{
			name: "aws-gp3-100GB-high-iops",
			metric: types.PVCMetric{
				SizeBytes:    100 * 1024 * 1024 * 1024,
				StorageClass: "gp3",
				ReadIOPS:     3000,
				WriteIOPS:    1000, // Total 4000
			},
			provider:     "aws",
			expectedCost: 8.0 + (1000 * 0.005), // 8.0 + 5.0 = 13.0
		},
		{
			name: "default-100GB",
			metric: types.PVCMetric{
				SizeBytes:    100 * 1024 * 1024 * 1024,
				StorageClass: "standard",
			},
			provider:     "gcp",
			expectedCost: 10.0, // 100 * 0.10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := calc.CalculatePVCCost(&tt.metric, tt.provider)
			if cost != tt.expectedCost {
				t.Errorf("Expected cost %.2f, got %.2f", tt.expectedCost, cost)
			}
		})
	}
}

func TestCalculator_GenerateSummary(t *testing.T) {
	calc := NewCalculatorWithProvider(&mockPricingProvider{})

	metrics := []types.PVCMetric{
		{
			Name:         "pvc-1",
			Namespace:    "default",
			SizeBytes:    100 * 1024 * 1024 * 1024,
			StorageClass: "gp3",
		},
		{
			Name:         "pvc-2",
			Namespace:    "production",
			SizeBytes:    200 * 1024 * 1024 * 1024,
			StorageClass: "standard",
		},
	}

	summary := calc.GenerateSummary(metrics, "aws")

	if summary.TotalMonthlyCost != 28.0 { // 8.0 + 20.0
		t.Errorf("Expected total cost 28.0, got %.2f", summary.TotalMonthlyCost)
	}

	if summary.ByNamespace["default"] != 8.0 {
		t.Errorf("Expected default ns cost 8.0, got %.2f", summary.ByNamespace["default"])
	}

	if summary.ByNamespace["production"] != 20.0 {
		t.Errorf("Expected production ns cost 20.0, got %.2f", summary.ByNamespace["production"])
	}
}

func TestCalculator_CalculateEgressCost(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		bytes    int64
		srcCloud string
		srcReg   string
		dstCloud string
		dstReg   string
		expected float64
	}{
		{"same-region", 100 * 1024 * 1024 * 1024, "aws", "us-east-1", "aws", "us-east-1", 0},
		{"same-cloud-diff-region", 100 * 1024 * 1024 * 1024, "aws", "us-east-1", "aws", "us-west-2", 2.0}, // 100 * 0.02
		{"cross-cloud", 100 * 1024 * 1024 * 1024, "aws", "us-east-1", "gcp", "us-central1", 9.0},          // 100 * 0.09
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := calc.CalculateEgressCost(tt.bytes, tt.srcCloud, tt.srcReg, tt.dstCloud, tt.dstReg)
			if cost != tt.expected {
				t.Errorf("Expected egress cost %.2f, got %.2f", tt.expected, cost)
			}
		})
	}
}
