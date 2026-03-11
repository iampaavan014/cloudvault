package cost

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── PricingCache ────────────────────────────────────────────────────────────

func TestPricingCache_SetAndGet(t *testing.T) {
	c := NewPricingCache(1 * time.Hour)
	c.Set("aws", "gp3", "us-east-1", 0.08)
	price, ok := c.Get("aws", "gp3", "us-east-1")
	require.True(t, ok)
	assert.InDelta(t, 0.08, price, 0.0001)
}

func TestPricingCache_Miss(t *testing.T) {
	c := NewPricingCache(1 * time.Hour)
	_, ok := c.Get("aws", "io2", "us-west-2")
	assert.False(t, ok)
}

func TestPricingCache_Expiry(t *testing.T) {
	c := NewPricingCache(1 * time.Millisecond)
	c.Set("aws", "gp3", "us-east-1", 0.08)
	time.Sleep(5 * time.Millisecond)
	_, ok := c.Get("aws", "gp3", "us-east-1")
	assert.False(t, ok, "entry should have expired")
}

func TestPricingCache_SetRawGetRaw(t *testing.T) {
	c := NewPricingCache(1 * time.Hour)
	c.SetRaw("egress-aws-us-east-1-internet", 0.09)
	price, ok := c.GetRaw("egress-aws-us-east-1-internet")
	require.True(t, ok)
	assert.InDelta(t, 0.09, price, 0.0001)
}

func TestPricingCache_MultipleEntries(t *testing.T) {
	c := NewPricingCache(1 * time.Hour)
	entries := map[string]float64{
		"aws-gp3-us-east-1":   0.08,
		"aws-io2-us-east-1":   0.125,
		"gcp-pd-standard-us":  0.04,
		"azure-Premium_LRS-e": 0.12,
	}
	for k, v := range entries {
		c.SetRaw(k, v)
	}
	for k, expected := range entries {
		got, ok := c.GetRaw(k)
		require.True(t, ok, "key %s should exist", k)
		assert.InDelta(t, expected, got, 0.0001)
	}
}

// ── StaticPricingProvider ───────────────────────────────────────────────────

func TestStaticPricingProvider_AWS(t *testing.T) {
	s := NewStaticPricingProvider()
	cases := []struct {
		sc       string
		expected float64
	}{
		{"gp3", 0.08},
		{"gp2", 0.10},
		{"io2", 0.125},
		{"io1", 0.125},
		{"st1", 0.045},
		{"sc1", 0.015},
		{"unknown-class", 0.08},
	}
	for _, tc := range cases {
		t.Run("aws/"+tc.sc, func(t *testing.T) {
			price := s.GetStoragePrice("aws", tc.sc, "us-east-1")
			assert.InDelta(t, tc.expected, price, 0.0001)
		})
	}
}

func TestStaticPricingProvider_GCP(t *testing.T) {
	s := NewStaticPricingProvider()
	assert.InDelta(t, 0.04, s.GetStoragePrice("gcp", "pd-standard", "us-central1"), 0.0001)
	assert.InDelta(t, 0.17, s.GetStoragePrice("gcp", "pd-ssd", "us-central1"), 0.0001)
	assert.InDelta(t, 0.10, s.GetStoragePrice("gcp", "pd-balanced", "us-central1"), 0.0001)
	assert.InDelta(t, 0.04, s.GetStoragePrice("gcp", "nonexistent", "us-central1"), 0.0001)
}

func TestStaticPricingProvider_Azure(t *testing.T) {
	s := NewStaticPricingProvider()
	assert.InDelta(t, 0.12, s.GetStoragePrice("azure", "Premium_LRS", "eastus"), 0.0001)
	assert.InDelta(t, 0.10, s.GetStoragePrice("azure", "StandardSSD_LRS", "eastus"), 0.0001)
	assert.InDelta(t, 0.05, s.GetStoragePrice("azure", "Standard_LRS", "eastus"), 0.0001)
	assert.InDelta(t, 0.05, s.GetStoragePrice("azure", "unknown", "eastus"), 0.0001)
}

