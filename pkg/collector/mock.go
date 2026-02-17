package collector

import (
	"context"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

// MockPVCCollector provides synthetic metrics for testing
type MockPVCCollector struct{}

func NewMockPVCCollector() *MockPVCCollector {
	return &MockPVCCollector{}
}

func (m *MockPVCCollector) CollectAll(ctx context.Context) ([]types.PVCMetric, error) {
	return []types.PVCMetric{
		{
			Name:         "mysql-data",
			Namespace:    "default",
			SizeBytes:    100 * 1024 * 1024 * 1024, // 100GB
			UsedBytes:    45 * 1024 * 1024 * 1024,  // 45GB
			StorageClass: "gp3",
			Provider:     "aws",
			Region:       "us-east-1",
			CreatedAt:    time.Now().Add(-24 * time.Hour),
		},
		{
			Name:         "redis-state",
			Namespace:    "cache",
			SizeBytes:    10 * 1024 * 1024 * 1024, // 10GB
			UsedBytes:    1 * 1024 * 1024 * 1024,  // 1GB
			StorageClass: "standard",
			Provider:     "aws",
			Region:       "us-east-1",
			CreatedAt:    time.Now().Add(-12 * time.Hour),
		},
	}, nil
}

func (m *MockPVCCollector) CollectByNamespace(ctx context.Context, ns string) ([]types.PVCMetric, error) {
	all, _ := m.CollectAll(ctx)
	var filtered []types.PVCMetric
	for _, metric := range all {
		if metric.Namespace == ns {
			filtered = append(filtered, metric)
		}
	}
	return filtered, nil
}
