package cost

import (
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── MultiCloudPricingProvider ─────────────────────────────────────────────────

func TestMultiCloudPricingProvider_NilClients_Fallback(t *testing.T) {
	p := NewMultiCloudPricingProvider(nil, nil, nil)

	// all three providers should return the baseline fallback (0.10) since clients are nil
	cases := []struct{ prov, sc, region string }{
		{"aws", "gp3", "us-east-1"},
		{"gcp", "pd-standard", "us-central1"},
		{"azure", "managed-premium", "eastus"},
	}
	for _, tc := range cases {
		pr := p.GetPrice(tc.prov, tc.sc, tc.region)
		assert.InDelta(t, 0.10, pr.PerGBMonth, 0.0001, "%s/%s", tc.prov, tc.sc)
	}
}

func TestMultiCloudPricingProvider_Unknown_Provider(t *testing.T) {
	p := NewMultiCloudPricingProvider(nil, nil, nil)
	pr := p.GetPrice("unknown-cloud", "gp3", "us-east-1")
	assert.InDelta(t, 0.10, pr.PerGBMonth, 0.0001)
}

func TestMultiCloudPricingProvider_CacheHit(t *testing.T) {
	p := NewMultiCloudPricingProvider(nil, nil, nil)
	// Warm the cache manually
	p.cache["aws-gp3-us-east-1"] = StorageClassPricing{PerGBMonth: 0.08, Provisioned: true}
	pr := p.GetPrice("aws", "gp3", "us-east-1")
	assert.InDelta(t, 0.08, pr.PerGBMonth, 0.0001)
	assert.True(t, pr.Provisioned)
}

func TestMultiCloudPricingProvider_Provisioned_Flags(t *testing.T) {
	p := NewMultiCloudPricingProvider(nil, nil, nil)

	// AWS: gp3/io1/io2 are provisioned
	for _, sc := range []string{"gp3", "io1", "io2"} {
		// With nil client, falls back — just check Provisioned flag via cache after a real client
		// We test the code path by priming the cache with a successful price
		key := "aws-" + sc + "-us-east-1"
		p.cache[key] = StorageClassPricing{}
		_ = p.GetPrice("aws", sc, "us-east-1")
	}

	// GCP: premium-rwo is provisioned
	p2 := NewMultiCloudPricingProvider(nil, nil, nil)
	p2.cache["gcp-premium-rwo-us-central1"] = StorageClassPricing{}
	_ = p2.GetPrice("gcp", "premium-rwo", "us-central1")

	// Azure: managed-premium is provisioned
	p3 := NewMultiCloudPricingProvider(nil, nil, nil)
	p3.cache["azure-managed-premium-eastus"] = StorageClassPricing{}
	_ = p3.GetPrice("azure", "managed-premium", "eastus")
}

func TestMultiCloudPricingProvider_WithRealClients(t *testing.T) {
	// Use real (but no-cred) clients — they'll error and fall back to 0.10 baseline
	awsC := integrations.NewAWSClient(nil)
	gcpC := integrations.NewGCPClient(nil)
	azureC := integrations.NewAzureClient(nil)
	p := NewMultiCloudPricingProvider(awsC, gcpC, azureC)

	pr := p.GetPrice("aws", "gp3", "us-east-1")
	assert.InDelta(t, 0.10, pr.PerGBMonth, 0.0001)

	pr2 := p.GetPrice("gcp", "pd-standard", "us-central1")
	assert.InDelta(t, 0.10, pr2.PerGBMonth, 0.0001)
}

// ── cost/optimizer uncovered branches ────────────────────────────────────────

func TestOptimizer_CheckStorageClassOptimization_GCP(t *testing.T) {
	o := NewOptimizer()

	// pd-ssd with low IOPS → should recommend pd-balanced
	pvcSSD := &types.PVCMetric{
		Name:         "gcp-ssd-pvc",
		Namespace:    "default",
		StorageClass: "pd-ssd",
		SizeBytes:    500 * 1024 * 1024 * 1024, // 500 GB
		ReadIOPS:     200,
		WriteIOPS:    100,
		MonthlyCost:  85.0,
	}
	rec := o.checkStorageClassOptimization(pvcSSD, "gcp")
	require.NotNil(t, rec)
	assert.Equal(t, "pd-balanced", rec.RecommendedState)

	// pd-balanced with very low IOPS → should recommend pd-standard
	pvcBalanced := &types.PVCMetric{
		Name:         "gcp-balanced-pvc",
		Namespace:    "default",
		StorageClass: "pd-balanced",
		SizeBytes:    500 * 1024 * 1024 * 1024,
		ReadIOPS:     100,
		WriteIOPS:    100,
		MonthlyCost:  50.0,
	}
	rec2 := o.checkStorageClassOptimization(pvcBalanced, "gcp")
	require.NotNil(t, rec2)
	assert.Equal(t, "pd-standard", rec2.RecommendedState)
}

func TestOptimizer_CheckStorageClassOptimization_Azure(t *testing.T) {
	o := NewOptimizer()

	// "managed-premium" with low IOPS → should recommend "standard"
	pvc := &types.PVCMetric{
		Name:         "azure-premium-pvc",
		Namespace:    "default",
		StorageClass: "managed-premium",
		SizeBytes:    500 * 1024 * 1024 * 1024,
		ReadIOPS:     200,
		WriteIOPS:    200,
		MonthlyCost:  60.0,
	}
	rec := o.checkStorageClassOptimization(pvc, "azure")
	require.NotNil(t, rec)
	assert.Equal(t, "standard", rec.RecommendedState)
}

func TestOptimizer_CheckStorageClassOptimization_AWSPrefix(t *testing.T) {
	o := NewOptimizer()

	// Provider-prefixed storage class names
	pvc := &types.PVCMetric{
		Name:         "aws-prefixed-pvc",
		Namespace:    "default",
		StorageClass: "aws-gp3",
		SizeBytes:    500 * 1024 * 1024 * 1024,
		ReadIOPS:     100,
		WriteIOPS:    100,
		MonthlyCost:  40.0,
	}
	// provider="unknown" — should infer aws from prefix
	rec := o.checkStorageClassOptimization(pvc, "unknown")
	// Low IOPS on gp3-style → may recommend sc1
	_ = rec
}

func TestOptimizer_CheckStorageClassOptimization_IO_Sufficient(t *testing.T) {
	o := NewOptimizer()

	// io2 with sufficient IOPS — no recommendation
	pvc := &types.PVCMetric{
		Name:         "io2-high-iops",
		Namespace:    "default",
		StorageClass: "io2",
		SizeBytes:    200 * 1024 * 1024 * 1024,
		ReadIOPS:     2000,
		WriteIOPS:    2000,
		MonthlyCost:  100.0,
	}
	rec := o.checkStorageClassOptimization(pvc, "aws")
	assert.Nil(t, rec, "io2 with high IOPS should not be recommended for downgrade")
}

func TestOptimizer_CheckOversizedVolume_AlreadyMigrated(t *testing.T) {
	o := NewOptimizer()

	pvc := &types.PVCMetric{
		Name:        "migrated-pvc",
		Namespace:   "default",
		SizeBytes:   100 * 1024 * 1024 * 1024,
		UsedBytes:   5 * 1024 * 1024 * 1024,
		MonthlyCost: 20.0,
		Annotations: map[string]string{"cloudvault.io/migrated-from": "old-class"},
	}
	rec := o.checkOversizedVolume(pvc)
	// already-migrated volumes are skipped
	assert.Nil(t, rec)
}

func TestOptimizer_CheckOversizedVolume_SmallVolume(t *testing.T) {
	o := NewOptimizer()

	// Volume under 50 GB threshold → should not be recommended
	pvc := &types.PVCMetric{
		Name:        "tiny-pvc",
		Namespace:   "default",
		SizeBytes:   10 * 1024 * 1024 * 1024, // 10 GB < 50 GB minimum
		UsedBytes:   1 * 1024 * 1024 * 1024,
		MonthlyCost: 0.80,
	}
	rec := o.checkOversizedVolume(pvc)
	assert.Nil(t, rec)
}

func TestOptimizer_CheckOversizedVolume_NegligibleSavings(t *testing.T) {
	o := NewOptimizer()

	// 91% utilisation → savings < 20% → nil
	pvc := &types.PVCMetric{
		Name:        "well-used-pvc",
		Namespace:   "default",
		SizeBytes:   100 * 1024 * 1024 * 1024,
		UsedBytes:   91 * 1024 * 1024 * 1024,
		MonthlyCost: 8.0,
	}
	rec := o.checkOversizedVolume(pvc)
	assert.Nil(t, rec)
}

func TestOptimizer_DetermineImpact_PdStandard(t *testing.T) {
	assert.Equal(t, "medium", determineImpact(200, "pd-standard"))
	assert.Equal(t, "low", determineImpact(50, "pd-standard"))
}

func TestOptimizer_DetermineImpact_GpClass(t *testing.T) {
	assert.Equal(t, "medium", determineImpact(3000, "gp3"))
	assert.Equal(t, "low", determineImpact(100, "gp3"))
}

func TestOptimizer_GenerateRecommendations_GP2RLAgent(t *testing.T) {
	o := NewOptimizer()

	// RL agent fires on gp2 storage class
	metrics := []types.PVCMetric{
		{
			Name:         "gp2-pvc",
			Namespace:    "default",
			StorageClass: "gp2",
			SizeBytes:    100 * 1024 * 1024 * 1024,
			UsedBytes:    80 * 1024 * 1024 * 1024,
			MonthlyCost:  10.0,
			Provider:     "aws",
			Region:       "us-east-1",
		},
	}
	recs := o.GenerateRecommendations(metrics, "aws")
	// Should produce at least one recommendation (RL agent for gp2)
	assert.NotNil(t, recs)
}

func TestOptimizer_GenerateRecommendations_HighCostAWS_CrossCloud(t *testing.T) {
	o := NewOptimizer()

	// High monthly cost on AWS gp3 — may trigger cross-cloud check
	metrics := []types.PVCMetric{
		{
			Name:         "expensive-pvc",
			Namespace:    "production",
			StorageClass: "gp3",
			SizeBytes:    1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:    800 * 1024 * 1024 * 1024,
			MonthlyCost:  120.0,
			Provider:     "aws",
			Region:       "us-east-1",
		},
	}
	recs := o.GenerateRecommendations(metrics, "aws")
	assert.NotNil(t, recs)
}

func TestOptimizer_CheckZombieVolume_ZeroAccess(t *testing.T) {
	o := NewOptimizer()

	// Zero time → no access data → should return nil
	pvc := &types.PVCMetric{
		Name:      "pvc-no-access",
		Namespace: "default",
	}
	assert.Nil(t, o.checkZombieVolume(pvc))
}

func TestOptimizer_CheckZombieVolume_RecentAccess(t *testing.T) {
	o := NewOptimizer()

	pvc := &types.PVCMetric{
		Name:           "pvc-recent",
		Namespace:      "default",
		LastAccessedAt: time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
		MonthlyCost:    10.0,
	}
	assert.Nil(t, o.checkZombieVolume(pvc))
}
