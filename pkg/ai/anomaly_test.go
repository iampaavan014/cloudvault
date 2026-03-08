package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnomalyEngine_ScoreVolume(t *testing.T) {
	e := NewAnomalyEngine(0.05)

	// Test insufficient history
	historySmall := []float64{0.1, 0.2, 0.3}
	assert.Equal(t, 0.0, e.ScoreVolume(historySmall, 0.4))

	// Test anomaly score
	history := []float64{0.1, 0.11, 0.12, 0.1, 0.11, 0.12, 0.1, 0.11}
	scoreNormal := e.ScoreVolume(history, 0.11)
	scoreAnomaly := e.ScoreVolume(history, 0.9)

	assert.Greater(t, scoreAnomaly, scoreNormal)
}

func TestAnomalyEngine_IsZombie(t *testing.T) {
	e := NewAnomalyEngine(0.05)

	// Zombie history
	zombie := []float64{0.01, 0.02, 0.0, 0.04, 0.01}
	assert.True(t, e.IsZombie(zombie))

	// Active history
	active := []float64{0.01, 0.02, 0.1, 0.04, 0.01}
	assert.False(t, e.IsZombie(active))

	// Empty history
	assert.False(t, e.IsZombie([]float64{}))
}

func TestAnomalyEngine_DetectCostSpike(t *testing.T) {
	e := NewAnomalyEngine(0.05)

	assert.True(t, e.DetectCostSpike(100.0, 30.0))
	assert.False(t, e.DetectCostSpike(40.0, 30.0))
	assert.False(t, e.DetectCostSpike(100.0, 0.0))
}
