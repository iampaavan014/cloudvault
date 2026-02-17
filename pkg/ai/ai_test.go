package ai

import (
	"testing"
)

func TestLSTMCell_PredictNextCost(t *testing.T) {
	cell := NewLSTMCell()

	// Test zero history
	if cost := cell.PredictNextCost([]float64{}); cost != 0 {
		t.Errorf("Expected 0 for empty history, got %f", cost)
	}

	// Test trend
	history := []float64{0.1, 0.2, 0.3}
	cost := cell.PredictNextCost(history)
	if cost <= 0 {
		t.Errorf("Expected positive prediction for upward trend, got %f", cost)
	}
}

func TestRLAgent_DecidePlacement(t *testing.T) {
	agent := NewRLAgent()
	workload := "test_workload"
	classes := []string{"gp3", "sc1"}

	// First decision
	decision := agent.DecidePlacement(workload, classes)
	found := false
	for _, c := range classes {
		if c == decision {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Decision %s not in available classes", decision)
	}

	// Reward and re-test
	agent.Reward(workload, "sc1", 10.0) // Highly positive reward for sc1

	// Force exploitation for testing (temporarily set exploration to 0)
	agent.explorationRate = 0
	decision = agent.DecidePlacement(workload, classes)
	if decision != "sc1" {
		t.Errorf("Expected sc1 after high reward, got %s", decision)
	}
}

func TestCostForecaster_ForecastMonthlySpend(t *testing.T) {
	forecaster := NewCostForecaster()
	current := 100.0
	trend := []float64{0.05, 0.05, 0.05}

	forecast := forecaster.ForecastMonthlySpend(current, trend)
	if forecast <= current {
		t.Errorf("Expected forecast %f to be greater than current %f for positive trend", forecast, current)
	}
}
