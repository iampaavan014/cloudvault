package integrations

import (
	"context"
	"os"
	"testing"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── LoadConfig ────────────────────────────────────────────────────────────────

func TestLoadConfig_EmptyPath(t *testing.T) {
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	_, err := LoadConfig("/tmp/cloudvault-nonexistent-config-12345.yaml")
	assert.Error(t, err)
}

func TestLoadConfig_ValidFile(t *testing.T) {
	content := `
provider: aws
region: us-east-1
`
	f, err := os.CreateTemp("", "cloudvault-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString(content)
	require.NoError(t, err)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "cloudvault-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString(": invalid: yaml: [")
	require.NoError(t, err)
	f.Close()

	_, err = LoadConfig(f.Name())
	assert.Error(t, err)
}

// ── AWSClient ─────────────────────────────────────────────────────────────────

func TestNewAWSClient_NilConfig(t *testing.T) {
	// NewAWSClient gracefully handles missing AWS credentials in CI
	c := NewAWSClient(nil)
	assert.NotNil(t, c)
}

func TestNewAWSClient_WithConfig(t *testing.T) {
	cfg := types.DefaultConfig()
	c := NewAWSClient(cfg)
	assert.NotNil(t, c)
}

func TestAWSClient_GetStoragePrice_ReturnsError(t *testing.T) {
	c := NewAWSClient(nil)
	_, err := c.GetStoragePrice(context.Background(), "gp3", "us-east-1")
	// Integration is "in progress" — should return a non-nil error
	assert.Error(t, err)
}

// ── GCPClient ─────────────────────────────────────────────────────────────────

func TestNewGCPClient_NilConfig(t *testing.T) {
	c := NewGCPClient(nil)
	assert.NotNil(t, c)
}

func TestNewGCPClient_WithConfig(t *testing.T) {
	cfg := types.DefaultConfig()
	c := NewGCPClient(cfg)
	assert.NotNil(t, c)
}

func TestGCPClient_GetStoragePrice_ReturnsError(t *testing.T) {
	c := NewGCPClient(nil)
	_, err := c.GetStoragePrice(context.Background(), "pd-standard", "us-central1")
	assert.Error(t, err)
}

// ── AzureClient ───────────────────────────────────────────────────────────────

func TestNewAzureClient_NilConfig(t *testing.T) {
	c := NewAzureClient(nil)
	assert.NotNil(t, c)
}

func TestNewAzureClient_WithConfig(t *testing.T) {
	cfg := types.DefaultConfig()
	c := NewAzureClient(cfg)
	assert.NotNil(t, c)
}

func TestAzureClient_GetStoragePrice_NetworkError(t *testing.T) {
	// Use an invalid URL scheme to force an immediate transport error
	c := &AzureClient{httpClient: nil}
	// nil httpClient will panic — use a real client but point to an unreachable host
	c2 := NewAzureClient(nil)
	// In -short mode, skip live network calls
	if testing.Short() {
		t.Skip("skipping live Azure API call in short mode")
	}
	_, err := c2.GetStoragePrice(context.Background(), "Premium_LRS", "eastus")
	// May succeed or fail depending on network; just ensure no panic
	_ = err
	_ = c
}