func TestStaticPricingProvider_UnknownProvider(t *testing.T) {
	s := NewStaticPricingProvider()
	price := s.GetStoragePrice("unknown-cloud", "gp3", "us-east-1")
	assert.InDelta(t, 0.10, price, 0.0001)
}

func TestStaticPricingProvider_GetPrice_Provisioned(t *testing.T) {
	s := NewStaticPricingProvider()

	p := s.GetPrice("aws", "io2", "us-east-1")
	assert.True(t, p.Provisioned)
	assert.InDelta(t, 0.005, p.PerIOPS, 0.0001)

	p2 := s.GetPrice("aws", "gp3", "us-east-1")
	assert.False(t, p2.Provisioned)
	assert.InDelta(t, 0.0, p2.PerIOPS, 0.0001)

	p3 := s.GetPrice("gcp", "pd-ssd", "us-central1")
	assert.True(t, p3.Provisioned)

	p4 := s.GetPrice("azure", "Premium_LRS", "eastus")
	assert.True(t, p4.Provisioned)
}

// ── Mapping helpers ──────────────────────────────────────────────────────────

func TestMapStorageClassToAWSVolumeType(t *testing.T) {
	cases := map[string]string{
		"gp2":         "gp2",
		"gp3":         "gp3",
		"io1":         "io1",
		"io2":         "io2",
		"st1":         "st1",
		"sc1":         "sc1",
		"standard":    "standard",
		"aws-ebs-gp3": "gp3",
		"aws-ebs-io2": "io2",
		"unknown":     "gp3",
	}
	for input, expected := range cases {
		assert.Equal(t, expected, mapStorageClassToAWSVolumeType(input), "input: %s", input)
	}
}

func TestMapRegionToAWSLocation(t *testing.T) {
	assert.Equal(t, "US East (N. Virginia)", mapRegionToAWSLocation("us-east-1"))
	assert.Equal(t, "US West (Oregon)", mapRegionToAWSLocation("us-west-2"))
	assert.Equal(t, "EU (Ireland)", mapRegionToAWSLocation("eu-west-1"))
	assert.Equal(t, "Asia Pacific (Tokyo)", mapRegionToAWSLocation("ap-northeast-1"))
	assert.Equal(t, "US East (N. Virginia)", mapRegionToAWSLocation("xx-unknown-1"))
}

func TestMapStorageClassToGCPSKU(t *testing.T) {
	assert.Equal(t, "Standard Disk", mapStorageClassToGCPSKU("pd-standard"))
	assert.Equal(t, "SSD backed", mapStorageClassToGCPSKU("pd-ssd"))
	assert.Equal(t, "Balanced", mapStorageClassToGCPSKU("pd-balanced"))
	assert.Equal(t, "Standard Disk", mapStorageClassToGCPSKU("unknown"))
}

func TestMapStorageClassToAzureProduct(t *testing.T) {
	assert.Equal(t, "Premium SSD Managed Disks", mapStorageClassToAzureProduct("Premium_LRS"))
	assert.Equal(t, "Standard SSD Managed Disks", mapStorageClassToAzureProduct("StandardSSD_LRS"))
	assert.Equal(t, "Standard HDD Managed Disks", mapStorageClassToAzureProduct("Standard_LRS"))
	assert.Equal(t, "Standard SSD Managed Disks", mapStorageClassToAzureProduct("unknown"))
}

func TestMapStorageClassToAzureSKU(t *testing.T) {
	assert.Equal(t, "P10 LRS", mapStorageClassToAzureSKU("Premium_LRS"))
	assert.Equal(t, "E10 LRS", mapStorageClassToAzureSKU("StandardSSD_LRS"))
	assert.Equal(t, "S10 LRS", mapStorageClassToAzureSKU("Standard_LRS"))
	assert.Equal(t, "E10 LRS", mapStorageClassToAzureSKU("unknown"))
}

