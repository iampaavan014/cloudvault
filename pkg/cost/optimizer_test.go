package cost

import (
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

func TestOptimizer_GenerateRecommendations(t *testing.T) {
	opt := NewOptimizer()

	now := time.Now()
	metrics := []types.PVCMetric{
		{
			Name:           "zombie-pvc",
			Namespace:      "default",
			SizeBytes:      100 * 1024 * 1024 * 1024,
			StorageClass:   "gp3",
			CreatedAt:      now.Add(-60 * 24 * time.Hour),
			LastAccessedAt: now.Add(-40 * 24 * time.Hour),
			MonthlyCost:    8.0,
		},
		{
			Name:         "oversized-pvc",
			Namespace:    "prod",
			SizeBytes:    200 * 1024 * 1024 * 1024,
			UsedBytes:    10 * 1024 * 1024 * 1024, // 5% utilized
			StorageClass: "gp3",
			CreatedAt:    now.Add(-10 * 24 * time.Hour),
			MonthlyCost:  16.0,
		},
	}

	recs := opt.GenerateRecommendations(metrics, "aws")

	foundZombie := false
	foundResize := false

	for _, rec := range recs {
		if rec.Type == "delete_zombie" {
			foundZombie = true
		}
		if rec.Type == "resize" {
			foundResize = true
		}
	}

	if !foundZombie {
		t.Error("Expected to find zombie recommendation")
	}
	if !foundResize {
		t.Error("Expected to find resize recommendation")
	}
}

func TestOptimizer_CheckStorageClassOptimization(t *testing.T) {
	opt := NewOptimizer()

	tests := []struct {
		name         string
		storageClass string
		readIOPS     float64
		expectedRec  bool
		targetClass  string
	}{
		{
			name:         "aws-gp3-low-iops",
			storageClass: "gp3",
			readIOPS:     100,
			expectedRec:  true,
			targetClass:  "sc1",
		},
		{
			name:         "aws-io1-low-iops",
			storageClass: "io1",
			readIOPS:     500,
			expectedRec:  true,
			targetClass:  "gp3",
		},
		{
			name:         "aws-gp3-high-iops-no-rec",
			storageClass: "gp3",
			readIOPS:     5000,
			expectedRec:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := types.PVCMetric{
				Name:         "test-pvc",
				StorageClass: tt.storageClass,
				ReadIOPS:     tt.readIOPS,
				SizeBytes:    100 * 1024 * 1024 * 1024,
			}
			rec := opt.checkStorageClassOptimization(&m, "aws")
			if tt.expectedRec {
				if rec == nil {
					t.Errorf("Expected recommendation, got nil")
				} else if rec.RecommendedState != tt.targetClass {
					t.Errorf("Expected target class %s, got %s", tt.targetClass, rec.RecommendedState)
				}
			} else {
				if rec != nil {
					t.Errorf("Expected nil recommendation, got %v", rec)
				}
			}
		})
	}
}
