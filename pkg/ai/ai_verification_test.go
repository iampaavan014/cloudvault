package ai

import (
	"math/rand"
	"testing"
)

// TestAIVerificationSuite covers the "Rock Solid" Pillar 2 requirements
func TestAIVerificationSuite(t *testing.T) {
	t.Run("LSTM_Forecasting_Accuracy", func(t *testing.T) {
		forecaster := NewCostForecaster()

		// Case 1: Steady 10% growth over 30 days
		history := []float64{0.1, 0.11, 0.12, 0.1, 0.13, 0.15}
		currentSpend := 100.0
		predicted := forecaster.ForecastMonthlySpend(currentSpend, history)

		if predicted <= currentSpend {
			t.Errorf("LSTM failed to predict growth. Current: %.2f, Predicted: %.2f", currentSpend, predicted)
		}

		// Case 2: Sharp decline
		historyDown := []float64{-0.1, -0.2, -0.3, -0.25}
		predictedDown := forecaster.ForecastMonthlySpend(currentSpend, historyDown)
		if predictedDown >= currentSpend {
			t.Errorf("LSTM failed to predict decline. Current: %.2f, Predicted: %.2f", currentSpend, predictedDown)
		}
	})

	t.Run("RL_Optimal_Placement_Learning", func(t *testing.T) {
		agent := NewRLAgent()
		workload := "cold_archive"
		classes := []string{"gp3", "sc1", "io1"}

		// Training loop: Reward sc1 for cold workloads
		for i := 0; i < 100; i++ {
			class := agent.DecidePlacement(workload, classes)
			var reward float64
			if class == "sc1" {
				reward = 10.0 // High reward for sc1
			} else if class == "io1" {
				reward = -10.0 // Penalty for expensive io1
			} else {
				reward = 0.0
			}
			agent.Reward(workload, class, reward)
		}

		// Verification: Agent should now exploit sc1 (after training, lower exploration)
		agent.explorationRate = 0.0 // Set to zero for pure exploitation check
		decision := agent.DecidePlacement(workload, classes)

		if decision != "sc1" {
			t.Errorf("RL Agent failed to learn optimal class. Got: %s, Want: sc1", decision)
		}
	})
}

func TestMain(m *testing.M) {
	rand.Seed(42) // Deterministic for verification
	m.Run()
}
