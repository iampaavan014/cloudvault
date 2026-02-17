package ai

import (
	"math/rand"
)

// PlacementEnv represents a simplified RL environment for PVC placement
type PlacementEnv struct {
	AvailableClasses []string
	CostMatrix       map[string]float64
	Performance      map[string]float64
}

// QTable stores the learned values for (workload_type, storage_class) pairs
type QTable map[string]map[string]float64

// RLAgent implements a simple Q-Learning agent for storage placement
type RLAgent struct {
	qTable          QTable
	learningRate    float64
	discountFactor  float64
	explorationRate float64
}

func NewRLAgent() *RLAgent {
	return &RLAgent{
		qTable:          make(QTable),
		learningRate:    0.1,
		discountFactor:  0.9,
		explorationRate: 0.2,
	}
}

// DecidePlacement chooses the best storage class for a workload profile
func (a *RLAgent) DecidePlacement(workloadType string, availableClasses []string) string {
	// Initialize workload in Q-table if new
	if _, ok := a.qTable[workloadType]; !ok {
		a.qTable[workloadType] = make(map[string]float64)
		for _, class := range availableClasses {
			a.qTable[workloadType][class] = 0.0
		}
	}

	// Exploration (ε-greedy)
	if rand.Float64() < a.explorationRate {
		return availableClasses[rand.Intn(len(availableClasses))]
	}

	// Exploitation
	bestClass := availableClasses[0]
	maxQ := -1e9
	for _, class := range availableClasses {
		if q := a.qTable[workloadType][class]; q > maxQ {
			maxQ = q
			bestClass = class
		}
	}

	return bestClass
}

// Reward allows the agent to learn from the results of a placement
func (a *RLAgent) Reward(workloadType, class string, reward float64) {
	oldQ := a.qTable[workloadType][class]
	// Q-Learning update rule (simplified)
	a.qTable[workloadType][class] = oldQ + a.learningRate*(reward-oldQ)
}
