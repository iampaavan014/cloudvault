package cost

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPricingCache(t *testing.T) {
	cache := NewPricingCache(1 * time.Second)

	// Test Set and Get
	cache.Set("aws", "gp3", "us-east-1", 0.08)
	price, ok := cache.Get("aws", "gp3", "us-east-1")
	assert.True(t, ok)
	assert.Equal(t, 0.08, price)

	// Test Expiry
	time.Sleep(1100 * time.Millisecond)
	_, ok = cache.Get("aws", "gp3", "us-east-1")
	assert.False(t, ok)
}

func TestMapStorageClassToAWSVolumeType(t *testing.T) {
	assert.Equal(t, "gp3", mapStorageClassToAWSVolumeType("gp3"))
	assert.Equal(t, "io2", mapStorageClassToAWSVolumeType("io2"))
	assert.Equal(t, "gp3", mapStorageClassToAWSVolumeType("unknown"))
}

func TestMapRegionToAWSLocation(t *testing.T) {
	assert.Equal(t, "US East (N. Virginia)", mapRegionToAWSLocation("us-east-1"))
	assert.Equal(t, "EU (Ireland)", mapRegionToAWSLocation("eu-west-1"))
}

func TestStaticPricingProvider_GetPrice(t *testing.T) {
	provider := NewStaticPricingProvider()

	p1 := provider.GetPrice("aws", "gp3", "us-east-1")
	assert.Equal(t, 0.08, p1.PerGBMonth)
	assert.False(t, p1.Provisioned)

	p2 := provider.GetPrice("aws", "io2", "us-east-1")
	assert.Equal(t, 0.125, p2.PerGBMonth)
	assert.True(t, p2.Provisioned)
	assert.Equal(t, 0.005, p2.PerIOPS)
}

func TestLivePricingProvider_GetPrice_Fallback(t *testing.T) {
	// Test that it falls back to static when client is nil
	provider := &LivePricingProvider{
		cache:          NewPricingCache(24 * time.Hour),
		staticFallback: NewStaticPricingProvider(),
	}

	price, err := provider.GetPrice(context.Background(), "aws", "gp3", "us-east-1")
	assert.NoError(t, err)
	assert.Equal(t, 0.08, price)
}
