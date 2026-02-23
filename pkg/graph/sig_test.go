package graph

import (
	"context"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

func TestNewSIG(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	uri := "bolt://localhost:7687"
	username := "neo4j"
	password := "password"

	sig, err := NewSIG(uri, username, password)
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	if sig == nil {
		t.Fatal("Expected non-nil SIG")
	}
}

func TestNewSIG_InvalidConnection(t *testing.T) {
	uri := "bolt://invalid-host:7687"
	username := "neo4j"
	password := "wrong"

	_, err := NewSIG(uri, username, password)
	if err == nil {
		t.Error("Expected error for invalid connection")
	}
}

func TestSIG_SyncPVCs(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()
	metrics := []types.PVCMetric{
		{
			Name:         "test-pvc-1",
			Namespace:    "default",
			StorageClass: "gp3",
			SizeBytes:    100 * 1024 * 1024 * 1024,
			UsedBytes:    50 * 1024 * 1024 * 1024,
			Region:       "us-east-1",
			Provider:     "aws",
			ClusterID:    "cluster-1",
			MonthlyCost:  8.5,
			ReadIOPS:     1000,
			WriteIOPS:    500,
		},
		{
			Name:         "test-pvc-2",
			Namespace:    "production",
			StorageClass: "io2",
			SizeBytes:    200 * 1024 * 1024 * 1024,
			UsedBytes:    150 * 1024 * 1024 * 1024,
			Region:       "us-west-2",
			Provider:     "aws",
			ClusterID:    "cluster-2",
			MonthlyCost:  25.0,
			ReadIOPS:     5000,
			WriteIOPS:    3000,
		},
	}

	err = sig.SyncPVCs(ctx, metrics)
	if err != nil {
		t.Fatalf("Failed to sync PVCs: %v", err)
	}
}

func TestSIG_SyncPVC(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	err = sig.SyncPVC(ctx, "default", "single-pvc", "gp3", 100*1024*1024*1024)
	if err != nil {
		t.Fatalf("Failed to sync single PVC: %v", err)
	}
}

func TestSIG_MapPodToPVC(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	// First create a PVC
	err = sig.SyncPVC(ctx, "default", "test-pvc", "gp3", 100*1024*1024*1024)
	if err != nil {
		t.Fatalf("Failed to sync PVC: %v", err)
	}

	// Then map pod to PVC
	err = sig.MapPodToPVC(ctx, "test-pod", "default", "test-pvc")
	if err != nil {
		t.Fatalf("Failed to map pod to PVC: %v", err)
	}
}

func TestSIG_GetCrossRegionGravity(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	// Setup test data with cross-region scenario
	metrics := []types.PVCMetric{
		{
			Name:      "cross-region-pvc",
			Namespace: "default",
			Region:    "us-west-2",
			Provider:  "aws",
		},
	}
	_ = sig.SyncPVCs(ctx, metrics)

	// Create pod in different region
	_ = sig.MapPodToPVC(ctx, "pod-east", "default", "cross-region-pvc")

	// Query cross-region gravity
	pvcs, err := sig.GetCrossRegionGravity(ctx)
	if err != nil {
		t.Fatalf("Failed to get cross-region gravity: %v", err)
	}

	t.Logf("Found %d cross-region PVCs", len(pvcs))
}

func TestSIG_GetCrossCloudWorkloads(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	workloads, err := sig.GetCrossCloudWorkloads(ctx)
	if err != nil {
		t.Fatalf("Failed to get cross-cloud workloads: %v", err)
	}

	t.Logf("Found %d cross-cloud workloads", len(workloads))
}

func TestSIG_GetStorageClassUtilization(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	// Create test PVCs with different storage classes
	metrics := []types.PVCMetric{
		{
			Name:         "pvc-gp3-1",
			Namespace:    "default",
			StorageClass: "gp3",
			SizeBytes:    100 * 1024 * 1024 * 1024,
			UsedBytes:    80 * 1024 * 1024 * 1024,
			MonthlyCost:  8.0,
		},
		{
			Name:         "pvc-io2-1",
			Namespace:    "default",
			StorageClass: "io2",
			SizeBytes:    200 * 1024 * 1024 * 1024,
			UsedBytes:    150 * 1024 * 1024 * 1024,
			MonthlyCost:  25.0,
		},
	}
	_ = sig.SyncPVCs(ctx, metrics)

	stats, err := sig.GetStorageClassUtilization(ctx)
	if err != nil {
		t.Fatalf("Failed to get storage class utilization: %v", err)
	}

	if len(stats) == 0 {
		t.Error("Expected non-empty stats")
	}

	for _, stat := range stats {
		t.Logf("Storage Class: %s, PVC Count: %d, Total Cost: %.2f",
			stat.StorageClass, stat.PVCCount, stat.TotalMonthlyCost)
	}
}

func TestSIG_DeletePVC(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	// Create PVC
	err = sig.SyncPVC(ctx, "default", "delete-me", "gp3", 100*1024*1024*1024)
	if err != nil {
		t.Fatalf("Failed to create PVC: %v", err)
	}

	// Delete PVC
	err = sig.DeletePVC(ctx, "default", "delete-me")
	if err != nil {
		t.Fatalf("Failed to delete PVC: %v", err)
	}
}

func TestCrossCloudWorkload_Structure(t *testing.T) {
	workload := CrossCloudWorkload{
		SourcePod:       "pod-1",
		SourceNamespace: "default",
		SourceProvider:  "aws",
		TargetPod:       "pod-2",
		TargetNamespace: "production",
		TargetProvider:  "gcp",
	}

	if workload.SourceProvider == workload.TargetProvider {
		t.Error("Expected different providers for cross-cloud workload")
	}

	if workload.SourcePod == "" {
		t.Error("Expected non-empty source pod")
	}
}

func TestStorageClassStats_Structure(t *testing.T) {
	stats := StorageClassStats{
		StorageClass:     "gp3",
		PVCCount:         10,
		TotalSizeBytes:   1024 * 1024 * 1024 * 1024,
		TotalUsedBytes:   800 * 1024 * 1024 * 1024,
		TotalMonthlyCost: 80.0,
		AvgUtilization:   80.0,
	}

	if stats.PVCCount <= 0 {
		t.Error("Expected positive PVC count")
	}

	if stats.TotalUsedBytes > stats.TotalSizeBytes {
		t.Error("Used bytes should not exceed total size")
	}

	if stats.AvgUtilization < 0 || stats.AvgUtilization > 100 {
		t.Error("Average utilization should be between 0 and 100")
	}
}

func TestSIG_SyncPVCs_EmptyList(t *testing.T) {
	t.Skip("Requires Neo4j instance")

	sig, err := setupTestSIG()
	if err != nil {
		t.Skipf("Cannot connect to Neo4j: %v", err)
	}
	defer func() {
		_ = sig.Close(context.Background())
	}()

	ctx := context.Background()

	// Should handle empty list gracefully
	err = sig.SyncPVCs(ctx, []types.PVCMetric{})
	if err != nil {
		t.Errorf("Should handle empty metrics: %v", err)
	}
}

// Helper function to setup test SIG connection
func setupTestSIG() (*SIG, error) {
	uri := "bolt://localhost:7687"
	username := "neo4j"
	password := "password"
	return NewSIG(uri, username, password)
}
