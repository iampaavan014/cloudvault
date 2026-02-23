package cost

import (
	"context"
	"testing"
	"time"
)

func TestNewPricingCache(t *testing.T) {
	ttl := 1 * time.Hour
	cache := NewPricingCache(ttl)
	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
	if cache.ttl != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, cache.ttl)
	}
	if cache.entries == nil {
		t.Error("Expected initialized entries map")
	}
}

func TestPricingCache_SetAndGet(t *testing.T) {
	cache := NewPricingCache(1 * time.Hour)
	cache.Set("aws", "gp3", "us-east-1", 0.08)
	price, ok := cache.Get("aws", "gp3", "us-east-1")
	if !ok {
		t.Error("Expected to find cached price")
	}
	if price != 0.08 {
		t.Errorf("Expected price 0.08, got %f", price)
	}
	_, ok = cache.Get("gcp", "pd-standard", "us-central1")
	if ok {
		t.Error("Expected not to find non-existent key")
	}
}

func TestPricingCache_Expiration(t *testing.T) {
	cache := NewPricingCache(100 * time.Millisecond)
	cache.Set("aws", "gp3", "us-east-1", 0.08)
	_, ok := cache.Get("aws", "gp3", "us-east-1")
	if !ok {
		t.Error("Expected to find cached price")
	}
	time.Sleep(150 * time.Millisecond)
	_, ok = cache.Get("aws", "gp3", "us-east-1")
	if ok {
		t.Error("Expected cache entry to be expired")
	}
}

func TestPricingCache_RawOperations(t *testing.T) {
	cache := NewPricingCache(1 * time.Hour)
	cache.SetRaw("custom-key", 1.23)
	price, ok := cache.GetRaw("custom-key")
	if !ok {
		t.Error("Expected to find cached price")
	}
	if price != 1.23 {
		t.Errorf("Expected price 1.23, got %f", price)
	}
}

