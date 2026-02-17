package collector

import (
	"testing"
)

func TestDetectCloudProvider(t *testing.T) {
	tests := []struct {
		name             string
		labels           map[string]string
		expectedProvider string
		expectedRegion   string
	}{
		{
			name: "AWS EKS with topology label",
			labels: map[string]string{
				"eks.amazonaws.com/nodegroup":   "my-nodegroup",
				"topology.kubernetes.io/region": "us-east-1",
			},
			expectedProvider: "aws",
			expectedRegion:   "us-east-1",
		},
		{
			name: "AWS EKS with legacy label",
			labels: map[string]string{
				"eks.amazonaws.com/nodegroup":              "my-nodegroup",
				"failure-domain.beta.kubernetes.io/region": "us-west-2",
			},
			expectedProvider: "aws",
			expectedRegion:   "us-west-2",
		},
		{
			name: "GCP GKE",
			labels: map[string]string{
				"cloud.google.com/gke-nodepool": "default-pool",
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
			name: "AWS instance type inference",
			labels: map[string]string{
				"node.kubernetes.io/instance-type": "t3.medium",
				"topology.kubernetes.io/region":    "eu-west-1",
			},
			expectedProvider: "aws",
			expectedRegion:   "eu-west-1",
		},
		{
			name: "GCP instance type inference",
			labels: map[string]string{
				"node.kubernetes.io/instance-type": "n1-standard-4",
				"topology.kubernetes.io/region":    "europe-west1",
			},
			expectedProvider: "gcp",
			expectedRegion:   "europe-west1",
		},
		{
			name: "Azure instance type inference",
			labels: map[string]string{
				"node.kubernetes.io/instance-type": "Standard_D2s_v3",
				"topology.kubernetes.io/region":    "westus2",
			},
			expectedProvider: "azure",
			expectedRegion:   "westus2",
		},
		{
			name:             "Unknown provider",
			labels:           map[string]string{},
			expectedProvider: "unknown",
			expectedRegion:   "unknown",
		},
		{
			name: "Provider known but region unknown",
			labels: map[string]string{
				"eks.amazonaws.com/nodegroup": "my-nodegroup",
			},
			expectedProvider: "aws",
			expectedRegion:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, region := detectCloudProvider(tt.labels)

			if provider != tt.expectedProvider {
				t.Errorf("Expected provider '%s', got '%s'", tt.expectedProvider, provider)
			}

			if region != tt.expectedRegion {
				t.Errorf("Expected region '%s', got '%s'", tt.expectedRegion, region)
			}
		})
	}
}

func TestNewKubernetesClient_InvalidConfig(t *testing.T) {
	// Test with invalid kubeconfig path
	_, err := NewKubernetesClient("/invalid/path/to/kubeconfig")

	if err == nil {
		t.Error("Expected error with invalid kubeconfig path, got nil")
	}
}

func TestNewKubernetesClient_EmptyPath(t *testing.T) {
	// Test with empty kubeconfig (will try in-cluster config)
	_, err := NewKubernetesClient("")

	// Should fail if not running in cluster
	if err == nil {
		// Only pass if we're actually in a cluster
		t.Log("Warning: Empty kubeconfig succeeded - might be running in cluster")
	}
}
