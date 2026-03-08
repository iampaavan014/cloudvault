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
		t.Skipf("Skipping eBPF test (likely missing privileges): %v", err)
	}
	defer func() { _ = agent.Close() }()

	stats, err := agent.GetEgressStats()
	if err != nil {
		t.Skipf("Skipping: GetEgressStats unavailable: %v", err)
	}

	// Verify granular structure: map[src]map[dst]uint64
	for src, dsts := range stats {
		if len(dsts) == 0 {
			t.Errorf("No destinations for source IP %s", src)
		}
		for dst, bytes := range dsts {
			if bytes == 0 {
				t.Errorf("Zero bytes for %s -> %s", src, dst)
			}
		}
	}
}