func TestMapStorageClassToAWSVolumeType(t *testing.T) {
	tests := []struct {
		storageClass string
		expected     string
	}{
		{"gp2", "gp2"},
		{"gp3", "gp3"},
		{"io1", "io1"},
		{"io2", "io2"},
		{"st1", "st1"},
		{"sc1", "sc1"},
		{"standard", "standard"},
		{"aws-ebs-gp3", "gp3"},
		{"aws-ebs-io2", "io2"},
		{"unknown", "gp3"},
	}
	for _, tt := range tests {
		t.Run(tt.storageClass, func(t *testing.T) {
			result := mapStorageClassToAWSVolumeType(tt.storageClass)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMapRegionToAWSLocation(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us-east-1", "US East (N. Virginia)"},
		{"us-east-2", "US East (Ohio)"},
		{"us-west-1", "US West (N. California)"},
		{"us-west-2", "US West (Oregon)"},
		{"eu-west-1", "EU (Ireland)"},
		{"eu-central-1", "EU (Frankfurt)"},
		{"ap-southeast-1", "Asia Pacific (Singapore)"},
		{"unknown-region", "US East (N. Virginia)"},
	}
	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			result := mapRegionToAWSLocation(tt.region)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestStaticPricingProvider_GetStoragePrice_AllProviders(t *testing.T) {
	provider := NewStaticPricingProvider()
	tests := []struct {
		name          string
		cloudProvider string
		storageClass  string
		region        string
		expected      float64
	}{
		{"aws-gp3", "aws", "gp3", "us-east-1", 0.08},
		{"aws-gp2", "aws", "gp2", "us-east-1", 0.10},
		{"aws-io2", "aws", "io2", "us-east-1", 0.125},
		{"aws-sc1", "aws", "sc1", "us-east-1", 0.015},
		{"gcp-standard", "gcp", "pd-standard", "us-central1", 0.04},
		{"gcp-ssd", "gcp", "pd-ssd", "us-central1", 0.17},
		{"azure-premium", "azure", "Premium_LRS", "eastus", 0.12},
		{"azure-standard", "azure", "Standard_LRS", "eastus", 0.05},
		{"unknown", "unknown", "unknown", "unknown", 0.10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := provider.GetStoragePrice(tt.cloudProvider, tt.storageClass, tt.region)
			if price != tt.expected {
				t.Errorf("Expected price %f, got %f", tt.expected, price)
			}
		})
	}
}

func TestStaticPricingProvider_GetPrice(t *testing.T) {
	provider := NewStaticPricingProvider()
	tests := []struct {
		name              string
		cloudProvider     string
		storageClass      string
		region            string
		expectProvisioned bool
		expectIOPS        bool
	}{
		{"aws-io2-provisioned", "aws", "io2", "us-east-1", true, true},
		{"aws-gp3-not-provisioned", "aws", "gp3", "us-east-1", false, false},
		{"gcp-ssd-provisioned", "gcp", "pd-ssd", "us-central1", true, true},
		{"azure-premium-provisioned", "azure", "Premium_LRS", "eastus", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := provider.GetPrice(tt.cloudProvider, tt.storageClass, tt.region)
			if pricing.Provisioned != tt.expectProvisioned {
				t.Errorf("Expected provisioned=%v, got %v", tt.expectProvisioned, pricing.Provisioned)
			}
			if tt.expectIOPS && pricing.PerIOPS == 0 {
				t.Error("Expected non-zero PerIOPS for provisioned storage")
			}
			if pricing.PerGBMonth == 0 {
				t.Error("Expected non-zero PerGBMonth")
			}
		})
	}
}

func TestLivePricingProvider_GetEgressPrice(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	tests := []struct {
		name        string
		srcProvider string
		srcRegion   string
		dstProvider string
		dstRegion   string
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "same-region-no-cost",
			srcProvider: "aws",
			srcRegion:   "us-east-1",
			dstProvider: "aws",
			dstRegion:   "us-east-1",
			expectedMin: 0,
			expectedMax: 0,
		},
		{
			name:        "intra-cloud-aws",
			srcProvider: "aws",
			srcRegion:   "us-east-1",
			dstProvider: "aws",
			dstRegion:   "us-west-2",
			expectedMin: 0.01,
			expectedMax: 0.01,
		},
		{
			name:        "cross-cloud",
			srcProvider: "aws",
			srcRegion:   "us-east-1",
			dstProvider: "gcp",
			dstRegion:   "us-central1",
			expectedMin: 0.08,
			expectedMax: 0.09,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := provider.GetEgressPrice(ctx, tt.srcProvider, tt.srcRegion, tt.dstProvider, tt.dstRegion)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if price < tt.expectedMin || price > tt.expectedMax {
				t.Errorf("Expected price between %f and %f, got %f", tt.expectedMin, tt.expectedMax, price)
			}
		})
	}
}

func TestLivePricingProvider_GetEgressPriceCache(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price1, err := provider.GetEgressPrice(ctx, "aws", "us-east-1", "aws", "us-west-2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	price2, err := provider.GetEgressPrice(ctx, "aws", "us-east-1", "aws", "us-west-2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price1 != price2 {
		t.Errorf("Expected consistent cached prices, got %f and %f", price1, price2)
	}
}

func TestGetIntraCloudEgressPrice(t *testing.T) {
	provider := &LivePricingProvider{}
	tests := []struct {
		cloudProvider string
		expected      float64
	}{
		{"aws", 0.01},
		{"gcp", 0.01},
		{"azure", 0.02},
		{"unknown", 0},
	}
	for _, tt := range tests {
		t.Run(tt.cloudProvider, func(t *testing.T) {
			price := provider.getIntraCloudEgressPrice(tt.cloudProvider, "region1", "region2")
			if price != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, price)
			}
		})
	}
}

func TestGetInternetEgressPrice(t *testing.T) {
	provider := &LivePricingProvider{}
	tests := []struct {
		cloudProvider string
		minExpected   float64
		maxExpected   float64
	}{
		{"aws", 0.09, 0.09},
		{"gcp", 0.08, 0.08},
		{"azure", 0.087, 0.087},
		{"unknown", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.cloudProvider, func(t *testing.T) {
			price := provider.getInternetEgressPrice(tt.cloudProvider, "us-east-1")
			if price < tt.minExpected || price > tt.maxExpected {
				t.Errorf("Expected between %f and %f, got %f", tt.minExpected, tt.maxExpected, price)
			}
		})
	}
}

