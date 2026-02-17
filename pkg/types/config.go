package types

import (
	"time"
)

// Config represents the CloudVault application configuration
type Config struct {
	KubeConfig    string        `yaml:"kubeconfig" json:"kubeconfig"`
	Interval      time.Duration `yaml:"interval" json:"interval"`
	Namespace     string        `yaml:"namespace" json:"namespace"`
	PrometheusURL string        `yaml:"prometheus_url" json:"prometheus_url"`
	DashboardPort int           `yaml:"dashboard_port" json:"dashboard_port"`
	Provider      string        `yaml:"provider" json:"provider"`
	TimescaleConn string        `yaml:"timescale_conn" json:"timescale_conn"`
	Mock          bool          `yaml:"mock" json:"mock"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Interval:      5 * time.Minute,
		DashboardPort: 8080,
		Provider:      "aws",
	}
}
