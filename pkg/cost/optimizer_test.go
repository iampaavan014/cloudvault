package cost

import (
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/ai"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

type MockPricingProvider struct {
	Prices map[string]StorageClassPricing
}

func (m *MockPricingProvider) GetPrice(provider, storageClass, region string) StorageClassPricing {
	key := storageClass
	if p, ok := m.Prices[key]; ok {
		return p
	}
	return StorageClassPricing{PerGBMonth: 0.10}
}

func TestNewOptimizer(t *testing.T) {
	opt := NewOptimizer()
	if opt == nil {
		t.Fatal("Expected optimizer to be created")
	}
	if opt.calculator == nil {
		t.Error("Expected calculator to be set")
	}
	if opt.forecaster == nil {
		t.Error("Expected forecaster to be set")
	}
	if opt.rlAgent == nil {
		t.Error("Expected rlAgent to be set")
	}
}

func TestOptimizer_GenerateRecommendations(t *testing.T) {
	mockPricing := &MockPricingProvider{
		Prices: map[string]StorageClassPricing{
			"gp3": {PerGBMonth: 0.08},
			"sc1": {PerGBMonth: 0.025},
		},
	}
	calc := NewCalculatorWithProvider(mockPricing)
	opt := &Optimizer{
		calculator: calc,
		forecaster: ai.NewCostForecaster(),
		rlAgent:    ai.NewRLAgent(),
	}

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
	mockPricing := &MockPricingProvider{
		Prices: map[string]StorageClassPricing{
			"gp3": {PerGBMonth: 0.08},
			"gp2": {PerGBMonth: 0.10},
			"io1": {PerGBMonth: 0.12, PerIOPS: 0.06, Provisioned: true},
			"sc1": {PerGBMonth: 0.025},
			"st1": {PerGBMonth: 0.045},
		},
	}
	calc := NewCalculatorWithProvider(mockPricing)
	opt := &Optimizer{
		calculator: calc,
		forecaster: ai.NewCostForecaster(),
		rlAgent:    ai.NewRLAgent(),
	}

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

func TestOptimizer_CalculateTotalSavings(t *testing.T) {
	opt := NewOptimizer()

	recommendations := []types.Recommendation{
		{
			Type:           "resize",
			MonthlySavings: 10.5,
		},
		{
			Type:           "delete_zombie",
			MonthlySavings: 5.25,
		},
		{
			Type:           "storage_class",
			MonthlySavings: 3.0,
		},
	}

	total := opt.CalculateTotalSavings(recommendations)
	expected := 18.75 // 10.5 + 5.25 + 3.0
	if total != expected {
		t.Errorf("Expected total savings %f, got %f", expected, total)
	}
}

func TestOptimizer_FilterByType(t *testing.T) {
	opt := NewOptimizer()

	recommendations := []types.Recommendation{
		{Type: "resize", MonthlySavings: 10.0},
		{Type: "delete_zombie", MonthlySavings: 5.0},
		{Type: "resize", MonthlySavings: 8.0},
		{Type: "storage_class", MonthlySavings: 3.0},
	}

	filtered := opt.FilterByType(recommendations, "resize")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 resize recommendations, got %d", len(filtered))
	}
	for _, rec := range filtered {
		if rec.Type != "resize" {
			t.Errorf("Expected type 'resize', got '%s'", rec.Type)
		}
	}
}

func TestOptimizer_FilterByImpact(t *testing.T) {
	opt := NewOptimizer()

	recommendations := []types.Recommendation{
		{Impact: "high", MonthlySavings: 100.0},
		{Impact: "medium", MonthlySavings: 50.0},
		{Impact: "low", MonthlySavings: 10.0},
		{Impact: "high", MonthlySavings: 80.0},
	}

	filtered := opt.FilterByImpact(recommendations, "high")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 high impact recommendations, got %d", len(filtered))
	}
	for _, rec := range filtered {
		if rec.Impact != "high" {
			t.Errorf("Expected impact 'high', got '%s'", rec.Impact)
		}
	}
}

func TestOptimizer_GetQuickWins(t *testing.T) {
	opt := NewOptimizer()

	recommendations := []types.Recommendation{
		{
			Type:           "resize",
			Impact:         "low",
			MonthlySavings: 100.0,
		},
		{
			Type:           "storage_class",
			Impact:         "medium",
			MonthlySavings: 50.0,
		},
		{
			Type:           "migrate",
			Impact:         "high",
			MonthlySavings: 200.0,
		},
		{
			Type:           "delete_zombie",
			Impact:         "low",
			MonthlySavings: 30.0,
		},
	}

	quickWins := opt.GetQuickWins(recommendations)
	// Quick wins: Impact == "low" and MonthlySavings > 5.0
	if len(quickWins) != 2 {
		t.Errorf("Expected 2 quick wins, got %d", len(quickWins))
	}
	for _, rec := range quickWins {
		if rec.Impact != "low" {
			t.Errorf("Expected low impact for quick wins, got '%s'", rec.Impact)
		}
		if rec.MonthlySavings <= 5.0 {
			t.Errorf("Expected savings > 5.0 for quick wins, got %f", rec.MonthlySavings)
		}
	}
}

