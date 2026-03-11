//go:build linux

package ebpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntToIP(t *testing.T) {
	// 127.0.0.1 in little-endian uint32
	// IP bytes: [127, 0, 0, 1]
	// LittleEndian.Uint32([127, 0, 0, 1]) = 1*2^24 + 0*2^16 + 0*2^8 + 127 = 16777343
	val := uint32(16777343)
	ip := intToIP(val)
	assert.Equal(t, "127.0.0.1", ip.String())

	// 8.8.8.8
	// LittleEndian.Uint32([8, 8, 8, 8]) = 0x08080808 = 134744072
	ip2 := intToIP(uint32(134744072))
	assert.Equal(t, "8.8.8.8", ip2.String())
}

func TestAgent_Close_Nil(t *testing.T) {
	var a *Agent
	assert.NoError(t, a.Close())
}

func TestAgent_Stats_Nil(t *testing.T) {
	var a *Agent
	_, err := a.GetEgressStats()
	assert.Error(t, err)
}
