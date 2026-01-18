package manifest

import (
	"strings"
	"testing"
)

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		manifest  Manifest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid file resource",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "file", Path: "/etc/test", Content: "hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "file missing path",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "file", Content: "hello"},
				},
			},
			wantErr:   true,
			errSubstr: "path is required",
		},
		{
			name: "file missing content",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "file", Path: "/etc/test"},
				},
			},
			wantErr:   true,
			errSubstr: "content is required",
		},
		{
			name: "valid package resource",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "package", Name: "nginx"},
				},
			},
			wantErr: false,
		},
		{
			name: "package missing name",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "package", State: "installed"},
				},
			},
			wantErr:   true,
			errSubstr: "name is required",
		},
		{
			name: "package invalid state",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "package", Name: "nginx", State: "invalid"},
				},
			},
			wantErr:   true,
			errSubstr: "must be 'installed' or 'absent'",
		},
		{
			name: "valid service resource",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "service", Name: "nginx"},
				},
			},
			wantErr: false,
		},
		{
			name: "service invalid state",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "service", Name: "nginx", State: "invalid"},
				},
			},
			wantErr:   true,
			errSubstr: "must be 'running' or 'stopped'",
		},
		{
			name: "valid exec resource",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "exec", Name: "update", Command: "apt-get update"},
				},
			},
			wantErr: false,
		},
		{
			name: "exec missing command",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "exec", Name: "update"},
				},
			},
			wantErr:   true,
			errSubstr: "command is required",
		},
		{
			name: "valid user resource",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "user", Name: "appuser"},
				},
			},
			wantErr: false,
		},
		{
			name: "user invalid state",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "user", Name: "appuser", State: "invalid"},
				},
			},
			wantErr:   true,
			errSubstr: "must be 'present' or 'absent'",
		},
		{
			name: "unknown resource type",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "unknown", Name: "test"},
				},
			},
			wantErr:   true,
			errSubstr: "unknown resource type",
		},
		{
			name: "missing resource type",
			manifest: Manifest{
				Resources: []Resource{
					{Name: "test"},
				},
			},
			wantErr:   true,
			errSubstr: "type is required",
		},
		{
			name: "invalid notifies format",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "file", Path: "/etc/test", Content: "hello", Notifies: "invalid"},
				},
			},
			wantErr:   true,
			errSubstr: "notifies must be in format",
		},
		{
			name: "valid notifies format",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "file", Path: "/etc/test", Content: "hello", Notifies: "service:nginx:restart"},
				},
			},
			wantErr: false,
		},
		{
			name: "host missing address",
			manifest: Manifest{
				Hosts: []Host{
					{User: "root"},
				},
			},
			wantErr:   true,
			errSubstr: "address is required",
		},
		{
			name: "host missing user",
			manifest: Manifest{
				Hosts: []Host{
					{Address: "192.168.1.1:22"},
				},
			},
			wantErr:   true,
			errSubstr: "user is required",
		},
		{
			name: "valid host",
			manifest: Manifest{
				Hosts: []Host{
					{Address: "192.168.1.1:22", User: "root"},
				},
			},
			wantErr: false,
		},
		{
			name: "verify missing name",
			manifest: Manifest{
				Verify: []Verify{
					{Command: "curl http://localhost"},
				},
			},
			wantErr:   true,
			errSubstr: "name is required",
		},
		{
			name: "verify missing command",
			manifest: Manifest{
				Verify: []Verify{
					{Name: "health check"},
				},
			},
			wantErr:   true,
			errSubstr: "command is required",
		},
		{
			name: "multiple errors",
			manifest: Manifest{
				Resources: []Resource{
					{Type: "file"}, // missing path and content
					{Type: "package", State: "invalid"},
				},
			},
			wantErr:   true,
			errSubstr: "path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Validate() error = %v, want substring %q", err, tt.errSubstr)
				}
			}
		})
	}
}

func TestManifest_ValidateFileMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"valid 3 digit", "644", false},
		{"valid 4 digit", "0644", false},
		{"valid executable", "0755", false},
		{"too short", "64", true},
		{"too long", "00644", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Manifest{
				Resources: []Resource{
					{Type: "file", Path: "/test", Content: "hello", Mode: tt.mode},
				},
			}
			err := m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() with mode %q error = %v, wantErr %v", tt.mode, err, tt.wantErr)
			}
		})
	}
}