func TestOptimizer_CheckCrossCloudMigration(t *testing.T) {
	mockPricing := &MockPricingProvider{
		Prices: map[string]StorageClassPricing{
			"gp3":         {PerGBMonth: 0.08},
			"pd-standard": {PerGBMonth: 0.04},
		},
	}
	calc := NewCalculatorWithProvider(mockPricing)
	opt := &Optimizer{
		calculator: calc,
		forecaster: ai.NewCostForecaster(),
		rlAgent:    ai.NewRLAgent(),
	}

	tests := []struct {
		name      string
		metric    types.PVCMetric
		expectRec bool
	}{
		{
			name: "high-cost-aws-to-gcp",
			metric: types.PVCMetric{
				Name:         "expensive-pvc",
				Namespace:    "prod",
				SizeBytes:    1000 * 1024 * 1024 * 1024, // 1TB
				StorageClass: "gp3",
				Provider:     "aws",
				Region:       "us-east-1",
				MonthlyCost:  80.0,
			},
			expectRec: true,
		},
		{
			name: "low-cost-no-migration",
			metric: types.PVCMetric{
				Name:         "cheap-pvc",
				Namespace:    "dev",
				SizeBytes:    10 * 1024 * 1024 * 1024, // 10GB
				StorageClass: "gp3",
				Provider:     "aws",
				Region:       "us-east-1",
				MonthlyCost:  0.8,
			},
			expectRec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := opt.checkCrossCloudMigration(&tt.metric)
			if tt.expectRec && rec == nil {
				t.Error("Expected migration recommendation, got nil")
			}
			if !tt.expectRec && rec != nil {
				t.Errorf("Expected no recommendation, got %v", rec)
			}
		})
	}
}

func TestOptimizer_CheckOversizedVolume(t *testing.T) {
	mockPricing := &MockPricingProvider{
		Prices: map[string]StorageClassPricing{
			"gp3": {PerGBMonth: 0.08},
		},
	}
	calc := NewCalculatorWithProvider(mockPricing)
	opt := &Optimizer{
		calculator: calc,
		forecaster: ai.NewCostForecaster(),
		rlAgent:    ai.NewRLAgent(),
	}

	tests := []struct {
		name      string
		sizeBytes int64
		usedBytes int64
		expectRec bool
	}{
		{
			name:      "highly-underutilized",
			sizeBytes: 1000 * 1024 * 1024 * 1024, // 1TB
			usedBytes: 50 * 1024 * 1024 * 1024,   // 50GB (5%)
			expectRec: true,
		},
		{
			name:      "well-utilized",
			sizeBytes: 100 * 1024 * 1024 * 1024, // 100GB
			usedBytes: 80 * 1024 * 1024 * 1024,  // 80GB (80%)
			expectRec: false,
		},
		{
			name:      "no-usage-data",
			sizeBytes: 100 * 1024 * 1024 * 1024, // 100GB
			usedBytes: 0,
			expectRec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := &types.PVCMetric{
				Name:         "test-pvc",
				Namespace:    "default",
				SizeBytes:    tt.sizeBytes,
				UsedBytes:    tt.usedBytes,
				StorageClass: "gp3",
			}
			rec := opt.checkOversizedVolume(metric)
			if tt.expectRec && rec == nil {
				t.Error("Expected resize recommendation, got nil")
			}
			if !tt.expectRec && rec != nil {
				t.Errorf("Expected no recommendation, got %v", rec)
			}
		})
	}
}

func TestOptimizer_CheckZombieVolume(t *testing.T) {
	opt := NewOptimizer()

	now := time.Now()

	tests := []struct {
		name           string
		lastAccessedAt time.Time
		expectRec      bool
	}{
		{
			name:           "old-zombie",
			lastAccessedAt: now.Add(-45 * 24 * time.Hour), // 45 days ago
			expectRec:      true,
		},
		{
			name:           "recent-access",
			lastAccessedAt: now.Add(-10 * 24 * time.Hour), // 10 days ago
			expectRec:      false,
		},
		{
			name:           "no-access-data",
			lastAccessedAt: time.Time{},
			expectRec:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := &types.PVCMetric{
				Name:           "test-pvc",
				Namespace:      "default",
				SizeBytes:      100 * 1024 * 1024 * 1024,
				StorageClass:   "gp3",
				LastAccessedAt: tt.lastAccessedAt,
			}
			rec := opt.checkZombieVolume(metric)
			if tt.expectRec && rec == nil {
				t.Error("Expected zombie recommendation, got nil")
			}
			if !tt.expectRec && rec != nil {
				t.Errorf("Expected no recommendation, got %v", rec)
			}
		})
	}
}

func TestDetermineImpact(t *testing.T) {
	tests := []struct {
		name           string
		iops           float64
		targetClass    string
		expectedImpact string
	}{
		{"cold-storage-high-iops", 200.0, "sc1", "medium"},
		{"cold-storage-low-iops", 50.0, "sc1", "low"},
		{"standard-hdd-high-iops", 150.0, "st1", "medium"},
		{"standard-hdd-low-iops", 50.0, "pd-standard", "low"},
		{"gp3-high-iops", 3000.0, "gp3", "medium"},
		{"gp3-low-iops", 500.0, "gp3", "low"},
		{"balanced-low-iops", 100.0, "pd-balanced", "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact := determineImpact(tt.iops, tt.targetClass)
			if impact != tt.expectedImpact {
				t.Errorf("Expected impact '%s', got '%s'", tt.expectedImpact, impact)
			}
		})
	}
}
