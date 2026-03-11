package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── NewMigrationExecutor ──────────────────────────────────────────────────────

func TestNewMigrationExecutor_NilConfig(t *testing.T) {
	exec, err := NewMigrationExecutor(nil, "argo-system")
	require.NoError(t, err)
	assert.NotNil(t, exec)
	assert.Equal(t, "argo-system", exec.argoNamespace)
	assert.NotNil(t, exec.migrations)
}

// ── selectMigrationStrategy ──────────────────────────────────────────────────

func TestSelectMigrationStrategy_Velero(t *testing.T) {
	exec := &MigrationExecutor{veleroEnabled: true}
	assert.Equal(t, "velero-backup-restore", exec.selectMigrationStrategy())
}

func TestSelectMigrationStrategy_NoVelero(t *testing.T) {
	exec := &MigrationExecutor{veleroEnabled: false}
	assert.Equal(t, "volume-clone", exec.selectMigrationStrategy())
}

// ── assessRisk ───────────────────────────────────────────────────────────────

func TestAssessRisk_LargeTransfer(t *testing.T) {
	exec := &MigrationExecutor{}
	impact := MigrationImpact{DataTransferSize: 200 * 1024 * 1024 * 1024}
	assert.Equal(t, "high", exec.assessRisk(types.Recommendation{}, impact))
}

func TestAssessRisk_ManyWorkloads(t *testing.T) {
	exec := &MigrationExecutor{}
	impact := MigrationImpact{
		DataTransferSize:  1024,
		AffectedWorkloads: []string{"a", "b", "c", "d", "e", "f"},
	}
	assert.Equal(t, "high", exec.assessRisk(types.Recommendation{}, impact))
}

func TestAssessRisk_ResizeMedium(t *testing.T) {
	exec := &MigrationExecutor{}
	impact := MigrationImpact{DataTransferSize: 1024}
	assert.Equal(t, "medium", exec.assessRisk(types.Recommendation{Type: "resize"}, impact))
}

func TestAssessRisk_Low(t *testing.T) {
	exec := &MigrationExecutor{}
	impact := MigrationImpact{DataTransferSize: 1024}
	assert.Equal(t, "low", exec.assessRisk(types.Recommendation{Type: "storage_class"}, impact))
}

// ── calculateImpact ──────────────────────────────────────────────────────────

func TestCalculateImpact_NilClient(t *testing.T) {
	exec := &MigrationExecutor{sourceClient: nil}
	pvcs := []types.PVCMetric{
		{Name: "pvc-1", Namespace: "default", UsedBytes: 5 * 1024 * 1024 * 1024},
	}
	impact := exec.calculateImpact(context.Background(), pvcs)
	assert.Equal(t, int64(5*1024*1024*1024), impact.DataTransferSize)
	assert.Equal(t, "low", impact.RiskLevel)
	assert.Equal(t, 5*time.Minute, impact.DowntimeRequired)
}

func TestCalculateImpact_Empty(t *testing.T) {
	exec := &MigrationExecutor{}
	impact := exec.calculateImpact(context.Background(), nil)
	assert.Equal(t, int64(0), impact.DataTransferSize)
	assert.Empty(t, impact.AffectedWorkloads)
}

// ── estimateMigrationDuration ────────────────────────────────────────────────

func TestEstimateMigrationDuration_WithData(t *testing.T) {
	pvcs := []types.PVCMetric{
		{UsedBytes: 5 * 1024 * 1024 * 1024},
		{UsedBytes: 10 * 1024 * 1024 * 1024},
	}
	assert.Equal(t, 15*time.Minute, estimateMigrationDuration(pvcs))
}

func TestEstimateMigrationDuration_Empty(t *testing.T) {
	assert.Equal(t, time.Duration(0), estimateMigrationDuration(nil))
}

// ── extractTargetFromRecommendation ─────────────────────────────────────────

func TestExtractTargetFromRecommendation(t *testing.T) {
	rec := types.Recommendation{Type: "move_cloud", RecommendedState: "gcp (us-central1)"}
	assert.NotEmpty(t, extractTargetFromRecommendation(rec))
}

// ── buildMigrationSteps ──────────────────────────────────────────────────────

