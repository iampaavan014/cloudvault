package collector

import (
	"context"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/ebpf"
	"github.com/cloudvault-io/cloudvault/pkg/integrations"
	"github.com/cloudvault-io/cloudvault/pkg/types"
)

func TestPrometheusEgressProvider_GetEgressBytes(t *testing.T) {
	provider := &PrometheusEgressProvider{}

	ctx := context.Background()
	data, err := provider.GetEgressBytes(ctx)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if data == nil {
		t.Error("Expected non-nil egress data")
	}
}

func TestNewEbpfEgressProvider(t *testing.T) {
	agent := ebpf.NewMockAgent()
	provider := NewEbpfEgressProvider(agent)

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	if provider.agent == nil {
		t.Error("Expected non-nil agent")
	}
}

func TestEbpfEgressProvider_GetEgressBytes_WithAgent(t *testing.T) {
	agent := ebpf.NewMockAgent()
	provider := NewEbpfEgressProvider(agent)

	ctx := context.Background()
	data, err := provider.GetEgressBytes(ctx)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if data == nil {
		t.Error("Expected non-nil egress data")
	}
}

func TestEbpfEgressProvider_GetEgressBytes_NoAgent(t *testing.T) {
	provider := NewEbpfEgressProvider(nil)

	ctx := context.Background()
	data, err := provider.GetEgressBytes(ctx)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if data == nil {
		t.Error("Expected non-nil egress data")
	}
}

func TestCorrelateEgress_WithoutPodIP(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "test-pvc",
			Namespace: "default",
			Labels:    make(map[string]string),
		},
	}

	egressData := map[string]map[string]uint64{
		"10.0.0.1": {
			"8.8.8.8": 1000000,
		},
	}

	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	// Without pod IP, egress should remain 0
	if metrics[0].EgressBytes != 0 {
		t.Errorf("Expected 0 egress bytes, got %d", metrics[0].EgressBytes)
	}
}

func TestCorrelateEgress_WithPodIP(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "test-pvc",
			Namespace: "default",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.1",
			},
		},
	}

	egressData := map[string]map[string]uint64{
		"10.0.0.1": {
			"8.8.8.8":  1000000, // External
			"10.0.0.2": 500000,  // Internal
		},
	}

	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	expectedTotal := uint64(1500000)
	if metrics[0].EgressBytes != expectedTotal {
		t.Errorf("Expected %d egress bytes, got %d", expectedTotal, metrics[0].EgressBytes)
	}

	// Check if egress label was set
	if egressLabel, ok := metrics[0].Labels["cloudvault.io/egress-bytes"]; !ok {
		t.Error("Expected egress-bytes label to be set")
	} else if egressLabel != "1500000" {
		t.Errorf("Expected egress-bytes label '1500000', got '%s'", egressLabel)
	}
}

func TestCorrelateEgress_ExternalEgressDetection(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "test-pvc",
			Namespace: "default",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.1",
			},
		},
	}

	egressData := map[string]map[string]uint64{
		"10.0.0.1": {
			"8.8.8.8": 1000000, // Public IP - should be detected as external
		},
	}

	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	// Check if external egress was detected
	if externalLabel, ok := metrics[0].Labels["cloudvault.io/external-egress"]; ok {
		if externalLabel != "true" {
			t.Errorf("Expected external-egress 'true', got '%s'", externalLabel)
		}
	}
}

func TestCorrelateEgress_MultipleDestinations(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "test-pvc",
			Namespace: "default",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.1",
			},
		},
	}

	egressData := map[string]map[string]uint64{
		"10.0.0.1": {
			"8.8.8.8":    1000000,
			"1.1.1.1":    2000000,
			"52.94.0.10": 3000000, // AWS IP range
			"10.0.0.100": 500000,  // Internal
		},
	}

	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	expectedTotal := uint64(6500000)
	if metrics[0].EgressBytes != expectedTotal {
		t.Errorf("Expected %d egress bytes, got %d", expectedTotal, metrics[0].EgressBytes)
	}
}

func TestCorrelateEgress_NoMatchingIP(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "test-pvc",
			Namespace: "default",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.99",
			},
		},
	}

	egressData := map[string]map[string]uint64{
		"10.0.0.1": {
			"8.8.8.8": 1000000,
		},
	}

	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	// No matching IP, should remain 0
	if metrics[0].EgressBytes != 0 {
		t.Errorf("Expected 0 egress bytes, got %d", metrics[0].EgressBytes)
	}
}

func TestCorrelateEgress_EmptyEgressData(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "test-pvc",
			Namespace: "default",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.1",
			},
		},
	}

	egressData := map[string]map[string]uint64{}
	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	// Empty data, should remain 0
	if metrics[0].EgressBytes != 0 {
		t.Errorf("Expected 0 egress bytes, got %d", metrics[0].EgressBytes)
	}
}

func TestCorrelateEgress_MultiplePVCs(t *testing.T) {
	metrics := []types.PVCMetric{
		{
			Name:      "pvc-1",
			Namespace: "default",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.1",
			},
		},
		{
			Name:      "pvc-2",
			Namespace: "production",
			Labels: map[string]string{
				"cloudvault.io/pod-ip": "10.0.0.2",
			},
		},
	}

	egressData := map[string]map[string]uint64{
		"10.0.0.1": {
			"8.8.8.8": 1000000,
		},
		"10.0.0.2": {
			"1.1.1.1": 2000000,
		},
	}

	resolver := integrations.NewRegionResolver()

	CorrelateEgress(metrics, egressData, resolver)

	if metrics[0].EgressBytes != 1000000 {
		t.Errorf("PVC-1: Expected 1000000 egress bytes, got %d", metrics[0].EgressBytes)
	}

	if metrics[1].EgressBytes != 2000000 {
		t.Errorf("PVC-2: Expected 2000000 egress bytes, got %d", metrics[1].EgressBytes)
	}
}
