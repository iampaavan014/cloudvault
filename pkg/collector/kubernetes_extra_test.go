package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCloudProvider_AzureFallbackRegion(t *testing.T) {
	labels := map[string]string{
		"kubernetes.azure.com/cluster":             "aks-cluster",
		"failure-domain.beta.kubernetes.io/region": "westus",
	}
	p, r := detectCloudProvider(labels)
	assert.Equal(t, "azure", p)
	assert.Equal(t, "westus", r)
}

func TestDetectCloudProvider_AWSFallbackRegion(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup":              "ng-1",
		"failure-domain.beta.kubernetes.io/region": "eu-west-1",
	}
	p, r := detectCloudProvider(labels)
	assert.Equal(t, "aws", p)
	assert.Equal(t, "eu-west-1", r)
}

func TestDetectCloudProvider_GCPFallbackRegion(t *testing.T) {
	labels := map[string]string{
		"cloud.google.com/gke-nodepool":            "pool-1",
		"failure-domain.beta.kubernetes.io/region": "europe-west1",
	}
	p, r := detectCloudProvider(labels)
	assert.Equal(t, "gcp", p)
	assert.Equal(t, "europe-west1", r)
}

func TestDetectCloudProvider_GCPProviderLabel(t *testing.T) {
	labels := map[string]string{
		"cloud.google.com/provider":     "gcp",
		"topology.kubernetes.io/region": "us-central1",
	}
	p, r := detectCloudProvider(labels)
	assert.Equal(t, "gcp", p)
	assert.Equal(t, "us-central1", r)
}

func TestDetectCloudProvider_AzureInstanceType(t *testing.T) {
	labels := map[string]string{
		"node.kubernetes.io/instance-type": "Standard_D4s_v3",
	}
	p, _ := detectCloudProvider(labels)
	assert.Equal(t, "azure", p)
}

func TestDetectCloudProvider_UnknownEmpty(t *testing.T) {
	p, r := detectCloudProvider(map[string]string{})
	assert.Equal(t, "unknown", p)
	assert.Equal(t, "unknown", r)
}

func TestDetectCloudProvider_TopologyRegionFallback(t *testing.T) {
	labels := map[string]string{
		"topology.kubernetes.io/region": "ap-southeast-1",
	}
	_, r := detectCloudProvider(labels)
	assert.Equal(t, "ap-southeast-1", r)
}

func TestNewKubernetesClient_NoConfig(t *testing.T) {
	_, err := NewKubernetesClient("/nonexistent/kubeconfig")
	assert.Error(t, err)
}

func TestPVCCollector_CollectAll_NilClient_Direct(t *testing.T) {
	c := &PVCCollector{client: nil}
	_, err := c.CollectAll(nil) //nolint:staticcheck
	assert.Error(t, err)
}
