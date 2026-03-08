package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCloudProvider(t *testing.T) {
	tests := []struct {
		name             string
		labels           map[string]string
		expectedProvider string
		expectedRegion   string
	}{
		{
			name: "AWS EKS",
			labels: map[string]string{
				"eks.amazonaws.com/nodegroup":   "my-nodes",
				"topology.kubernetes.io/region": "us-east-1",
			},
			expectedProvider: "aws",
			expectedRegion:   "us-east-1",
		},
		{
			name: "GCP GKE",
			labels: map[string]string{
				"cloud.google.com/gke-nodepool": "my-pool",
				"topology.kubernetes.io/region": "us-central1",
			},
			expectedProvider: "gcp",
			expectedRegion:   "us-central1",
		},
		{
			name: "Azure AKS",
			labels: map[string]string{
				"kubernetes.azure.com/cluster":  "my-cluster",
				"topology.kubernetes.io/region": "eastus",
			},
			expectedProvider: "azure",
			expectedRegion:   "eastus",
		},
		{
			name: "Inferred AWS",
			labels: map[string]string{
				"node.kubernetes.io/instance-type": "m5.large",
			},
			expectedProvider: "aws",
			expectedRegion:   "unknown",
		},
		{
			name: "Inferred GCP",
			labels: map[string]string{
				"node.kubernetes.io/instance-type": "n1-standard-1",
			},
			expectedProvider: "gcp",
			expectedRegion:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, r := detectCloudProvider(tt.labels)
			assert.Equal(t, tt.expectedProvider, p)
			assert.Equal(t, tt.expectedRegion, r)
		})
	}
}

func TestKubernetesClient_Getters(t *testing.T) {
	k := &KubernetesClient{}
	assert.Nil(t, k.GetClientset())
	assert.Nil(t, k.GetConfig())
	assert.Nil(t, k.GetDynamicClient())
}
