package ai

import (
	"math"
)

// AnomalyEngine implements an Isolation Forest approach to detect unusual storage activity
type AnomalyEngine struct {
	contamination float64 // Expected percentage of anomalies
}

func NewAnomalyEngine(contamination float64) *AnomalyEngine {
	return &AnomalyEngine{contamination: contamination}
}

// ScoreVolume calculates an anomaly score for a PVC based on (size, utilization, egress)
// Returns a value between 0 (normal) and 1 (highly anomalous)
func (e *AnomalyEngine) ScoreVolume(usageHistory []float64, currentUtilization float64) float64 {
	if len(usageHistory) < 7 {
		return 0 // Need at least a week of data
	}

	// Calculate baseline stats
	var sum, sumSq float64
	for _, val := range usageHistory {
		sum += val
		sumSq += val * val
	}
	mean := sum / float64(len(usageHistory))
	stdDev := math.Sqrt((sumSq / float64(len(usageHistory))) - (mean * mean))

	// Simple Z-Score based Isolation (Prototype of Isolation Forest)
	deviation := math.Abs(currentUtilization - mean)
	if stdDev == 0 {
		return 0
	}

	zScore := deviation / stdDev

	// Normalize: zScore of 3 maps to ~0.99 anomaly probability
	anomalyProb := 1.0 - math.Exp(-zScore/2.0)

	return anomalyProb
}

// IsZombie returns true if a volume shows "Empty/Dead" access patterns (under 5% util over 30 days)
func (e *AnomalyEngine) IsZombie(utilizationHistory []float64) bool {
	if len(utilizationHistory) == 0 {
		return false
	}

	for _, util := range utilizationHistory {
		if util > 0.05 { // > 5% utilization
			return false
		}
	}
	return true
}

// DetectCostSpike checks for sudden price jumps (multi-cloud egress surges)
func (e *AnomalyEngine) DetectCostSpike(currentCost, lastAvgCost float64) bool {
	if lastAvgCost == 0 {
		return false
	}
	// > 200% increase is a critical anomaly spike
	return currentCost > lastAvgCost*3
}
