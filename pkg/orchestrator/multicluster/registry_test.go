package multicluster

import (
	"context"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewClusterRegistry(t *testing.T) {
	registry := NewClusterRegistry(nil)
	assert.NotNil(t, registry)
	assert.Equal(t, 5*time.Minute, registry.config.DiscoveryInterval)
}

func TestClusterRegistry_RegisterCluster(t *testing.T) {
	registry := NewClusterRegistry(&RegistryConfig{
		DiscoveryInterval:   1 * time.Hour,
		HealthCheckInterval: 1 * time.Hour,
		MetricsInterval:     1 * time.Hour,
	})

	info := &ClusterInfo{
		ID:       "cluster-1",
		Name:     "Prod Cluster",
		Provider: "aws",
		Region:   "us-east-1",
	}

	// Register without client (will skip discovery and metrics)
	err := registry.RegisterCluster(context.Background(), info)
	assert.NoError(t, err)

	cluster, err := registry.GetCluster("cluster-1")
	assert.NoError(t, err)
	assert.Equal(t, "healthy", cluster.Status)
}

func TestClusterRegistry_UnregisterCluster(t *testing.T) {
	registry := NewClusterRegistry(nil)
	info := &ClusterInfo{ID: "cluster-1"}
	_ = registry.RegisterCluster(context.Background(), info)

	err := registry.UnregisterCluster("cluster-1")
	assert.NoError(t, err)

	_, err = registry.GetCluster("cluster-1")
	assert.Error(t, err)
}

func TestClusterRegistry_GetOptimalCluster(t *testing.T) {
	registry := NewClusterRegistry(nil)

	c1 := &ClusterInfo{
		ID:       "c1",
		Provider: "aws",
		CostData: ClusterCostData{CostPerGB: 0.1},
		Status:   "healthy",
	}
	c2 := &ClusterInfo{
		ID:       "c2",
		Provider: "gcp",
		CostData: ClusterCostData{CostPerGB: 0.05},
		Status:   "healthy",
	}

	_ = registry.RegisterCluster(context.Background(), c1)
	_ = registry.RegisterCluster(context.Background(), c2)

	request := WorkloadPlacementRequest{
		Name: "test-app",
		Constraints: PlacementConstraints{
			MaxCostPerGB: 0.2,
		},
	}

	optimal, err := registry.GetOptimalCluster(request)
	assert.NoError(t, err)
	assert.Equal(t, "c2", optimal.ID)
}

func TestClusterRegistry_MeetsConstraints(t *testing.T) {
	registry := NewClusterRegistry(nil)
	cluster := &ClusterInfo{
		ID:       "test-cluster",
		Provider: "aws",
		Region:   "us-east-1",
		CostData: ClusterCostData{CostPerGB: 0.1},
		Capabilities: ClusterCapabilities{
			StorageClasses: []string{"gp3", "io2"},
		},
	}

	tests := []struct {
		name        string
		constraints PlacementConstraints
		expected    bool
	}{
		{
			name: "Correct Provider",
			constraints: PlacementConstraints{
				RequiredProvider: "aws",
			},
			expected: true,
		},
		{
			name: "Wrong Provider",
			constraints: PlacementConstraints{
				RequiredProvider: "gcp",
			},
			expected: false,
		},
		{
			name: "Max Cost OK",
			constraints: PlacementConstraints{
				MaxCostPerGB: 0.2,
			},
			expected: true,
		},
		{
			name: "Max Cost Too High",
			constraints: PlacementConstraints{
				MaxCostPerGB: 0.05,
			},
			expected: false,
		},
		{
			name: "Required Capability Present",
			constraints: PlacementConstraints{
				RequireCapabilities: []string{"gp3"},
			},
			expected: true,
		},
		{
			name: "Required Capability Missing",
			constraints: PlacementConstraints{
				RequireCapabilities: []string{"standard"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, registry.meetsConstraints(cluster, tt.constraints))
		})
	}
}

func TestClusterRegistry_UpdateClusterHeartbeat(t *testing.T) {
	registry := NewClusterRegistry(nil)

	metrics := &types.StorageMetrics{
		TotalStorageBytes: 1000,
		UsedStorageBytes:  500,
		PVCs:              make([]types.PVCMetric, 2),
	}

	err := registry.UpdateClusterHeartbeat("new-cluster", "v1.0.0", metrics)
	assert.NoError(t, err)

	cluster, _ := registry.GetCluster("new-cluster")
	assert.Equal(t, "healthy", cluster.Status)
	assert.Equal(t, int64(1000), cluster.Metrics.TotalStorageBytes)
	assert.Equal(t, 2, cluster.Metrics.TotalPVCs)
}

func TestClusterRegistry_GetCrossClusterCostAggregation(t *testing.T) {
	registry := NewClusterRegistry(nil)

	c1 := &ClusterInfo{
		ID:       "c1",
		Provider: "aws",
		Region:   "us-east-1",
		CostData: ClusterCostData{MonthlyCost: 100},
	}
	c2 := &ClusterInfo{
		ID:       "c2",
		Provider: "aws",
		Region:   "us-west-2",
		CostData: ClusterCostData{MonthlyCost: 150},
	}

	_ = registry.RegisterCluster(context.Background(), c1)
	_ = registry.RegisterCluster(context.Background(), c2)

	agg := registry.GetCrossClusterCostAggregation()
	assert.Equal(t, 250.0, agg.TotalCost)
	assert.Equal(t, 250.0, agg.ByProvider["aws"])
	assert.Equal(t, 100.0, agg.ByRegion["us-east-1"])
}

func TestClusterRegistry_CalculatePlacementScore(t *testing.T) {
	registry := NewClusterRegistry(nil)

	cluster := &ClusterInfo{
		ID:       "c1",
		Provider: "aws",
		Region:   "us-east-1",
		CostData: ClusterCostData{CostPerGB: 0.1},
		Metrics: ClusterMetrics{
			AvailableCapacity: 1000,
			IOPS:              100,
		},
		Status: "healthy",
	}

	_ = registry.RegisterCluster(context.Background(), cluster)

	// Test 1: High gravity (same cluster)
	req1 := WorkloadPlacementRequest{
		Name:        "app-1",
		StorageSize: 500,
		IOPS:        50,
		DataGravity: []string{"c1"},
	}
	score1 := registry.calculatePlacementScore(cluster, req1)

	// Test 2: Lower gravity (different provider)
	req2 := WorkloadPlacementRequest{
		Name:        "app-2",
		StorageSize: 500,
		IOPS:        50,
		DataGravity: []string{"other-cluster"},
	}
	score2 := registry.calculatePlacementScore(cluster, req2)

	assert.Greater(t, score1, score2)
}

func TestClusterRegistry_PerformHealthChecks(t *testing.T) {
	registry := NewClusterRegistry(nil)

	cluster := &ClusterInfo{
		ID:       "unreachable-cluster",
		LastSeen: time.Now().Add(-10 * time.Minute),
		Status:   "healthy",
	}
	registry.clusters[cluster.ID] = cluster

	registry.performHealthChecks()

	assert.Equal(t, "unreachable", cluster.Status)
}

func TestClusterRegistry_ListClusters(t *testing.T) {
	registry := NewClusterRegistry(nil)

	_ = registry.RegisterCluster(context.Background(), &ClusterInfo{ID: "c1"})
	_ = registry.RegisterCluster(context.Background(), &ClusterInfo{ID: "c2"})

	// Make c2 unreachable via time trick
	c2, _ := registry.GetCluster("c2")
	c2.LastSeen = time.Now().Add(-1 * time.Hour)
	registry.performHealthChecks()

	all := registry.GetAllClusters()
	assert.Equal(t, 2, len(all))

	healthy := registry.GetHealthyClusters()
	assert.Equal(t, 1, len(healthy))
	assert.Equal(t, "c1", healthy[0].ID)
}
