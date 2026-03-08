package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPVCMetric_Conversions(t *testing.T) {
	m := PVCMetric{
		SizeBytes: 100 * 1024 * 1024 * 1024,
		UsedBytes: 50 * 1024 * 1024 * 1024,
	}

	assert.Equal(t, 100.0, m.SizeGB())
	assert.Equal(t, 50.0, m.UsedGB())
	assert.Equal(t, 50.0, m.UsagePercent())

	// Test zero size
	m0 := PVCMetric{SizeBytes: 0}
	assert.Equal(t, 0.0, m0.UsagePercent())
}

func TestPVCMetric_AnnualCost(t *testing.T) {
	m := PVCMetric{
		MonthlyCost: 10.5,
	}
	assert.Equal(t, 126.0, m.AnnualCost())
}

func TestPVCMetric_IsZombie(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		usedBytes      int64
		sizeBytes      int64
		lastAccessedAt time.Time
		expected       bool
	}{
		{
			name:           "Zombie - Unused for > 30 days",
			usedBytes:      1 * 1024 * 1024,
			sizeBytes:      100 * 1024 * 1024 * 1024,
			lastAccessedAt: now.Add(-40 * 24 * time.Hour),
			expected:       true,
		},
		{
			name:           "Not Zombie - Recently used",
			usedBytes:      1 * 1024 * 1024,
			sizeBytes:      100 * 1024 * 1024 * 1024,
			lastAccessedAt: now.Add(-2 * 24 * time.Hour),
			expected:       false,
		},
		{
			name:           "Not Zombie - Never used (zero value)",
			usedBytes:      1 * 1024 * 1024,
			sizeBytes:      100 * 1024 * 1024 * 1024,
			lastAccessedAt: time.Time{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := PVCMetric{
				UsedBytes:      tt.usedBytes,
				SizeBytes:      tt.sizeBytes,
				LastAccessedAt: tt.lastAccessedAt,
			}
			assert.Equal(t, tt.expected, m.IsZombie())
		})
	}
}

func TestRecommendation_Impact(t *testing.T) {
	r := Recommendation{Impact: "high"}
	assert.Equal(t, "high", r.Impact)
}