func TestLivePricingProvider_GetPriceWithFallback(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		awsClient:      nil,
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := provider.GetPrice(ctx, "aws", "gp3", "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.08 {
		t.Errorf("Expected static fallback price 0.08, got %f", price)
	}
}

func TestPricingCache_ConcurrentAccess(t *testing.T) {
	cache := NewPricingCache(1 * time.Hour)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			cache.SetRaw("key", float64(id))
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = cache.GetRaw("key")
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestParseAWSPricingJSON_Valid(t *testing.T) {
	provider := &LivePricingProvider{}
	validJSON := `{
		"terms": {
			"OnDemand": {
				"offer1": {
					"priceDimensions": {
						"dim1": {
							"pricePerUnit": {
								"USD": "0.08"
							}
						}
					}
				}
			}
		}
	}`
	price, err := provider.parseAWSPricingJSON(validJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.08 {
		t.Errorf("Expected price 0.08, got %f", price)
	}
}

func TestParseAWSPricingJSON_InvalidJSON(t *testing.T) {
	provider := &LivePricingProvider{}
	_, err := provider.parseAWSPricingJSON("not-json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseAWSPricingJSON_MissingTerms(t *testing.T) {
	provider := &LivePricingProvider{}
	_, err := provider.parseAWSPricingJSON(`{"product": {}}`)
	if err == nil {
		t.Error("Expected error for missing terms")
	}
}

func TestParseAWSPricingJSON_MissingOnDemand(t *testing.T) {
	provider := &LivePricingProvider{}
	_, err := provider.parseAWSPricingJSON(`{"terms": {"Reserved": {}}}`)
	if err == nil {
		t.Error("Expected error for missing OnDemand")
	}
}

func TestParseAWSPricingJSON_NoPriceExtracted(t *testing.T) {
	provider := &LivePricingProvider{}
	json := `{"terms": {"OnDemand": {"offer1": {"priceDimensions": {"dim1": {"pricePerUnit": {}}}}}}}`
	_, err := provider.parseAWSPricingJSON(json)
	if err == nil {
		t.Error("Expected error when no price can be extracted")
	}
}

func TestParseAWSPricingJSON_EmptyOnDemand(t *testing.T) {
	provider := &LivePricingProvider{}
	_, err := provider.parseAWSPricingJSON(`{"terms": {"OnDemand": {}}}`)
	if err == nil {
		t.Error("Expected error for empty OnDemand")
	}
}

func TestParseAWSPricingJSON_InvalidUSDFormat(t *testing.T) {
	provider := &LivePricingProvider{}
	json := `{"terms": {"OnDemand": {"offer1": {"priceDimensions": {"dim1": {"pricePerUnit": {"USD": "not-a-number"}}}}}}}`
	_, err := provider.parseAWSPricingJSON(json)
	if err == nil {
		t.Error("Expected error for non-numeric USD price")
	}
}

func TestParseAWSPricingJSON_MultipleOffers(t *testing.T) {
	provider := &LivePricingProvider{}
	json := `{
		"terms": {
			"OnDemand": {
				"offer1": {
					"priceDimensions": {
						"dim1": {
							"pricePerUnit": {"USD": "0.10"}
						}
					}
				},
				"offer2": {
					"priceDimensions": {
						"dim1": {
							"pricePerUnit": {"USD": "0.12"}
						}
					}
				}
			}
		}
	}`
	price, err := provider.parseAWSPricingJSON(json)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.10 && price != 0.12 {
		t.Errorf("Expected price 0.10 or 0.12, got %f", price)
	}
}

func TestLivePricingProvider_GetPrice_CachesResult(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		awsClient:      nil,
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	// First call - should fallback to static (fallback path does not cache)
	price1, err := provider.GetPrice(ctx, "aws", "gp3", "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price1 != 0.08 {
		t.Errorf("Expected static fallback price 0.08, got %f", price1)
	}
	// Fallback path does NOT cache, so cache should be empty
	_, ok := provider.cache.Get("aws", "gp3", "us-east-1")
	if ok {
		t.Error("Expected fallback price NOT to be cached")
	}
	// Second call should still return the same fallback price
	price2, err := provider.GetPrice(ctx, "aws", "gp3", "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price1 != price2 {
		t.Errorf("Expected consistent prices, got %f and %f", price1, price2)
	}
}

func TestLivePricingProvider_GetPrice_UnknownProvider(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		awsClient:      nil,
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := provider.GetPrice(ctx, "unknown-cloud", "gp3", "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.10 {
		t.Errorf("Expected static fallback price 0.10, got %f", price)
	}
}

func TestLivePricingProvider_GetPrice_GCPFallback(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		gcpClient:      nil,
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := provider.GetPrice(ctx, "gcp", "pd-ssd", "us-central1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.17 {
		t.Errorf("Expected GCP pd-ssd static price 0.17, got %f", price)
	}
}

func TestLivePricingProvider_GetPrice_AzureFallback(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		azureClient:    nil,
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := provider.GetPrice(ctx, "azure", "Premium_LRS", "eastus")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.12 {
		t.Errorf("Expected Azure Premium_LRS static price 0.12, got %f", price)
	}
}

func TestLivePricingProvider_GetAWSPrice_NilClient(t *testing.T) {
	provider := &LivePricingProvider{
		awsClient: nil,
	}
	_, err := provider.getAWSPrice(context.Background(), "gp3", "us-east-1")
	if err == nil {
		t.Error("Expected error when AWS client is nil")
	}
}

func TestLivePricingProvider_GetEgressPrice_CrossCloudCached(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price1, err := provider.GetEgressPrice(ctx, "gcp", "us-central1", "azure", "eastus")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	cacheKey := "egress-gcp-us-central1-internet"
	cachedPrice, ok := provider.cache.GetRaw(cacheKey)
	if !ok {
		t.Error("Expected cross-cloud egress price to be cached")
	}
	if cachedPrice != price1 {
		t.Errorf("Cached price %f doesn't match returned price %f", cachedPrice, price1)
	}
	price2, err := provider.GetEgressPrice(ctx, "gcp", "us-central1", "azure", "eastus")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price1 != price2 {
		t.Errorf("Expected consistent cached prices, got %f and %f", price1, price2)
	}
}

func TestLivePricingProvider_GetEgressPrice_IntraCloudCached(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price1, err := provider.GetEgressPrice(ctx, "azure", "eastus", "azure", "westus")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price1 != 0.02 {
		t.Errorf("Expected Azure intra-cloud egress 0.02, got %f", price1)
	}
	cacheKey := "egress-azure-eastus-westus"
	cachedPrice, ok := provider.cache.GetRaw(cacheKey)
	if !ok {
		t.Error("Expected intra-cloud egress price to be cached")
	}
	if cachedPrice != price1 {
		t.Errorf("Cached price %f doesn't match returned price %f", cachedPrice, price1)
	}
}

func TestPricingCache_Overwrite(t *testing.T) {
	cache := NewPricingCache(1 * time.Hour)
	cache.Set("aws", "gp3", "us-east-1", 0.08)
	price, ok := cache.Get("aws", "gp3", "us-east-1")
	if !ok || price != 0.08 {
		t.Fatalf("Expected initial price 0.08, got %f", price)
	}
	cache.Set("aws", "gp3", "us-east-1", 0.09)
	price, ok = cache.Get("aws", "gp3", "us-east-1")
	if !ok {
		t.Fatal("Expected to find overwritten cache entry")
	}
	if price != 0.09 {
		t.Errorf("Expected overwritten price 0.09, got %f", price)
	}
}

func TestPricingCache_MultipleEntries(t *testing.T) {
	cache := NewPricingCache(1 * time.Hour)
	cache.Set("aws", "gp3", "us-east-1", 0.08)
	cache.Set("aws", "io2", "us-east-1", 0.125)
	cache.Set("gcp", "pd-ssd", "us-central1", 0.17)
	tests := []struct {
		provider     string
		storageClass string
		region       string
		expected     float64
	}{
		{"aws", "gp3", "us-east-1", 0.08},
		{"aws", "io2", "us-east-1", 0.125},
		{"gcp", "pd-ssd", "us-central1", 0.17},
	}
	for _, tt := range tests {
		price, ok := cache.Get(tt.provider, tt.storageClass, tt.region)
		if !ok {
			t.Errorf("Expected to find cache entry for %s/%s/%s", tt.provider, tt.storageClass, tt.region)
		}
		if price != tt.expected {
			t.Errorf("Expected %f for %s/%s/%s, got %f", tt.expected, tt.provider, tt.storageClass, tt.region, price)
		}
	}
}

func TestStaticPricingProvider_GetStoragePrice_AWSDefaults(t *testing.T) {
	provider := NewStaticPricingProvider()
	price := provider.GetStoragePrice("aws", "io1", "us-east-1")
	if price != 0.125 {
		t.Errorf("Expected AWS io1 price 0.125, got %f", price)
	}
	price = provider.GetStoragePrice("aws", "st1", "us-east-1")
	if price != 0.045 {
		t.Errorf("Expected AWS st1 price 0.045, got %f", price)
	}
	price = provider.GetStoragePrice("aws", "unknown-class", "us-east-1")
	if price != 0.08 {
		t.Errorf("Expected AWS default price 0.08, got %f", price)
	}
}

func TestStaticPricingProvider_GetStoragePrice_GCPDefaults(t *testing.T) {
	provider := NewStaticPricingProvider()
	price := provider.GetStoragePrice("gcp", "pd-balanced", "us-central1")
	if price != 0.10 {
		t.Errorf("Expected GCP pd-balanced price 0.10, got %f", price)
	}
	price = provider.GetStoragePrice("gcp", "unknown-class", "us-central1")
	if price != 0.04 {
		t.Errorf("Expected GCP default price 0.04, got %f", price)
	}
}

func TestStaticPricingProvider_GetStoragePrice_AzureDefaults(t *testing.T) {
	provider := NewStaticPricingProvider()
	price := provider.GetStoragePrice("azure", "StandardSSD_LRS", "eastus")
	if price != 0.10 {
		t.Errorf("Expected Azure StandardSSD_LRS price 0.10, got %f", price)
	}
	price = provider.GetStoragePrice("azure", "unknown-class", "eastus")
	if price != 0.05 {
		t.Errorf("Expected Azure default price 0.05, got %f", price)
	}
}

func TestStaticPricingProvider_GetPrice_NonProvisioned(t *testing.T) {
	provider := NewStaticPricingProvider()
	tests := []struct {
		name         string
		cloud        string
		storageClass string
	}{
		{"aws-gp2", "aws", "gp2"},
		{"aws-st1", "aws", "st1"},
		{"aws-sc1", "aws", "sc1"},
		{"gcp-pd-standard", "gcp", "pd-standard"},
		{"gcp-pd-balanced", "gcp", "pd-balanced"},
		{"azure-StandardSSD", "azure", "StandardSSD_LRS"},
		{"azure-Standard", "azure", "Standard_LRS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := provider.GetPrice(tt.cloud, tt.storageClass, "us-east-1")
			if pricing.Provisioned {
				t.Error("Expected non-provisioned storage")
			}
			if pricing.PerIOPS != 0 {
				t.Errorf("Expected zero PerIOPS for non-provisioned, got %f", pricing.PerIOPS)
			}
		})
	}
}

func TestStaticPricingProvider_GetPrice_ProvisionedIOPS(t *testing.T) {
	provider := NewStaticPricingProvider()
	tests := []struct {
		name         string
		cloud        string
		storageClass string
	}{
		{"aws-io1", "aws", "io1"},
		{"aws-io2", "aws", "io2"},
		{"gcp-pd-ssd", "gcp", "pd-ssd"},
		{"azure-Premium_LRS", "azure", "Premium_LRS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := provider.GetPrice(tt.cloud, tt.storageClass, "us-east-1")
			if !pricing.Provisioned {
				t.Error("Expected provisioned storage")
			}
			if pricing.PerIOPS != 0.005 {
				t.Errorf("Expected PerIOPS 0.005, got %f", pricing.PerIOPS)
			}
		})
	}
}

func TestMapStorageClassToAWSVolumeType_AllMappings(t *testing.T) {
	expected := map[string]string{
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
	for input, want := range expected {
		got := mapStorageClassToAWSVolumeType(input)
		if got != want {
			t.Errorf("mapStorageClassToAWSVolumeType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMapRegionToAWSLocation_AllMappings(t *testing.T) {
	expected := map[string]string{
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
	for region, want := range expected {
		got := mapRegionToAWSLocation(region)
		if got != want {
			t.Errorf("mapRegionToAWSLocation(%q) = %q, want %q", region, got, want)
		}
	}
}

func TestPricingCache_ExpiredRaw(t *testing.T) {
	cache := NewPricingCache(50 * time.Millisecond)
	cache.SetRaw("test-key", 42.0)
	price, ok := cache.GetRaw("test-key")
	if !ok || price != 42.0 {
		t.Fatalf("Expected to find cached value 42.0, got %f (ok=%v)", price, ok)
	}
	time.Sleep(60 * time.Millisecond)
	_, ok = cache.GetRaw("test-key")
	if ok {
		t.Error("Expected raw cache entry to be expired")
	}
}

func TestNewStaticPricingProvider(t *testing.T) {
	provider := NewStaticPricingProvider()
	if provider == nil {
		t.Fatal("Expected non-nil StaticPricingProvider")
	}
}

func TestLivePricingProvider_GetPrice_PreCached(t *testing.T) {
	ctx := context.Background()
	provider := &LivePricingProvider{
		awsClient:      nil,
		cache:          NewPricingCache(1 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	provider.cache.Set("aws", "gp3", "us-east-1", 0.05)
	price, err := provider.GetPrice(ctx, "aws", "gp3", "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if price != 0.05 {
		t.Errorf("Expected cached price 0.05, got %f", price)
	}
}

func TestGetIntraCloudEgressPrice_AllProviders(t *testing.T) {
	provider := &LivePricingProvider{}
	if price := provider.getIntraCloudEgressPrice("aws", "us-east-1", "eu-west-1"); price != 0.01 {
		t.Errorf("Expected AWS intra-cloud 0.01, got %f", price)
	}
	if price := provider.getIntraCloudEgressPrice("gcp", "us-central1", "europe-west1"); price != 0.01 {
		t.Errorf("Expected GCP intra-cloud 0.01, got %f", price)
	}
	if price := provider.getIntraCloudEgressPrice("azure", "eastus", "westeurope"); price != 0.02 {
		t.Errorf("Expected Azure intra-cloud 0.02, got %f", price)
	}
	if price := provider.getIntraCloudEgressPrice("oracle", "us-ashburn-1", "eu-frankfurt-1"); price != 0 {
		t.Errorf("Expected unknown provider intra-cloud 0, got %f", price)
	}
}

func TestGetInternetEgressPrice_AllProviders(t *testing.T) {
	provider := &LivePricingProvider{}
	if price := provider.getInternetEgressPrice("aws", "us-east-1"); price != 0.09 {
		t.Errorf("Expected AWS internet egress 0.09, got %f", price)
	}
	if price := provider.getInternetEgressPrice("gcp", "us-central1"); price != 0.08 {
		t.Errorf("Expected GCP internet egress 0.08, got %f", price)
	}
	if price := provider.getInternetEgressPrice("azure", "eastus"); price != 0.087 {
		t.Errorf("Expected Azure internet egress 0.087, got %f", price)
	}
	if price := provider.getInternetEgressPrice("oracle", "us-ashburn-1"); price != 0 {
		t.Errorf("Expected unknown provider internet egress 0, got %f", price)
	}
}

func TestPricingCache_ConcurrentReadWrite(t *testing.T) {
	cache := NewPricingCache(1 * time.Hour)
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func(id int) {
			cache.Set("aws", "gp3", "us-east-1", float64(id)*0.01)
			done <- true
		}(i)
		go func() {
			cache.Get("aws", "gp3", "us-east-1")
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
