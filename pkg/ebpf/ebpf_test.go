package ebpf

import (
	"testing"
)

func TestNewAgent(t *testing.T) {
	agent, err := NewAgent()
	if err != nil {
		t.Skipf("Skipping eBPF test (likely missing privileges): %v", err)
	}
	defer func() { _ = agent.Close() }()

	if agent == nil {
		t.Fatal("Agent is nil")
	}
}

func TestGetEgressStats(t *testing.T) {
	agent, err := NewAgent()
	if err != nil {
		// Even if initialization fails, the hardened receiver should not panic
		stats, err := agent.GetEgressStats()
		if err != nil {
			t.Fatalf("Hardened GetEgressStats failed: %v", err)
		}
		if len(stats) == 0 {
			t.Error("Expected at least one egress entry from mock fallback")
		}
		return
	}

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