func TestContainsCaseInsensitive(t *testing.T) {
	assert.True(t, containsCaseInsensitive("Standard Disk Storage", "standard"))
	assert.True(t, containsCaseInsensitive("SSD backed PD", "SSD"))
	assert.False(t, containsCaseInsensitive("Standard Disk", "premium"))
}

// ── LivePricingProvider — egress methods ────────────────────────────────────

func TestLivePricingProvider_GetEgressPrice_SameRegion(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetEgressPrice(nil, "aws", "us-east-1", "aws", "us-east-1")
	require.NoError(t, err)
	assert.Equal(t, 0.0, price)
}

func TestLivePricingProvider_GetEgressPrice_IntraCloud(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetEgressPrice(nil, "aws", "us-east-1", "aws", "us-west-2")
	require.NoError(t, err)
	assert.InDelta(t, 0.01, price, 0.0001)

	// cache hit
	price2, err := p.GetEgressPrice(nil, "aws", "us-east-1", "aws", "us-west-2")
	require.NoError(t, err)
	assert.InDelta(t, price, price2, 0.0001)
}

func TestLivePricingProvider_GetEgressPrice_CrossCloud(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetEgressPrice(nil, "aws", "us-east-1", "gcp", "us-central1")
	require.NoError(t, err)
	assert.InDelta(t, 0.09, price, 0.0001)

	// cache hit
	price2, err := p.GetEgressPrice(nil, "aws", "us-east-1", "gcp", "us-central1")
	require.NoError(t, err)
	assert.Equal(t, price, price2)
}

func TestLivePricingProvider_GetEgressPrice_GCPIntra(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetEgressPrice(nil, "gcp", "us-central1", "gcp", "europe-west1")
	require.NoError(t, err)
	assert.InDelta(t, 0.01, price, 0.0001)
}

func TestLivePricingProvider_GetEgressPrice_AzureIntra(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetEgressPrice(nil, "azure", "eastus", "azure", "westus")
	require.NoError(t, err)
	assert.InDelta(t, 0.02, price, 0.0001)
}

func TestLivePricingProvider_GetPrice_StaticFallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetPrice(nil, "aws", "gp3", "us-east-1")
	require.NoError(t, err)
	assert.InDelta(t, 0.08, price, 0.0001)

	// cache hit
	price2, err := p.GetPrice(nil, "aws", "gp3", "us-east-1")
	require.NoError(t, err)
	assert.InDelta(t, price, price2, 0.0001)
}

func TestLivePricingProvider_GetPrice_GCP_Fallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetPrice(nil, "gcp", "pd-standard", "us-central1")
	require.NoError(t, err)
	assert.InDelta(t, 0.04, price, 0.0001)
}

func TestLivePricingProvider_GetPrice_Azure_Fallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetPrice(nil, "azure", "Premium_LRS", "eastus")
	require.NoError(t, err)
	assert.InDelta(t, 0.12, price, 0.0001)
}

func TestLivePricingProvider_GetPrice_UnknownProvider(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}
	price, err := p.GetPrice(nil, "unknown-cloud", "gp3", "us-east-1")
	require.NoError(t, err)
	assert.InDelta(t, 0.10, price, 0.0001)
}

// ── Calculator extra coverage ────────────────────────────────────────────────

func TestCalculator_FormatCostPerMonth(t *testing.T) {
	assert.Equal(t, "$10.50/mo", FormatCostPerMonth(10.50))
}

func TestCalculator_FormatCostPerYear(t *testing.T) {
	assert.Equal(t, "$120.00/yr", FormatCostPerYear(10.00))
}

func TestCalculator_GetPricing(t *testing.T) {
	calc := NewCalculator()
	p := calc.GetPricing("aws", "io2")
	require.NotNil(t, p)
	assert.True(t, p.Provisioned)
	assert.InDelta(t, 0.125, p.PerGBMonth, 0.001)
}
