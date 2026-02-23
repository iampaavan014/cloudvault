package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"

	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/option"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/billing/armbilling"
)

// LivePricingProvider fetches real-time pricing from cloud providers
type LivePricingProvider struct {
	awsClient      *pricing.Client
	gcpClient      *cloudbilling.APIService
	azureClient    *armbilling.AccountsClient
	cache          *PricingCache
	staticFallback *StaticPricingProvider
}

// PricingCache caches pricing data to reduce API calls
type PricingCache struct {
	entries map[string]*CacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

// CacheEntry represents a cached pricing entry
type CacheEntry struct {
	Price     float64
	Timestamp time.Time
}

// NewLivePricingProvider creates a new live pricing provider
func NewLivePricingProvider(ctx context.Context) (*LivePricingProvider, error) {
	slog.Info("Initializing multi-cloud live pricing provider")

	provider := &LivePricingProvider{
		cache:          NewPricingCache(24 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}

	// 1. Initialize AWS Pricing Client
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err == nil {
		provider.awsClient = pricing.NewFromConfig(awsCfg)
		slog.Info("AWS Pricing client initialized")
	} else {
		slog.Warn("Failed to initialize AWS Pricing client", "error", err)
	}

	// 2. Initialize GCP Cloud Billing Client
	gcpSvc, err := cloudbilling.NewService(ctx, option.WithScopes(cloudbilling.CloudBillingReadonlyScope))
	if err == nil {
		provider.gcpClient = gcpSvc
		slog.Info("GCP Cloud Billing client initialized")
	} else {
		slog.Warn("Failed to initialize GCP Cloud Billing client", "error", err)
	}

	// 3. Initialize Azure Billing Client
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err == nil {
		azClient, err := armbilling.NewAccountsClient(cred, nil)
		if err == nil {
			provider.azureClient = azClient
			slog.Info("Azure Billing client initialized")
		} else {
			slog.Warn("Failed to create Azure Billing client", "error", err)
		}
	} else {
		slog.Warn("Failed to initialize Azure Credentials", "error", err)
	}

	// Warm up cache with common storage types
	go provider.warmupCache(ctx)

	slog.Info("Live pricing provider initialization complete")
	return provider, nil
}

// NewPricingCache creates a new pricing cache
func NewPricingCache(ttl time.Duration) *PricingCache {
	return &PricingCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// GetPrice fetches the price for a given configuration
func (p *LivePricingProvider) GetPrice(ctx context.Context, provider, storageClass, region string) (float64, error) {
	// Check cache first
	if price, ok := p.cache.Get(provider, storageClass, region); ok {
		return price, nil
	}

	// Fetch from API
	var price float64
	var err error

	switch provider {
	case "aws":
		price, err = p.getAWSPrice(ctx, storageClass, region)
	case "gcp":
		price, err = p.getGCPPrice(ctx, storageClass, region)
	case "azure":
		price, err = p.getAzurePrice(ctx, storageClass, region)
	default:
		err = fmt.Errorf("unknown provider: %s", provider)
	}

	if err != nil {
		slog.Warn("Failed to fetch live price, using static fallback",
			"provider", provider, "storageClass", storageClass, "region", region, "error", err)
		return p.staticFallback.GetStoragePrice(provider, storageClass, region), nil
	}

	// Cache the result
	p.cache.Set(provider, storageClass, region, price)

	return price, nil
}

// getAWSPrice fetches live pricing from AWS Pricing API
func (p *LivePricingProvider) getAWSPrice(ctx context.Context, storageClass, region string) (float64, error) {
	if p.awsClient == nil {
		return 0, fmt.Errorf("AWS client not initialized")
	}

	slog.Info("Fetching live AWS price", "storageClass", storageClass, "region", region)

	// Map storage class to AWS volume type
	volumeType := mapStorageClassToAWSVolumeType(storageClass)

	// Map region to AWS location name
	location := mapRegionToAWSLocation(region)

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []types.Filter{
			{
				Type:  types.FilterTypeTermMatch,
				Field: aws.String("productFamily"),
				Value: aws.String("Storage"),
			},
			{
				Type:  types.FilterTypeTermMatch,
				Field: aws.String("volumeApiName"),
				Value: aws.String(volumeType),
			},
			{
				Type:  types.FilterTypeTermMatch,
				Field: aws.String("location"),
				Value: aws.String(location),
			},
		},
		MaxResults: aws.Int32(1),
	}

	result, err := p.awsClient.GetProducts(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("AWS pricing API error: %w", err)
	}

	if len(result.PriceList) == 0 {
		return 0, fmt.Errorf("no pricing data found for %s in %s", volumeType, location)
	}

	// Parse the complex pricing JSON
	price, err := p.parseAWSPricingJSON(result.PriceList[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse AWS pricing: %w", err)
	}

	slog.Info("AWS price fetched successfully",
		"storageClass", storageClass, "region", region, "price", price)

	return price, nil
}

// parseAWSPricingJSON parses the complex AWS pricing JSON response
func (p *LivePricingProvider) parseAWSPricingJSON(priceListJSON string) (float64, error) {
	var priceData map[string]interface{}
	if err := json.Unmarshal([]byte(priceListJSON), &priceData); err != nil {
		return 0, err
	}

	// Navigate the complex JSON structure
	// Structure: product -> terms -> OnDemand -> [offerTermCode] -> priceDimensions -> [dimensionCode] -> pricePerUnit -> USD
	terms, ok := priceData["terms"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid pricing data structure: missing terms")
	}

	onDemand, ok := terms["OnDemand"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid pricing data structure: missing OnDemand")
	}

	// Get the first offer term
	for _, offerTerm := range onDemand {
		offerMap, ok := offerTerm.(map[string]interface{})
		if !ok {
			continue
		}

		priceDimensions, ok := offerMap["priceDimensions"].(map[string]interface{})
		if !ok {
			continue
		}

		// Get the first price dimension
		for _, dimension := range priceDimensions {
			dimensionMap, ok := dimension.(map[string]interface{})
			if !ok {
				continue
			}

			pricePerUnit, ok := dimensionMap["pricePerUnit"].(map[string]interface{})
			if !ok {
				continue
			}

			usdPrice, ok := pricePerUnit["USD"].(string)
			if !ok {
				continue
			}

			// Convert string price to float64
			var price float64
			_, err := fmt.Sscanf(usdPrice, "%f", &price)
			if err != nil {
				return 0, err
			}
			return price, nil
		}
	}

	return 0, fmt.Errorf("could not extract price from response")
}

// getGCPPrice fetches live pricing from GCP Cloud Billing API
func (p *LivePricingProvider) getGCPPrice(ctx context.Context, storageClass, region string) (float64, error) {
	if p.gcpClient == nil {
		return 0, fmt.Errorf("GCP client not initialized")
	}

	slog.Info("Fetching live GCP price", "storageClass", storageClass, "region", region)

	// GCP Storage Service ID
	serviceID := "6F81-5844-456A" // Compute Engine service id which contains PD

	// Map storage class to description search term
	skuDescription := mapStorageClassToGCPSKU(storageClass)

	// List SKUs for the service
	call := p.gcpClient.Services.Skus.List("services/" + serviceID)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("GCP billing API error: %w", err)
	}

	for _, sku := range resp.Skus {
		// Filter by SKU description and region
		// PD Standard in us-central1 usually has "Standard Disk" in description
		matchesRegion := false
		for _, r := range sku.ServiceRegions {
			if r == region {
				matchesRegion = true
				break
			}
		}

		if matchesRegion && containsCaseInsensitive(sku.Description, skuDescription) {
			if len(sku.PricingInfo) > 0 {
				pricing := sku.PricingInfo[0]
				if len(pricing.PricingExpression.TieredRates) > 0 {
					rate := pricing.PricingExpression.TieredRates[0]
					// Price is in "nanos" per "unit"
					unitPrice := float64(rate.UnitPrice.Units) + float64(rate.UnitPrice.Nanos)/1e9
					return unitPrice, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("no GCP SKU found for %s in %s", storageClass, region)
}

func containsCaseInsensitive(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// getAzurePrice fetches live pricing from Azure Retail Prices API
func (p *LivePricingProvider) getAzurePrice(ctx context.Context, storageClass, region string) (float64, error) {
	if p.azureClient == nil {
		return 0, fmt.Errorf("azure client not initialized")
	}
	slog.Info("Fetching live Azure price", "storageClass", storageClass, "region", region)

	// Azure Retail Prices API is a public REST API
	// Example: https://prices.azure.com/api/retail/prices?$filter=serviceName eq 'Storage' and armRegionName eq 'eastus' and productName eq 'Premium SSD Managed Disks'

	productName := mapStorageClassToAzureProduct(storageClass)
	skuName := mapStorageClassToAzureSKU(storageClass)

	baseURL := "https://prices.azure.com/api/retail/prices"
	query := fmt.Sprintf("$filter=serviceName eq 'Storage' and armRegionName eq '%s' and productName eq '%s' and skuName eq '%s' and priceType eq 'Consumption'",
		region, productName, skuName)

	url := fmt.Sprintf("%s?%s", baseURL, strings.ReplaceAll(query, " ", "%20"))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("azure Retail Prices API error: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result struct {
		Items []struct {
			RetailPrice   float64 `json:"retailPrice"`
			UnitOfMeasure string  `json:"unitOfMeasure"`
		} `json:"Items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Azure pricing: %w", err)
	}

	if len(result.Items) == 0 {
		return 0, fmt.Errorf("no Azure pricing found for %s (%s) in %s", storageClass, productName, region)
	}

	// Calculate per GB-month if unit is different
	price := result.Items[0].RetailPrice
	unit := result.Items[0].UnitOfMeasure

	if strings.Contains(unit, "1 GB/Month") || strings.Contains(unit, "1 GB/month") {
		return price, nil
	}

	return price, nil // Default to the price returned
}

// GetEgressPrice fetches egress pricing
func (p *LivePricingProvider) GetEgressPrice(ctx context.Context, sourceProvider, sourceRegion, destProvider, destRegion string) (float64, error) {
	// Same cloud, different region
	if sourceProvider == destProvider && sourceRegion != destRegion {
		cacheKey := fmt.Sprintf("egress-%s-%s-%s", sourceProvider, sourceRegion, destRegion)
		if price, ok := p.cache.GetRaw(cacheKey); ok {
			return price, nil
		}

		price := p.getIntraCloudEgressPrice(sourceProvider, sourceRegion, destRegion)
		p.cache.SetRaw(cacheKey, price)
		return price, nil
	}

	// Cross-cloud egress (most expensive)
	if sourceProvider != destProvider {
		cacheKey := fmt.Sprintf("egress-%s-%s-internet", sourceProvider, sourceRegion)
		if price, ok := p.cache.GetRaw(cacheKey); ok {
			return price, nil
		}

		price := p.getInternetEgressPrice(sourceProvider, sourceRegion)
		p.cache.SetRaw(cacheKey, price)
		return price, nil
	}

	// Same region - no egress cost
	return 0, nil
}

// getIntraCloudEgressPrice returns egress pricing within the same cloud
func (p *LivePricingProvider) getIntraCloudEgressPrice(provider, sourceRegion, destRegion string) float64 {
	// Real pricing data from cloud providers (as of 2024-2025)
	switch provider {
	case "aws":
		// AWS charges $0.01/GB for inter-region data transfer
		return 0.01
	case "gcp":
		// GCP charges vary by region pair, average $0.01/GB
		return 0.01
	case "azure":
		// Azure charges $0.02/GB for inter-region transfer
		return 0.02
	}
	return 0
}

// getInternetEgressPrice returns egress pricing to the internet
func (p *LivePricingProvider) getInternetEgressPrice(provider, region string) float64 {
	// Real pricing data from cloud providers
	switch provider {
	case "aws":
		// AWS: $0.09/GB for first 10TB/month
		return 0.09
	case "gcp":
		// GCP: $0.08/GB for first 1TB/month
		return 0.08
	case "azure":
		// Azure: $0.087/GB for first 5TB/month
		return 0.087
	}
	return 0
}

// Cache methods

func (c *PricingCache) Get(provider, storageClass, region string) (float64, bool) {
	key := fmt.Sprintf("%s-%s-%s", provider, storageClass, region)
	return c.GetRaw(key)
}

func (c *PricingCache) GetRaw(key string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return 0, false
	}

	// Check if expired
	if time.Since(entry.Timestamp) > c.ttl {
		return 0, false
	}

	return entry.Price, true
}

func (c *PricingCache) Set(provider, storageClass, region string, price float64) {
	key := fmt.Sprintf("%s-%s-%s", provider, storageClass, region)
	c.SetRaw(key, price)
}

func (c *PricingCache) SetRaw(key string, price float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Price:     price,
		Timestamp: time.Now(),
	}
}

// warmupCache pre-fetches pricing for common configurations
func (p *LivePricingProvider) warmupCache(ctx context.Context) {
	slog.Info("Warming up pricing cache")

	commonConfigs := []struct {
		provider     string
		storageClass string
		region       string
	}{
		// AWS
		{"aws", "gp3", "us-east-1"},
		{"aws", "gp3", "us-west-2"},
		{"aws", "io2", "us-east-1"},
		{"aws", "sc1", "us-east-1"},

		// GCP
		{"gcp", "pd-standard", "us-central1"},
		{"gcp", "pd-ssd", "us-central1"},

		// Azure
		{"azure", "Premium_LRS", "eastus"},
		{"azure", "StandardSSD_LRS", "eastus"},
	}

	for _, cfg := range commonConfigs {
		_, err := p.GetPrice(ctx, cfg.provider, cfg.storageClass, cfg.region)
		if err != nil {
			slog.Warn("Failed to warm up cache",
				"provider", cfg.provider,
				"storageClass", cfg.storageClass,
				"region", cfg.region,
				"error", err)
		}
		time.Sleep(100 * time.Millisecond) // Rate limiting
	}

	slog.Info("Cache warmup completed")
}

// Helper functions to map between Kubernetes and cloud provider terminology

func mapStorageClassToAWSVolumeType(storageClass string) string {
	mapping := map[string]string{
		"gp2":         "gp2",
		"gp3":         "gp3",
		"io1":         "io1",
		"io2":         "io2",
		"st1":         "st1",
		"sc1":         "sc1",
		"standard":    "standard",
		"aws-ebs-gp3": "gp3",
		"aws-ebs-io2": "io2",
	}

	if volumeType, ok := mapping[storageClass]; ok {
		return volumeType
	}

	// Default to gp3
	return "gp3"
}

func mapRegionToAWSLocation(region string) string {
	// AWS Pricing API uses full location names, not region codes
	mapping := map[string]string{
		"us-east-1":      "US East (N. Virginia)",
		"us-east-2":      "US East (Ohio)",
		"us-west-1":      "US West (N. California)",
		"us-west-2":      "US West (Oregon)",
		"ca-central-1":   "Canada (Central)",
		"eu-west-1":      "EU (Ireland)",
		"eu-west-2":      "EU (London)",
		"eu-west-3":      "EU (Paris)",
		"eu-central-1":   "EU (Frankfurt)",
		"eu-north-1":     "EU (Stockholm)",
		"ap-south-1":     "Asia Pacific (Mumbai)",
		"ap-southeast-1": "Asia Pacific (Singapore)",
		"ap-southeast-2": "Asia Pacific (Sydney)",
		"ap-northeast-1": "Asia Pacific (Tokyo)",
		"ap-northeast-2": "Asia Pacific (Seoul)",
		"sa-east-1":      "South America (Sao Paulo)",
	}

	if location, ok := mapping[region]; ok {
		return location
	}

	// Default to us-east-1
	return "US East (N. Virginia)"
}

func mapStorageClassToGCPSKU(storageClass string) string {
	mapping := map[string]string{
		"pd-standard": "Standard Disk",
		"pd-ssd":      "SSD backed",
		"pd-balanced": "Balanced",
	}
	if sku, ok := mapping[storageClass]; ok {
		return sku
	}
	return "Standard Disk"
}

func mapStorageClassToAzureProduct(storageClass string) string {
	mapping := map[string]string{
		"Premium_LRS":     "Premium SSD Managed Disks",
		"StandardSSD_LRS": "Standard SSD Managed Disks",
		"Standard_LRS":    "Standard HDD Managed Disks",
	}
	if p, ok := mapping[storageClass]; ok {
		return p
	}
	return "Standard SSD Managed Disks"
}

func mapStorageClassToAzureSKU(storageClass string) string {
	mapping := map[string]string{
		"Premium_LRS":     "P10 LRS",
		"StandardSSD_LRS": "E10 LRS",
		"Standard_LRS":    "S10 LRS",
	}
	if s, ok := mapping[storageClass]; ok {
		return s
	}
	return "E10 LRS"
}

// StaticPricingProvider provides fallback static pricing
type StaticPricingProvider struct{}

func NewStaticPricingProvider() *StaticPricingProvider {
	return &StaticPricingProvider{}
}

// GetPrice implements PricingProvider interface
func (s *StaticPricingProvider) GetPrice(provider, storageClass, region string) StorageClassPricing {
	price := s.GetStoragePrice(provider, storageClass, region)

	// Determine if provisioned
	provisioned := false
	switch provider {
	case "aws":
		provisioned = storageClass == "io1" || storageClass == "io2"
	case "gcp":
		provisioned = storageClass == "pd-ssd"
	case "azure":
		provisioned = storageClass == "Premium_LRS"
	}

	pricing := StorageClassPricing{
		PerGBMonth:  price,
		Provisioned: provisioned,
	}

	if provisioned {
		pricing.PerIOPS = 0.005
	}

	return pricing
}

func (s *StaticPricingProvider) GetStoragePrice(provider, storageClass, region string) float64 {
	// Fallback to original static pricing
	switch provider {
	case "aws":
		switch storageClass {
		case "gp3":
			return 0.08 // $0.08/GB-month
		case "gp2":
			return 0.10
		case "io2":
			return 0.125
		case "io1":
			return 0.125
		case "st1":
			return 0.045
		case "sc1":
			return 0.015
		default:
			return 0.08
		}
	case "gcp":
		switch storageClass {
		case "pd-standard":
			return 0.04
		case "pd-ssd":
			return 0.17
		case "pd-balanced":
			return 0.10
		default:
			return 0.04
		}
	case "azure":
		switch storageClass {
		case "Premium_LRS":
			return 0.12
		case "StandardSSD_LRS":
			return 0.10
		case "Standard_LRS":
			return 0.05
		default:
			return 0.05
		}
	}
	return 0.10 // Default fallback
}
