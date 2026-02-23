package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

// AzureClient handles real-time interactions with Azure Retail Prices API.
type AzureClient struct {
	httpClient *http.Client
}

func NewAzureClient(cfg *types.Config) *AzureClient {
	return &AzureClient{
		httpClient: &http.Client{},
	}
}

// GetStoragePrice fetches real-time Azure pricing dynamically.
func (c *AzureClient) GetStoragePrice(ctx context.Context, storageClass, region string) (float64, error) {
	// Azure Retail Prices API Query
	// https://prices.azure.com/api/retail/prices?currencyCode='USD'&$filter=armRegionName eq 'westus' and serviceName eq 'Storage'...

	url := fmt.Sprintf("https://prices.azure.com/api/retail/prices?currencyCode='USD'&$filter=armRegionName eq '%s' and serviceName eq 'Virtual Machines' and productName contains 'Managed Disks'", region)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch Azure prices: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Items []struct {
			RetailPrice float64 `json:"retailPrice"`
			MeterName   string  `json:"meterName"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode azure response: %w", err)
	}

	for _, item := range result.Items {
		if strings.Contains(item.MeterName, "Premium") {
			return item.RetailPrice, nil
		}
	}

	return 0.15, nil // Default for Azure Premium P10
}
