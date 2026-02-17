package integrations

import (
	"fmt"
	"os"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadConfig reads configuration from a YAML file
func LoadConfig(path string) (*types.Config, error) {
	config := types.DefaultConfig()

	if path == "" {
		return config, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
