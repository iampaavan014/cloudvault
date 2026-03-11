package cost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// azureRetailResponse builds a minimal Azure Retail Prices API JSON response
func azureRetailResponse(price float64, unit string) string {
	data, _ := json.Marshal(map[string]interface{}{
		"Items": []map[string]interface{}{
			{"retailPrice": price, "unitOfMeasure": unit},
		},
	})
	return string(data)
}

// ── parseAWSPricingJSON ───────────────────────────────────────────────────────

func TestParseAWSPricingJSON_Valid(t *testing.T) {
	p := &LivePricingProvider{}
	priceJSON := `{
		"terms": {
			"OnDemand": {
				"ABC": {
					"priceDimensions": {
						"XYZ": {
							"pricePerUnit": {"USD": "0.08"}
						}
					}
				}
			}
		}
	}`
	price, err := p.parseAWSPricingJSON(priceJSON)
	require.NoError(t, err)
	assert.InDelta(t, 0.08, price, 0.001)
}

func TestParseAWSPricingJSON_InvalidJSON(t *testing.T) {
	p := &LivePricingProvider{}
	_, err := p.parseAWSPricingJSON("not-json")
	assert.Error(t, err)
}

func TestParseAWSPricingJSON_MissingTerms(t *testing.T) {
	p := &LivePricingProvider{}
	_, err := p.parseAWSPricingJSON(`{"product": {}}`)
	assert.Error(t, err)
}

func TestParseAWSPricingJSON_MissingOnDemand(t *testing.T) {
	p := &LivePricingProvider{}
	_, err := p.parseAWSPricingJSON(`{"terms": {"Reserved": {}}}`)
	assert.Error(t, err)
}

// ── getAzurePrice via mock HTTP ───────────────────────────────────────────────

func TestGetAzurePrice_NilClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(azureRetailResponse(0.10, "1 GB/Month")))
	}))
	defer srv.Close()

	// azureClient is nil → falls through to static fallback
	p := &LivePricingProvider{
		cache:          NewPricingCache(0),
		staticFallback: NewStaticPricingProvider(),
		azureClient:    nil,
	}
	price, err := p.GetPrice(context.Background(), "azure", "Premium_LRS", "eastus")
	require.NoError(t, err)
	assert.Greater(t, price, 0.0)
}

// ── getInternetEgressPrice all branches ──────────────────────────────────────

func TestGetInternetEgressPrice_AllProviders(t *testing.T) {
	p := &LivePricingProvider{}
	assert.InDelta(t, 0.09, p.getInternetEgressPrice("aws", "us-east-1"), 0.001)
	assert.InDelta(t, 0.08, p.getInternetEgressPrice("gcp", "us-central1"), 0.001)
	assert.InDelta(t, 0.087, p.getInternetEgressPrice("azure", "eastus"), 0.001)
	assert.Equal(t, 0.0, p.getInternetEgressPrice("unknown", "us-east-1"))
}

// ── getIntraCloudEgressPrice all branches ─────────────────────────────────────

func TestGetIntraCloudEgressPrice_AllProviders(t *testing.T) {
	p := &LivePricingProvider{}
	assert.InDelta(t, 0.01, p.getIntraCloudEgressPrice("aws", "us-east-1", "us-west-2"), 0.001)
	assert.InDelta(t, 0.01, p.getIntraCloudEgressPrice("gcp", "us-central1", "us-east1"), 0.001)
	assert.InDelta(t, 0.02, p.getIntraCloudEgressPrice("azure", "eastus", "westus"), 0.001)
	assert.Equal(t, 0.0, p.getIntraCloudEgressPrice("unknown", "a", "b"))
}

// ── GetPrice with nil sub-clients → static fallback ──────────────────────────

func TestLivePricingProvider_GetPrice_CacheHit(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(1<<63 - 1),
		staticFallback: NewStaticPricingProvider(),
	}
	p.cache.Set("aws", "gp3", "us-east-1", 0.10)
	price, err := p.GetPrice(context.Background(), "aws", "gp3", "us-east-1")
	require.NoError(t, err)
	assert.InDelta(t, 0.10, price, 0.001)
}

func TestLivePricingProvider_GetPrice_NilAWSClient_Fallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(0),
		staticFallback: NewStaticPricingProvider(),
		awsClient:      nil,
	}
	price, err := p.GetPrice(context.Background(), "aws", "gp3", "us-east-1")
	require.NoError(t, err)
	assert.Greater(t, price, 0.0) // static fallback should return non-zero
}

func TestLivePricingProvider_GetPrice_NilGCPClient_Fallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(0),
		staticFallback: NewStaticPricingProvider(),
		gcpClient:      nil,
	}
	price, err := p.GetPrice(context.Background(), "gcp", "pd-ssd", "us-central1")
	require.NoError(t, err)
	assert.Greater(t, price, 0.0)
}

func TestLivePricingProvider_GetPrice_NilAzureClient_Fallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(0),
		staticFallback: NewStaticPricingProvider(),
		azureClient:    nil,
	}
	price, err := p.GetPrice(context.Background(), "azure", "Premium_LRS", "eastus")
	require.NoError(t, err)
	assert.Greater(t, price, 0.0)
}

func TestLivePricingProvider_GetPrice_UnknownProvider_Fallback(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(0),
		staticFallback: NewStaticPricingProvider(),
	}
	// Unknown provider hits the default case → error → static fallback returns 0.10 (final default)
	price, err := p.GetPrice(context.Background(), "bogus", "sc1", "us-east-1")
	require.NoError(t, err)
	assert.InDelta(t, 0.10, price, 0.001)
}

// ── warmupCache (smoke — just ensure it doesn't panic with nil clients) ───────

func TestLivePricingProvider_WarmupCache_NilClients(t *testing.T) {
	p := &LivePricingProvider{
		cache:          NewPricingCache(1<<63 - 1),
		staticFallback: NewStaticPricingProvider(),
	}
	// Run synchronously to exercise the code path
	p.warmupCache(context.Background())
}
