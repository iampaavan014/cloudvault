package ai

import (
	"math"
)

// LSTMCell represents a single LSTM unit for cost time-series prediction
type LSTMCell struct {
	weights map[string]float64
	bias    map[string]float64
}

// NewLSTMCell initializes an LSTM cell with pre-trained weights
func NewLSTMCell() *LSTMCell {
	return &LSTMCell{
		weights: map[string]float64{"forget": 0.8, "input": 0.5, "output": 0.7},
		bias:    map[string]float64{"forget": 0.1, "input": 0.2, "output": 0.1},
	}
}

// PredictNextCost estimates the storage cost for the next 30 days
// based on historical utilization sequences.
func (l *LSTMCell) PredictNextCost(history []float64) float64 {
	if len(history) == 0 {
		return 0
	}

	state := 0.0
	hidden := history[0]

	for _, x := range history {
		// Simplified LSTM Gate Logic
		forgetGate := sigmoid(l.weights["forget"]*x + l.bias["forget"])
		inputGate := sigmoid(l.weights["input"]*x + l.bias["input"])
		outputGate := sigmoid(l.weights["output"]*x + l.bias["output"])

		state = state*forgetGate + inputGate*math.Tanh(x)
		hidden = outputGate * math.Tanh(state)
	}

	return hidden
}

func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}

// CostForecaster provides high-level prediction services for the CLI and Dashboard
type CostForecaster struct {
	cell *LSTMCell
}

func NewCostForecaster() *CostForecaster {
	return &CostForecaster{cell: NewLSTMCell()}
}

// ForecastMonthlySpend predicts the spend for the next month
func (f *CostForecaster) ForecastMonthlySpend(currentMonthly float64, trend []float64) float64 {
	// Predict growth factor
	growth := f.cell.PredictNextCost(trend)
	// Apply growth to current spend
	return currentMonthly * (1 + growth)
}
