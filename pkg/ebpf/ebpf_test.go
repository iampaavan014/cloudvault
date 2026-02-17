package ebpf

import (
	"testing"
)

func TestNewAgent(t *testing.T) {
	agent, err := NewAgent()
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	if agent == nil {
		t.Fatal("Agent is nil")
	}
}

func TestGetEgressStats(t *testing.T) {
	agent, _ := NewAgent()
	stats, err := agent.GetEgressStats()
	if err != nil {
		t.Fatalf("Failed to get egress stats: %v", err)
	}

	if len(stats) == 0 {
		t.Error("Expected at least one egress entry")
	}

	if val, ok := stats["10.0.1.5"]; !ok || val == 0 {
		t.Errorf("Expected stat for 10.0.1.5, got %v", val)
	}
}
