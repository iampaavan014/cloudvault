package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPVCMetric_TotalIOPS(t *testing.T) {
	metric := PVCMetric{
		ReadIOPS:  1000,
		WriteIOPS: 500,
	}

	total := metric.TotalIOPS()
	expected := 1500.0

	if total != expected {
		t.Errorf("Expected TotalIOPS %v, got %v", expected, total)
	}
}

func TestPVCMetric_IsZombie(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		lastAccessedAt time.Time
		expected       bool
	}{
		{
			name:           "Zombie - 40 days old",
			lastAccessedAt: now.Add(-40 * 24 * time.Hour),
			expected:       true,
		},
		{
			name:           "Not Zombie - 20 days old",
			lastAccessedAt: now.Add(-20 * 24 * time.Hour),
			expected:       false,
		},
		{
			name:           "Not Zombie - Recent",
			lastAccessedAt: now.Add(-1 * time.Hour),
			expected:       false,
		},
		{
			name:           "Not Zombie - Zero time",
			lastAccessedAt: time.Time{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := PVCMetric{
				LastAccessedAt: tt.lastAccessedAt,
			}

			result := metric.IsZombie()
			if result != tt.expected {
				t.Errorf("Expected IsZombie() = %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPVCMetric_JSONMarshaling(t *testing.T) {
	now := time.Now()
	metric := PVCMetric{
		Name:         "test-pvc",
		Namespace:    "default",
		ClusterID:    "test-cluster",
		StorageClass: "gp3",
		SizeBytes:    100 * 1024 * 1024 * 1024,
		UsedBytes:    50 * 1024 * 1024 * 1024,
		HourlyCost:   0.011,
		MonthlyCost:  8.00,
		CreatedAt:    now,
		Labels: map[string]string{
			"app": "test",
		},
		Annotations: map[string]string{
			"note": "test-annotation",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(metric)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded PVCMetric
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.Name != metric.Name {
		t.Errorf("Name mismatch: expected %s, got %s", metric.Name, decoded.Name)
	}
	if decoded.Namespace != metric.Namespace {
		t.Errorf("Namespace mismatch")
	}
	if decoded.SizeBytes != metric.SizeBytes {
		t.Errorf("SizeBytes mismatch")
	}
	if decoded.MonthlyCost != metric.MonthlyCost {
		t.Errorf("MonthlyCost mismatch")
	}
}

func TestCostSummary_JSONMarshaling(t *testing.T) {
	summary := CostSummary{
		TotalMonthlyCost: 100.50,
		ByNamespace: map[string]float64{
			"production": 60.00,
			"staging":    40.50,
		},
		ByStorageClass: map[string]float64{
			"gp3": 80.00,
			"io1": 20.50,
		},
		TopExpensive: []PVCMetric{
			{Name: "expensive-pvc", MonthlyCost: 50.00},
		},
		ZombieVolumes: []PVCMetric{
			{Name: "zombie-pvc", MonthlyCost: 10.00},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded CostSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.TotalMonthlyCost != summary.TotalMonthlyCost {
		t.Errorf("TotalMonthlyCost mismatch")
	}
	if len(decoded.ByNamespace) != len(summary.ByNamespace) {
		t.Errorf("ByNamespace length mismatch")
	}
	if len(decoded.TopExpensive) != len(summary.TopExpensive) {
		t.Errorf("TopExpensive length mismatch")
	}
}

func TestRecommendation_JSONMarshaling(t *testing.T) {
	rec := Recommendation{
		Type:             "resize",
		PVC:              "test-pvc",
		Namespace:        "default",
		CurrentState:     "200GB",
		RecommendedState: "100GB",
		MonthlySavings:   8.00,
		Reasoning:        "Volume is underutilized",
		Impact:           "medium",
	}

	// Marshal to JSON
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded Recommendation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.Type != rec.Type {
		t.Errorf("Type mismatch")
	}
	if decoded.MonthlySavings != rec.MonthlySavings {
		t.Errorf("MonthlySavings mismatch")
	}
	if decoded.Impact != rec.Impact {
		t.Errorf("Impact mismatch")
	}
}

func TestClusterInfo_JSONMarshaling(t *testing.T) {
	info := ClusterInfo{
		ID:       "test-cluster-id",
		Name:     "test-cluster",
		Provider: "aws",
		Region:   "us-east-1",
		Version:  "v1.28.0",
	}

	// Marshal to JSON
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded ClusterInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.ID != info.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.Provider != info.Provider {
		t.Errorf("Provider mismatch")
	}
	if decoded.Region != info.Region {
		t.Errorf("Region mismatch")
	}
}
