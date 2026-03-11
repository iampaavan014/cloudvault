package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLifecycleController_GetStatus(t *testing.T) {
	c := &LifecycleController{
		policies: []v1alpha1.StorageLifecyclePolicy{{}},
	}
	status := c.GetStatus()
	assert.Equal(t, 1, status.ActivePolicies)
	assert.False(t, status.SIGEnabled)
}

func TestLifecycleController_SetPolicies(t *testing.T) {
	c := &LifecycleController{}
	policies := []v1alpha1.StorageLifecyclePolicy{{}}
	c.SetPolicies(policies)
	assert.Equal(t, 1, len(c.policies))
}

func TestLifecycleController_ProcessOptimization_NoCollector(t *testing.T) {
	c := &LifecycleController{}
	err := c.processOptimization(context.Background())
	assert.NoError(t, err) // Should just warn and return nil
}

func TestLifecycleController_SyncPodRelationships_NilSIG(t *testing.T) {
	c := &LifecycleController{sig: nil}
	metrics := []types.PVCMetric{{Name: "pvc-1"}}
	err := c.syncPodRelationships(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestLifecycleController_LifecycleStatus(t *testing.T) {
	c := &LifecycleController{
		policies: []v1alpha1.StorageLifecyclePolicy{{}},
	}
	status := c.GetStatus()
	assert.Equal(t, 1, status.ActivePolicies)
}

func TestNewLifecycleController(t *testing.T) {
	c := NewLifecycleController(1*time.Minute, nil, nil, nil, nil)
	assert.NotNil(t, c)
	assert.NotNil(t, c.manager)
}

func TestLifecycleController_GetManager(t *testing.T) {
	c := NewLifecycleController(1*time.Minute, nil, nil, nil, nil)
	assert.NotNil(t, c.GetManager())
}

// ── MigrationManager ─────────────────────────────────────────────────────────

func TestMigrationManager_ExecuteMigration_NilClient(t *testing.T) {
	mgr := NewMigrationManager(nil) // nil kube client → skips patch, still records action
	err := mgr.ExecuteMigration(context.Background(), "default", "pvc-1", "sc1")
	assert.NoError(t, err)
}

func TestMigrationManager_ExecuteMigration_AlreadyRunning(t *testing.T) {
	mgr := NewMigrationManager(nil)
	mgr.activeJobs["default/pvc-1"] = true
	err := mgr.ExecuteMigration(context.Background(), "default", "pvc-1", "sc1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestMigrationManager_GetMigrationStatus_Active(t *testing.T) {
	mgr := NewMigrationManager(nil)
	mgr.activeJobs["default/pvc-1"] = true
	status := mgr.GetMigrationStatus("default", "pvc-1")
	assert.Contains(t, status, "Running")
}

func TestMigrationManager_GetMigrationStatus_Succeeded(t *testing.T) {
	mgr := NewMigrationManager(nil)
	status := mgr.GetMigrationStatus("default", "pvc-x")
	assert.Equal(t, "Succeeded", status)
}

func TestMigrationManager_GetHistory_AfterAction(t *testing.T) {
	mgr := NewMigrationManager(nil)
	_ = mgr.ExecuteMigration(context.Background(), "default", "pvc-1", "sc1")
	history := mgr.GetHistory()
	require.Len(t, history, 1)
	assert.Equal(t, "pvc-1", history[0].PVC)
	assert.Equal(t, "sc1", history[0].ToTier)
	assert.Equal(t, "completed", history[0].Status)
}

func TestMigrationManager_RecordAction_CapsAt100(t *testing.T) {
	mgr := NewMigrationManager(nil)
	for i := 0; i < 110; i++ {
		mgr.recordAction("default", "pvc", "old", "new", "completed")
	}
	assert.Len(t, mgr.GetHistory(), 100)
}

// ── LifecycleController.processOptimization ──────────────────────────────────

func TestLifecycleController_ProcessOptimization_WithPolicies(t *testing.T) {
	lc := NewLifecycleController(0, nil, nil, nil, nil)
	lc.SetPolicies([]v1alpha1.StorageLifecyclePolicy{
		{Spec: v1alpha1.StorageLifecyclePolicySpec{
			Selector: v1alpha1.PolicySelector{MatchNamespaces: []string{"default"}},
		}},
	})
	err := lc.processOptimization(context.Background())
	assert.NoError(t, err)
}

func TestLifecycleController_SyncPodRelationships_NoPods(t *testing.T) {
	lc := NewLifecycleController(0, nil, nil, nil, nil)
	metrics := []types.PVCMetric{
		{Name: "pvc-1", Namespace: "default", MountedPods: nil},
	}
	err := lc.syncPodRelationships(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestLifecycleController_Start_CancelImmediate(t *testing.T) {
	lc := NewLifecycleController(100*time.Millisecond, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := lc.Start(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
