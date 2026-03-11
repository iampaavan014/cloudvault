package collector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── MockPVCCollector ──────────────────────────────────────────────────────────

func TestNewMockPVCCollector(t *testing.T) {
	m := NewMockPVCCollector()
	require.NotNil(t, m)
}

func TestMockPVCCollector_CollectAll(t *testing.T) {
	m := NewMockPVCCollector()
	metrics, err := m.CollectAll(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
	for _, mv := range metrics {
		assert.NotEmpty(t, mv.Name)
	}
}

func TestMockPVCCollector_CollectByNamespace_Match(t *testing.T) {
	m := NewMockPVCCollector()
	metrics, err := m.CollectByNamespace(context.Background(), "default")
	require.NoError(t, err)
	for _, mv := range metrics {
		assert.Equal(t, "default", mv.Namespace)
	}
}

func TestMockPVCCollector_CollectByNamespace_NoMatch(t *testing.T) {
	m := NewMockPVCCollector()
	metrics, err := m.CollectByNamespace(context.Background(), "nonexistent-ns")
	require.NoError(t, err)
	assert.Empty(t, metrics)
}

// ── PVCCollector — nil client guards ─────────────────────────────────────────

func TestPVCCollector_CollectAll_NilClient(t *testing.T) {
	c := NewPVCCollector(nil, nil)
	_, err := c.CollectAll(context.Background())
	assert.Error(t, err)
}

func TestPVCCollector_GetPVCCount_NilClient(t *testing.T) {
	c := NewPVCCollector(nil, nil)
	_, err := c.GetPVCCount(context.Background())
	assert.Error(t, err)
}

func TestPVCCollector_GetPVCsByStorageClass_NilClient(t *testing.T) {
	c := NewPVCCollector(nil, nil)
	_, err := c.GetPVCsByStorageClass(context.Background(), "gp3")
	assert.Error(t, err)
}

func TestPVCCollector_GetNamespaces_NilClient(t *testing.T) {
	c := NewPVCCollector(nil, nil)
	_, err := c.GetNamespaces(context.Background())
	assert.Error(t, err)
}

func TestPVCCollector_SetEgressProvider(t *testing.T) {
	c := NewPVCCollector(nil, nil)
	p := &PrometheusEgressProvider{}
	c.SetEgressProvider(p)
	assert.NotNil(t, c.egressProvider)
}
