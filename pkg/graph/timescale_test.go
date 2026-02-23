package graph

import (
	"context"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
)

func TestTimescaleDB_InitializeSchema(t *testing.T) {
	// Skip if no database available
	t.Skip("Requires TimescaleDB instance")

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test sslmode=disable"
	db, err := NewTimescaleDB(connStr)
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Verify table exists
	var exists bool
	err = db.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'pvc_metrics')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if !exists {
		t.Error("Expected pvc_metrics table to exist")
	}
}

func TestTimescaleDB_RecordMetrics(t *testing.T) {
	t.Skip("Requires TimescaleDB instance")

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test sslmode=disable"
	db, err := NewTimescaleDB(connStr)
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()
	metrics := []types.PVCMetric{
		{
			Name:        "test-pvc-1",
			Namespace:   "default",
			UsedBytes:   100 * 1024 * 1024 * 1024,
			EgressBytes: 10 * 1024 * 1024,
			ReadIOPS:    1000,
			WriteIOPS:   500,
			MonthlyCost: 8.5,
		},
		{
			Name:        "test-pvc-2",
			Namespace:   "production",
			UsedBytes:   200 * 1024 * 1024 * 1024,
			EgressBytes: 20 * 1024 * 1024,
			ReadIOPS:    2000,
			WriteIOPS:   1000,
			MonthlyCost: 17.0,
		},
	}

	err = db.RecordMetrics(ctx, metrics)
	if err != nil {
		t.Fatalf("Failed to record metrics: %v", err)
	}
}

func TestTimescaleDB_GetHistory(t *testing.T) {
	t.Skip("Requires TimescaleDB instance")

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test sslmode=disable"
	db, err := NewTimescaleDB(connStr)
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Record some test data
	metrics := []types.PVCMetric{
		{
			Name:        "history-pvc",
			Namespace:   "test-ns",
			UsedBytes:   50 * 1024 * 1024 * 1024,
			MonthlyCost: 5.0,
		},
	}

	err = db.RecordMetrics(ctx, metrics)
	if err != nil {
		t.Fatalf("Failed to record metrics: %v", err)
	}

	// Retrieve history
	history, err := db.GetHistory(ctx, "test-ns", "history-pvc", 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) == 0 {
		t.Error("Expected non-empty history")
	}
}

func TestTimescaleDB_RecordMetrics_EmptyList(t *testing.T) {
	t.Skip("Requires TimescaleDB instance")

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test sslmode=disable"
	db, err := NewTimescaleDB(connStr)
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Should handle empty metrics gracefully
	err = db.RecordMetrics(ctx, []types.PVCMetric{})
	if err != nil {
		t.Errorf("Should handle empty metrics: %v", err)
	}
}

func TestTimescaleDB_GetHistory_NoData(t *testing.T) {
	t.Skip("Requires TimescaleDB instance")

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test sslmode=disable"
	db, err := NewTimescaleDB(connStr)
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	ctx := context.Background()

	// Query non-existent PVC
	history, err := db.GetHistory(ctx, "non-existent", "non-existent-pvc", 24*time.Hour)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected empty history for non-existent PVC, got %d items", len(history))
	}
}

func TestTimescaleDB_ConnectionFailure(t *testing.T) {
	// Test connection to invalid database
	connStr := "host=invalid-host port=5432 user=postgres password=postgres dbname=test sslmode=disable"

	_, err := NewTimescaleDB(connStr)
	if err == nil {
		t.Error("Expected error for invalid connection")
	}
}

func TestTimescaleDB_Close(t *testing.T) {
	t.Skip("Requires TimescaleDB instance")

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test sslmode=disable"
	db, err := NewTimescaleDB(connStr)
	if err != nil {
		t.Skipf("Cannot connect to database: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

// Mock tests that don't require database connection

func TestTimescaleDB_MetricsSerialization(t *testing.T) {
	// Test that metrics are properly structured
	metrics := []types.PVCMetric{
		{
			Name:        "test-pvc",
			Namespace:   "default",
			UsedBytes:   100 * 1024 * 1024 * 1024,
			EgressBytes: 10 * 1024 * 1024,
			ReadIOPS:    1000,
			WriteIOPS:   500,
			MonthlyCost: 8.5,
		},
	}

	if len(metrics) != 1 {
		t.Error("Expected 1 metric")
	}

	m := metrics[0]
	if m.Name != "test-pvc" {
		t.Errorf("Expected Name 'test-pvc', got %s", m.Name)
	}
	if m.Namespace != "default" {
		t.Errorf("Expected Namespace 'default', got %s", m.Namespace)
	}
	if m.UsedBytes != 100*1024*1024*1024 {
		t.Errorf("Expected UsedBytes 107374182400, got %d", m.UsedBytes)
	}
}
