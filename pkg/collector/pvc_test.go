package collector

import (
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/cost"
	"github.com/cloudvault-io/cloudvault/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewPVCCollector(t *testing.T) {
	// Test with nil prometheus client
	collector := NewPVCCollector(nil, nil)
	if collector == nil {
		t.Fatal("NewPVCCollector() returned nil")
	}
	if collector.client != nil {
		t.Error("Expected nil client")
	}
}

func TestCalculateBasicCost(t *testing.T) {
	tests := []struct {
		name         string
		sizeBytes    int64
		storageClass string
		provider     string
		expectedCost float64
		tolerance    float64
	}{
		{
			name:         "AWS gp3 100GB",
			sizeBytes:    100 * 1024 * 1024 * 1024,
			storageClass: "gp3",
			provider:     "aws",
			expectedCost: 8.00,
			tolerance:    0.001,
		},
		{
			name:         "GCP pd-standard 200GB",
			sizeBytes:    200 * 1024 * 1024 * 1024,
			storageClass: "pd-standard",
			provider:     "gcp",
			expectedCost: 8.00,
			tolerance:    0.001,
		},
	}

	calc := cost.NewCalculator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := types.PVCMetric{
				SizeBytes:    tt.sizeBytes,
				StorageClass: tt.storageClass,
			}
			costValue := calc.CalculatePVCCost(&metric, tt.provider)

			diff := costValue - tt.expectedCost
			if diff < 0 {
				diff = -diff
			}

			if diff > tt.tolerance {
				t.Errorf("CalculatePVCCost() = %v, want %v", costValue, tt.expectedCost)
			}
		})
	}
}

func TestInitializePVCMetric(t *testing.T) {
	collector := NewPVCCollector(nil, nil)

	quantity := resource.MustParse("100Gi")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-pvc",
			Namespace:         "pro",
			Labels:            map[string]string{"app": "db"},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
			StorageClassName: stringPtr("gp3"),
		},
	}

	clusterInfo := &types.ClusterInfo{
		ID:       "test-cluster",
		Provider: "aws",
		Region:   "us-east-1",
	}

	metric := collector.initializePVCMetric(pvc, clusterInfo)

	if metric.Name != "test-pvc" {
		t.Errorf("expected name test-pvc, got %s", metric.Name)
	}
	if metric.SizeBytes != 100*1024*1024*1024 {
		t.Errorf("expected size 100Gi, got %d", metric.SizeBytes)
	}
	if metric.StorageClass != "gp3" {
		t.Errorf("expected sc gp3, got %s", metric.StorageClass)
	}
}

func stringPtr(s string) *string {
	return &s
}
