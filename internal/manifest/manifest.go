package manifest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest represents the root configuration structure
type Manifest struct {
	Hosts     []Host     `yaml:"hosts"`
	Resources []Resource `yaml:"resources"`
}

// Host represents a target server configuration
type Host struct {
	Address  string `yaml:"address"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// Resource represents a resource to be applied
type Resource struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	Command  string `yaml:"command,omitempty"`
	Content  string `yaml:"content,omitempty"`
	Owner    string `yaml:"owner,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Group    string `yaml:"group,omitempty"`
	Mode     string `yaml:"mode,omitempty"`
	State    string `yaml:"state,omitempty"`
	Enabled  bool   `yaml:"enabled,omitempty"`
	Notifies string `yaml:"notifies,omitempty"`
}

// Load reads and parses a YAML manifest file
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	return &m, nil
}
