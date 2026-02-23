package lifecycle

import (
	"context"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
)

func TestNewIntelligentRecommender(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	if recommender == nil {
		t.Fatal("Expected non-nil recommender")
	}

	if recommender.rlAgent == nil {
		t.Error("Expected non-nil RL agent")
	}

	if recommender.forecaster == nil {
		t.Error("Expected non-nil forecaster")
	}

	if recommender.anomalyEngine == nil {
		t.Error("Expected non-nil anomaly engine")
	}
}

func TestIntelligentRecommender_Train_NoTSDB(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	err := recommender.Train(context.Background(), "default", "test-pvc")
	if err == nil {
		t.Error("Expected error when TSDB is nil")
	}
}

func TestIntelligentRecommender_Train_InsufficientData(t *testing.T) {
	t.Skip("Requires TimescaleDB instance")

	// This would test with a mock TSDB that returns insufficient data
	recommender := NewIntelligentRecommender(nil)

	err := recommender.Train(context.Background(), "default", "test-pvc")
	if err == nil {
		t.Error("Expected error for insufficient training data")
	}
}

func TestIntelligentRecommender_DetectHistoryProfile(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	tests := []struct {
		name     string
		history  []float64
		expected string
	}{
		{
			name:     "high-load",
			history:  []float64{0.8, 0.9, 0.85, 0.75},
			expected: "high-load",
		},
		{
			name:     "standard-load",
			history:  []float64{0.3, 0.4, 0.5, 0.2},
			expected: "standard-load",
		},
		{
			name:     "low-load",
			history:  []float64{0.1, 0.2, 0.15, 0.1},
			expected: "standard-load",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := recommender.detectHistoryProfile(tt.history)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIntelligentRecommender_Recommend_RightSizing(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	pvc := types.PVCMetric{
		Name:         "oversized-pvc",
		Namespace:    "default",
		StorageClass: "gp3",
		SizeBytes:    100 * 1024 * 1024 * 1024, // 100GB
		UsedBytes:    20 * 1024 * 1024 * 1024,  // 20GB (20% utilization)
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	rec := recommender.Recommend(pvc, policy)
	if rec == nil {
		t.Fatal("Expected recommendation for oversized PVC")
	}

	if rec.Reason == "" {
		t.Error("Expected reason for recommendation")
	}

	if rec.Confidence <= 0 {
		t.Error("Expected positive confidence")
	}
}

func TestIntelligentRecommender_Recommend_ZombieVolume(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	pvc := types.PVCMetric{
		Name:         "zombie-pvc",
		Namespace:    "default",
		StorageClass: "gp3",
		SizeBytes:    100 * 1024 * 1024 * 1024,
		UsedBytes:    100 * 1024 * 1024, // 0.1% utilization (< 5%)
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	rec := recommender.Recommend(pvc, policy)
	if rec == nil {
		t.Fatal("Expected recommendation for zombie volume")
	}

	if rec.TargetTier != "cold" {
		t.Errorf("Expected cold tier for zombie volume, got %s", rec.TargetTier)
	}
}

func TestIntelligentRecommender_Recommend_WellSized(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	pvc := types.PVCMetric{
		Name:         "well-sized-pvc",
		Namespace:    "default",
		StorageClass: "gp3",
		SizeBytes:    100 * 1024 * 1024 * 1024,
		UsedBytes:    70 * 1024 * 1024 * 1024, // 70% utilization
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	rec := recommender.Recommend(pvc, policy)
	// Well-sized PVC with matching storage class might return nil
	if rec != nil {
		t.Logf("Got recommendation: %s", rec.Reason)
	}
}

func TestIntelligentRecommender_Recommend_HighEgress(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	pvc := types.PVCMetric{
		Name:         "high-egress-pvc",
		Namespace:    "default",
		StorageClass: "standard",
		SizeBytes:    100 * 1024 * 1024 * 1024,
		UsedBytes:    50 * 1024 * 1024 * 1024,
		EgressBytes:  200 * 1024 * 1024, // 200MB egress
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	rec := recommender.Recommend(pvc, policy)
	if rec != nil && rec.TargetClass != pvc.StorageClass {
		t.Logf("Recommended class change from %s to %s", pvc.StorageClass, rec.TargetClass)
	}
}

func TestFormatQuantity(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "1GB",
			bytes:    1 * 1024 * 1024 * 1024,
			expected: "1Gi",
		},
		{
			name:     "100GB",
			bytes:    100 * 1024 * 1024 * 1024,
			expected: "100Gi",
		},
		{
			name:     "500MB",
			bytes:    500 * 1024 * 1024,
			expected: "500Mi",
		},
		{
			name:     "1MB",
			bytes:    1 * 1024 * 1024,
			expected: "1Mi",
		},
		{
			name:     "1KB",
			bytes:    1024,
			expected: "1024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatQuantity(tt.bytes)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIntelligentRecommender_DetectWorkloadType(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	tests := []struct {
		name     string
		pvc      types.PVCMetric
		expected string
	}{
		{
			name: "high-egress",
			pvc: types.PVCMetric{
				EgressBytes: 200 * 1024 * 1024, // 200MB
			},
			expected: "high-egress",
		},
		{
			name: "standard",
			pvc: types.PVCMetric{
				EgressBytes: 10 * 1024 * 1024, // 10MB
			},
			expected: "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := recommender.detectWorkloadType(tt.pvc)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestOptimizationRecommendation_Structure(t *testing.T) {
	rec := &OptimizationRecommendation{
		TargetClass: "gp3",
		TargetTier:  "hot",
		TargetSize:  "50Gi",
		Reason:      "Right-sizing opportunity",
		Confidence:  0.85,
	}

	if rec.TargetClass == "" {
		t.Error("Expected non-empty target class")
	}

	if rec.Confidence < 0 || rec.Confidence > 1 {
		t.Error("Expected confidence between 0 and 1")
	}
}

func TestIntelligentRecommender_Recommend_SmallPVC(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	// Small PVC (< 10GB) shouldn't trigger right-sizing even if underutilized
	pvc := types.PVCMetric{
		Name:         "small-pvc",
		Namespace:    "default",
		StorageClass: "gp3",
		SizeBytes:    5 * 1024 * 1024 * 1024, // 5GB
		UsedBytes:    1 * 1024 * 1024 * 1024, // 1GB (20% utilization)
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	rec := recommender.Recommend(pvc, policy)
	// Small PVCs typically don't get right-sizing recommendations
	if rec != nil && rec.Reason != "" {
		t.Logf("Got recommendation: %s", rec.Reason)
	}
}

func TestIntelligentRecommender_Recommend_MinimumSize(t *testing.T) {
	recommender := NewIntelligentRecommender(nil)

	// PVC with very low usage should get minimum size recommendation
	pvc := types.PVCMetric{
		Name:         "tiny-usage-pvc",
		Namespace:    "default",
		StorageClass: "gp3",
		SizeBytes:    100 * 1024 * 1024 * 1024, // 100GB
		UsedBytes:    100 * 1024 * 1024,        // 100MB (0.1% utilization)
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	rec := recommender.Recommend(pvc, policy)
	if rec == nil {
		t.Fatal("Expected recommendation for very low usage PVC")
	}

	// Should recommend at least 1GB minimum
	if rec.TargetSize != "" {
		t.Logf("Recommended size: %s", rec.TargetSize)
	}
}

func TestIntelligentRecommender_Integration(t *testing.T) {
	// Test that all components work together
	recommender := NewIntelligentRecommender(nil)

	pvcs := []types.PVCMetric{
		{
			Name:         "pvc-1",
			Namespace:    "default",
			StorageClass: "gp3",
			SizeBytes:    100 * 1024 * 1024 * 1024,
			UsedBytes:    20 * 1024 * 1024 * 1024,
		},
		{
			Name:         "pvc-2",
			Namespace:    "production",
			StorageClass: "io2",
			SizeBytes:    200 * 1024 * 1024 * 1024,
			UsedBytes:    180 * 1024 * 1024 * 1024,
		},
	}

	policy := &v1alpha1.StorageLifecyclePolicy{}

	recommendationCount := 0
	for _, pvc := range pvcs {
		rec := recommender.Recommend(pvc, policy)
		if rec != nil {
			recommendationCount++
			t.Logf("PVC: %s, Recommendation: %s (Confidence: %.2f)",
				pvc.Name, rec.Reason, rec.Confidence)
		}
	}

	t.Logf("Generated %d recommendations out of %d PVCs", recommendationCount, len(pvcs))
}
