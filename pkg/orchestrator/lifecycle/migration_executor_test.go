package lifecycle

import (
	"context"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestMigrationExecutor_CalculateImpact(t *testing.T) {
	executor, _ := NewMigrationExecutor(nil, "default")

	pvcs := []types.PVCMetric{
		{Name: "pvc-1", UsedBytes: 1024 * 1024},
		{Name: "pvc-2", UsedBytes: 2048 * 1024},
	}

	impact := executor.calculateImpact(context.Background(), pvcs)
	assert.Equal(t, int64(3072*1024), impact.DataTransferSize)
	assert.Equal(t, "low", impact.RiskLevel)
}

func TestMigrationExecutor_SelectMigrationStrategy(t *testing.T) {
	executor, _ := NewMigrationExecutor(nil, "default")

	// Test without Velero (client is nil)
	strategy := executor.selectMigrationStrategy()
	assert.Equal(t, "volume-clone", strategy) // Correct return value
}

func TestMigrationExecutor_EstimateMigrationDuration(t *testing.T) {
	duration := estimateMigrationDuration([]types.PVCMetric{{UsedBytes: 10 * 1024 * 1024 * 1024}})
	assert.Equal(t, 10, int(duration.Minutes()))
}

func TestMigrationExecutor_ExtractTargetFromRecommendation(t *testing.T) {
	rec := types.Recommendation{RecommendedState: "gp3"}
	target := extractTargetFromRecommendation(rec)
	assert.Equal(t, "optimized-cluster", target)
}

func TestMigrationExecutor_BuildMigrationSteps(t *testing.T) {
	executor, _ := NewMigrationExecutor(nil, "default")
	plan := &MigrationPlan{
		Strategy: "volume-clone",
	}
	steps := executor.buildMigrationSteps(plan)
	assert.Greater(t, len(steps), 0)
}
