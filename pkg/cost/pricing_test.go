package cost

import (
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

func TestMultiCloudPricingProvider(t *testing.T) {
	cfg := &types.Config{}
	aws := integrations.NewAWSClient(cfg)
	gcp := integrations.NewGCPClient(cfg)
	azure := integrations.NewAzureClient(cfg)

	p := NewMultiCloudPricingProvider(aws, gcp, azure)

	t.Run("Default Fallback for Errors", func(t *testing.T) {
		// Since clients are initialized but APIs aren't reachable/realized in test,
		// it should return the global default 0.10
		price := p.GetPrice("aws", "gp3", "us-east-1")
		if price.PerGBMonth != 0.10 {
			t.Errorf("Expected fallback 0.10, got %f", price.PerGBMonth)
		}
	})

	t.Run("Unsupported Provider", func(t *testing.T) {
		price := p.GetPrice("digitalocean", "standard", "nyc1")
		if price.PerGBMonth != 0.10 {
			t.Errorf("Expected fallback 0.10 for unknown provider, got %f", price.PerGBMonth)
		}
	})

	t.Run("AWS Provider", func(t *testing.T) {
		price := p.GetPrice("aws", "gp3", "us-east-1")
		if price.PerGBMonth <= 0 {
			t.Errorf("Expected positive price, got %f", price.PerGBMonth)
		}
	})

	t.Run("GCP Provider", func(t *testing.T) {
		price := p.GetPrice("gcp", "pd-standard", "us-central1")
		if price.PerGBMonth <= 0 {
			t.Errorf("Expected positive price, got %f", price.PerGBMonth)
		}
	})

	t.Run("Azure Provider", func(t *testing.T) {
		price := p.GetPrice("azure", "Premium_LRS", "eastus")
		if price.PerGBMonth <= 0 {
			t.Errorf("Expected positive price, got %f", price.PerGBMonth)
		}
	})
}

func TestStaticPricingProvider(t *testing.T) {
	provider := NewStaticPricingProvider()

	tests := []struct {
		name         string
		provider     string
		storageClass string
		region       string
		wantPrice    float64
		wantIOPS     float64
		provisioned  bool
	}{
		{"AWS gp3", "aws", "gp3", "us-east-1", 0.08, 0, false},
		{"AWS gp2", "aws", "gp2", "us-east-1", 0.10, 0, false},
		{"AWS io1", "aws", "io1", "us-east-1", 0.125, 0.005, true},
		{"AWS io2", "aws", "io2", "us-east-1", 0.125, 0.005, true},
		{"AWS st1", "aws", "st1", "us-east-1", 0.045, 0, false},
		{"AWS sc1", "aws", "sc1", "us-east-1", 0.015, 0, false},
		{"AWS unknown", "aws", "unknown", "us-east-1", 0.08, 0, false},
		{"GCP pd-standard", "gcp", "pd-standard", "us-central1", 0.04, 0, false},
		{"GCP pd-ssd", "gcp", "pd-ssd", "us-central1", 0.17, 0.005, true},
		{"GCP pd-balanced", "gcp", "pd-balanced", "us-central1", 0.10, 0, false},
		{"GCP unknown", "gcp", "unknown", "us-central1", 0.04, 0, false},
		{"Azure Premium_LRS", "azure", "Premium_LRS", "eastus", 0.12, 0.005, true},
		{"Azure StandardSSD_LRS", "azure", "StandardSSD_LRS", "eastus", 0.10, 0, false},
		{"Azure Standard_LRS", "azure", "Standard_LRS", "eastus", 0.05, 0, false},
		{"Azure unknown", "azure", "unknown", "eastus", 0.05, 0, false},
		{"Unknown provider", "unknown", "standard", "region", 0.10, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := provider.GetPrice(tt.provider, tt.storageClass, tt.region)
			if pricing.PerGBMonth != tt.wantPrice {
				t.Errorf("PerGBMonth = %f, want %f", pricing.PerGBMonth, tt.wantPrice)
			}
			if pricing.PerIOPS != tt.wantIOPS {
				t.Errorf("PerIOPS = %f, want %f", pricing.PerIOPS, tt.wantIOPS)
			}
			if pricing.Provisioned != tt.provisioned {
				t.Errorf("Provisioned = %v, want %v", pricing.Provisioned, tt.provisioned)
			}
		})
	}
}

func TestStaticPricingProvider_GetStoragePrice(t *testing.T) {
	provider := NewStaticPricingProvider()

	tests := []struct {
		name         string
		provider     string
		storageClass string
		region       string
		want         float64
	}{
		{"AWS gp3", "aws", "gp3", "us-east-1", 0.08},
		{"GCP pd-standard", "gcp", "pd-standard", "us-central1", 0.04},
		{"Azure Premium_LRS", "azure", "Premium_LRS", "eastus", 0.12},
		{"Unknown provider", "unknown", "any", "any", 0.10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.GetStoragePrice(tt.provider, tt.storageClass, tt.region)
			if got != tt.want {
				t.Errorf("GetStoragePrice() = %f, want %f", got, tt.want)
			}
		})
	}
}
