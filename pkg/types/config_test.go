package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 8080, config.DashboardPort)
	assert.Equal(t, "aws", config.Provider)
}