func TestBuildMigrationSteps_VolumeClone(t *testing.T) {
	exec := &MigrationExecutor{veleroEnabled: false}
	plan := &MigrationPlan{Strategy: "volume-clone"}
	steps := exec.buildMigrationSteps(plan)
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	assert.Contains(t, names, "pre-flight-check")
	assert.Contains(t, names, "copy-data")
}

func TestBuildMigrationSteps_Velero(t *testing.T) {
	exec := &MigrationExecutor{veleroEnabled: true}
	plan := &MigrationPlan{Strategy: "velero-backup-restore"}
	steps := exec.buildMigrationSteps(plan)
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	assert.Contains(t, names, "create-velero-backup")
	assert.Contains(t, names, "restore-to-target")
}

// ── GetMigrationStatus / GetAllMigrations / GetActiveMigrations ──────────────

func TestGetMigrationStatus_Found(t *testing.T) {
	exec := &MigrationExecutor{migrations: map[string]*MigrationStatus{
		"mig-1": {State: "completed"},
	}}
	s, err := exec.GetMigrationStatus("mig-1")
	require.NoError(t, err)
	assert.Equal(t, "completed", s.State)
}

func TestGetMigrationStatus_NotFound(t *testing.T) {
	exec := &MigrationExecutor{migrations: map[string]*MigrationStatus{}}
	_, err := exec.GetMigrationStatus("nonexistent")
	assert.Error(t, err)
}

func TestGetAllMigrations_Multiple(t *testing.T) {
	exec := &MigrationExecutor{migrations: map[string]*MigrationStatus{
		"a": {State: "completed"},
		"b": {State: "backing-up"},
	}}
	assert.Len(t, exec.GetAllMigrations(), 2)
}

func TestGetActiveMigrations_FiltersCompleted(t *testing.T) {
	exec := &MigrationExecutor{migrations: map[string]*MigrationStatus{
		"mig-1": {State: "completed"},
		"mig-2": {State: "backing-up"},
		"mig-3": {State: "failed"},
		"mig-4": {State: "pending"},
	}}
	active := exec.GetActiveMigrations()
	for _, s := range active {
		assert.NotEqual(t, "completed", s.State)
		assert.NotEqual(t, "failed", s.State)
	}
	assert.Len(t, active, 2)
}

// ── CreateMigrationPlan ──────────────────────────────────────────────────────

func TestCreateMigrationPlan_Basic(t *testing.T) {
	exec := &MigrationExecutor{migrations: map[string]*MigrationStatus{}}
	rec := types.Recommendation{
		Type:             "resize",
		PVC:              "my-pvc",
		RecommendedState: "50Gi",
		MonthlySavings:   10.0,
	}
	pvcs := []types.PVCMetric{
		{Name: "my-pvc", Namespace: "default", UsedBytes: 20 * 1024 * 1024 * 1024},
	}
	plan, err := exec.CreateMigrationPlan(context.Background(), rec, pvcs)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.NotEmpty(t, plan.ID)
	assert.Equal(t, "50Gi", plan.TargetClass)
	assert.InDelta(t, 10.0, plan.EstimatedSavings, 0.01)
}

// ── FormatQuantity ───────────────────────────────────────────────────────────

func TestFormatQuantity_GB(t *testing.T) {
	assert.Equal(t, "100Gi", FormatQuantity(100*1024*1024*1024))
}

func TestFormatQuantity_MB(t *testing.T) {
	assert.Equal(t, "512Mi", FormatQuantity(512*1024*1024))
}

func TestFormatQuantity_Bytes(t *testing.T) {
	assert.Equal(t, "1024", FormatQuantity(1024))
}

// ── buildArgoWorkflow ────────────────────────────────────────────────────────

func TestBuildArgoWorkflow_Structure(t *testing.T) {
	exec := &MigrationExecutor{argoNamespace: "argo"}
	plan := &MigrationPlan{ID: "test-123", PVCs: []types.PVCMetric{{Namespace: "default"}}}
	wf := exec.buildArgoWorkflow(plan)
	assert.NotNil(t, wf)
	assert.Contains(t, wf.GetName(), "migration-test-123")
}

// ── LifecycleController — SetPVCCollector ────────────────────────────────────

func TestLifecycleController_SetPVCCollector_Nil(t *testing.T) {
	lc := NewLifecycleController(0, nil, nil, nil, nil)
	lc.SetPVCCollector(nil) // must not panic
}
