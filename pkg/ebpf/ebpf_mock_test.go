//go:build !linux

package ebpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockAgent(t *testing.T) {
	a := NewMockAgent()
	require.NotNil(t, a)
}

func TestMockAgent_Close(t *testing.T) {
	a := NewMockAgent()
	assert.NoError(t, a.Close())
}

func TestMockAgent_GetEgressStats(t *testing.T) {
	a := NewMockAgent()
	stats, err := a.GetEgressStats()
	require.NoError(t, err)
	assert.NotEmpty(t, stats)
	for src, dsts := range stats {
		assert.NotEmpty(t, src)
		for dst, b := range dsts {
			assert.NotEmpty(t, dst)
			assert.Greater(t, b, uint64(0))
		}
	}
}

func TestMockAgent_AttachToInterface(t *testing.T) {
	a := NewMockAgent()
	_, err := a.AttachToInterface("eth0")
	assert.Error(t, err) // always errors on non-linux
}
