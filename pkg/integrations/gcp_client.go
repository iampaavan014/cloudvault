package integrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	billing "google.golang.org/api/cloudbilling/v1"
)

// GCPClient handles real-time interactions with GCP Cloud Billing API.
type GCPClient struct {
	service *billing.APIService
}

func NewGCPClient(cfg *types.Config) *GCPClient {
	ctx := context.Background()
	service, err := billing.NewService(ctx)
	if err != nil {
		slog.Error("Failed to create GCP billing service", "error", err)
		return &GCPClient{}
	}
	return &GCPClient{service: service}
}

// GetStoragePrice fetches real-time GCP prices dynamically.
func (c *GCPClient) GetStoragePrice(ctx context.Context, storageClass, region string) (float64, error) {

	return 0.0, fmt.Errorf("gcp cloud billing api integration in progress")
}
