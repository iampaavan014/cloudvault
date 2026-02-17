package cost

import (
	"testing"
)

func TestStaticPricingProvider(t *testing.T) {
	provider := NewStaticPricingProvider()

	tests := []struct {
		name         string
		cloudProv    string
		storageClass string
		expectedCost float64
		expectProv   bool
	}{
		{
			name:         "AWS gp3 (Known)",
			cloudProv:    "aws",
			storageClass: "gp3",
			expectedCost: 0.08,
			expectProv:   true,
		},
		{
			name:         "GCP Standard (Known)",
			cloudProv:    "gcp",
			storageClass: "standard",
			expectedCost: 0.04,
			expectProv:   false,
		},
		{
			name:         "Unknown Class (Fallback to Provider Default)",
			cloudProv:    "aws",
			storageClass: "unknown-sc",
			expectedCost: 0.10, // aws-default -> gp2
			expectProv:   false,
		},
		{
			name:         "Unknown Provider (Global Default)",
			cloudProv:    "alibaba",
			storageClass: "standard",
			expectedCost: 0.10,
			expectProv:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := provider.GetPrice(tt.cloudProv, tt.storageClass, "us-east-1")

			if pricing.PerGBMonth != tt.expectedCost {
				t.Errorf("expected cost %.2f, got %.2f", tt.expectedCost, pricing.PerGBMonth)
			}
			if pricing.Provisioned != tt.expectProv {
				t.Errorf("expected provisioned %v, got %v", tt.expectProv, pricing.Provisioned)
			}
		})
	}
}
