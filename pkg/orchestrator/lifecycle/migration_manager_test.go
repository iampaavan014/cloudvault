package lifecycle

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrationManager_Basic(t *testing.T) {
	manager := NewMigrationManager(nil)
	assert.NotNil(t, manager)

	// Test recording action
	manager.recordAction("default", "pvc-1", "hot", "cold", "completed")
	history := manager.GetHistory()
	assert.Equal(t, 1, len(history))
	assert.Equal(t, "pvc-1", history[0].PVC)

	// Test status
	status := manager.GetMigrationStatus("default", "pvc-1")
	assert.Equal(t, "Succeeded", status)
}

func TestMigrationManager_ExecuteMigrationConflict(t *testing.T) {
	manager := NewMigrationManager(nil)

	// Manually inject an active job
	manager.activeJobs["default/pvc-1"] = true

	err := manager.ExecuteMigration(context.Background(), "default", "pvc-1", "warm")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestMigrationManager_HistoryLimit(t *testing.T) {
	manager := NewMigrationManager(nil)
	for i := 0; i < 150; i++ {
		manager.recordAction("default", "pvc", "hot", "cold", "completed")
	}
	history := manager.GetHistory()
	assert.Equal(t, 100, len(history))
}
