package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/cloudvault-io/cloudvault/pkg/types/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
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
