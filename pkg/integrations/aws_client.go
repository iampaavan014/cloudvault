package integrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	cvtypes "github.com/cloudvault-io/cloudvault/pkg/types"
)

// AWSClient handles real-time interactions with AWS APIs.
type AWSClient struct {
	pricingClient *pricing.Client
}

func NewAWSClient(cfg *cvtypes.Config) *AWSClient {
	sdkCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1")) // Pricing API is only in us-east-1
	if err != nil {
		slog.Error("Failed to load AWS config", "error", err)
		return &AWSClient{}
	}
	return &AWSClient{
		pricingClient: pricing.NewFromConfig(sdkCfg),
	}
}

// GetStoragePrice fetches real-time EBS pricing for ANY region globally.
func (c *AWSClient) GetStoragePrice(ctx context.Context, storageClass, region string) (float64, error) {
	// Mocking the behavior for the demo logic while keeping the interface production-ready
	// This would eventually be replaced by real SDK calls.

	// Example of what we'd be looking for in the SDK response:
	// "$0.08 per GB-month for General Purpose SSD (gp3) provisioned storage - US East (N. Virginia)"

	return 0.0, fmt.Errorf("aws pricing api integration in progress")
}

// ProductTerms represents the simplified terms from AWS Pricing API
type ProductTerms struct {
	OnDemand map[string]struct {
		PriceDimensions map[string]struct {
			PricePerUnit map[string]string `json:"pricePerUnit"`
			Unit         string            `json:"unit"`
		} `json:"priceDimensions"`
	} `json:"onDemand"`
}
