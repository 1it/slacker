package manifest

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest represents the root configuration structure
type Manifest struct {
	Hosts     []Host     `yaml:"hosts"`
	Resources []Resource `yaml:"resources"`
	Verify    []Verify   `yaml:"verify,omitempty"`
}

// Verify represents a verification check to run after deployment
type Verify struct {
	Name    string `yaml:"name"`             // Human-readable name
	Command string `yaml:"command"`          // Command to run (supports ${HOST} placeholder)
	Expect  string `yaml:"expect,omitempty"` // Expected substring in output
}

// Host represents a target server configuration
type Host struct {
	Address  string `yaml:"address"`
	User     string `yaml:"user"`
	Password string `yaml:"password,omitempty"`
	Key      string `yaml:"key,omitempty"` // Path to SSH private key (auto-detected if empty)
}

// Resource represents a resource to be applied
type Resource struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	Command  string `yaml:"command,omitempty"`
	Unless   string `yaml:"unless,omitempty"`
	Content  string `yaml:"content,omitempty"`
	Owner    string `yaml:"owner,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Group    string `yaml:"group,omitempty"`
	Mode     string `yaml:"mode,omitempty"`
	State    string `yaml:"state,omitempty"`
	Enabled  bool   `yaml:"enabled,omitempty"`
	Notifies string `yaml:"notifies,omitempty"`
	// User resource fields
	UID    int    `yaml:"uid,omitempty"`
	GID    int    `yaml:"gid,omitempty"`
	Home   string `yaml:"home,omitempty"`
	Shell  string `yaml:"shell,omitempty"`
	System bool   `yaml:"system,omitempty"`
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

// Validate checks the manifest for errors before applying
func (m *Manifest) Validate() error {
	var errs []string

	// Validate resources
	for i, r := range m.Resources {
		resourceErrs := validateResource(i, r)
		errs = append(errs, resourceErrs...)
	}

	// Validate hosts (for remote manifests)
	for i, h := range m.Hosts {
		if h.Address == "" {
			errs = append(errs, fmt.Sprintf("host[%d]: address is required", i))
		}
		if h.User == "" {
			errs = append(errs, fmt.Sprintf("host[%d]: user is required", i))
		}
	}

	// Validate verify checks
	for i, v := range m.Verify {
		if v.Name == "" {
			errs = append(errs, fmt.Sprintf("verify[%d]: name is required", i))
		}
		if v.Command == "" {
			errs = append(errs, fmt.Sprintf("verify[%d]: command is required", i))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("manifest validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

// validateResource validates a single resource definition
func validateResource(index int, r Resource) []string {
	var errs []string
	prefix := fmt.Sprintf("resource[%d]", index)

	if r.Type == "" {
		errs = append(errs, fmt.Sprintf("%s: type is required", prefix))
		return errs // Can't validate further without type
	}

	switch r.Type {
	case "file":
		if r.Path == "" {
			errs = append(errs, fmt.Sprintf("%s (file): path is required", prefix))
		}
		if r.Content == "" {
			errs = append(errs, fmt.Sprintf("%s (file): content is required", prefix))
		}
		if r.Mode != "" {
			if len(r.Mode) < 3 || len(r.Mode) > 4 {
				errs = append(errs, fmt.Sprintf("%s (file): mode should be 3-4 octal digits (e.g., '0644')", prefix))
			}
		}

	case "package":
		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("%s (package): name is required", prefix))
		}
		if r.State != "" && r.State != "installed" && r.State != "absent" {
			errs = append(errs, fmt.Sprintf("%s (package): state must be 'installed' or 'absent', got %q", prefix, r.State))
		}

	case "service":
		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("%s (service): name is required", prefix))
		}
		if r.State != "" && r.State != "running" && r.State != "stopped" {
			errs = append(errs, fmt.Sprintf("%s (service): state must be 'running' or 'stopped', got %q", prefix, r.State))
		}

	case "exec":
		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("%s (exec): name is required", prefix))
		}
		if r.Command == "" {
			errs = append(errs, fmt.Sprintf("%s (exec): command is required", prefix))
		}

	case "user":
		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("%s (user): name is required", prefix))
		}
		if r.State != "" && r.State != "present" && r.State != "absent" {
			errs = append(errs, fmt.Sprintf("%s (user): state must be 'present' or 'absent', got %q", prefix, r.State))
		}
		if r.UID < 0 {
			errs = append(errs, fmt.Sprintf("%s (user): uid cannot be negative", prefix))
		}
		if r.GID < 0 {
			errs = append(errs, fmt.Sprintf("%s (user): gid cannot be negative", prefix))
		}

	default:
		errs = append(errs, fmt.Sprintf("%s: unknown resource type %q", prefix, r.Type))
	}

	// Validate notifies format if specified
	if r.Notifies != "" {
		parts := strings.SplitN(r.Notifies, ":", 3)
		if len(parts) != 3 {
			errs = append(errs, fmt.Sprintf("%s: notifies must be in format 'type:name:action', got %q", prefix, r.Notifies))
		}
	}

	return errs
}
