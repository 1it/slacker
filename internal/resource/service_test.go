package resource

import (
	"context"
	"testing"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

func TestServiceHandler_NeedsChange(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		desiredState string
		enabled      bool
		isActive     bool
		isEnabled    bool
		want         bool
	}{
		{
			name:         "service stopped, want running",
			serviceName:  "nginx",
			desiredState: "running",
			isActive:     false,
			want:         true,
		},
		{
			name:         "service running, want running",
			serviceName:  "nginx",
			desiredState: "running",
			isActive:     true,
			want:         false,
		},
		{
			name:         "service running, want stopped",
			serviceName:  "nginx",
			desiredState: "stopped",
			isActive:     true,
			want:         true,
		},
		{
			name:         "service disabled, want enabled",
			serviceName:  "nginx",
			desiredState: "running",
			enabled:      true,
			isActive:     true,
			isEnabled:    false,
			want:         true,
		},
		{
			name:         "service enabled, want disabled",
			serviceName:  "nginx",
			desiredState: "running",
			enabled:      false,
			isActive:     true,
			isEnabled:    true,
			want:         true,
		},
		{
			name:         "all matches",
			serviceName:  "nginx",
			desiredState: "running",
			enabled:      true,
			isActive:     true,
			isEnabled:    true,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "systemctl" && len(args) > 0 {
						switch args[0] {
						case "is-active":
							if tt.isActive {
								return []byte("active"), nil, 0, nil
							}
							return []byte("inactive"), nil, 3, nil
						case "is-enabled":
							if tt.isEnabled {
								return []byte("enabled"), nil, 0, nil
							}
							return []byte("disabled"), nil, 1, nil
						}
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewServiceHandler(manifest.Resource{
				Type:    "service",
				Name:    tt.serviceName,
				State:   tt.desiredState,
				Enabled: tt.enabled,
			})

			got, err := handler.NeedsChange(context.Background(), mock)
			if err != nil {
				t.Errorf("NeedsChange() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("NeedsChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceHandler_Apply(t *testing.T) {
	tests := []struct {
		name         string
		desiredState string
		enabled      bool
		isActive     bool
		isEnabled    bool
		wantCmds     []string
		wantChanged  bool
	}{
		{
			name:         "start service",
			desiredState: "running",
			isActive:     false,
			wantCmds:     []string{"start"},
			wantChanged:  true,
		},
		{
			name:         "stop service",
			desiredState: "stopped",
			isActive:     true,
			wantCmds:     []string{"stop"},
			wantChanged:  true,
		},
		{
			name:         "enable service",
			desiredState: "running",
			enabled:      true,
			isActive:     true,
			isEnabled:    false,
			wantCmds:     []string{"enable"},
			wantChanged:  true,
		},
		{
			name:         "disable service",
			desiredState: "running",
			enabled:      false,
			isActive:     true,
			isEnabled:    true,
			wantCmds:     []string{"disable"},
			wantChanged:  true,
		},
		{
			name:         "start and enable",
			desiredState: "running",
			enabled:      true,
			isActive:     false,
			isEnabled:    false,
			wantCmds:     []string{"start", "enable"},
			wantChanged:  true,
		},
		{
			name:         "no change needed",
			desiredState: "running",
			enabled:      true,
			isActive:     true,
			isEnabled:    true,
			wantCmds:     []string{},
			wantChanged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCmds []string

			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "systemctl" && len(args) > 0 {
						switch args[0] {
						case "is-active":
							if tt.isActive {
								return []byte("active"), nil, 0, nil
							}
							return []byte("inactive"), nil, 3, nil
						case "is-enabled":
							if tt.isEnabled {
								return []byte("enabled"), nil, 0, nil
							}
							return []byte("disabled"), nil, 1, nil
						case "start", "stop", "enable", "disable":
							capturedCmds = append(capturedCmds, args[0])
						}
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewServiceHandler(manifest.Resource{
				Type:    "service",
				Name:    "nginx",
				State:   tt.desiredState,
				Enabled: tt.enabled,
			})

			changed, err := handler.Apply(context.Background(), mock)
			if err != nil {
				t.Errorf("Apply() unexpected error: %v", err)
				return
			}
			if changed != tt.wantChanged {
				t.Errorf("Apply() changed = %v, want %v", changed, tt.wantChanged)
			}

			if len(capturedCmds) != len(tt.wantCmds) {
				t.Errorf("Apply() commands = %v, want %v", capturedCmds, tt.wantCmds)
			}
		})
	}
}

func TestServiceHandler_DefaultState(t *testing.T) {
	handler := NewServiceHandler(manifest.Resource{
		Type: "service",
		Name: "nginx",
		// State not specified
	})

	// Should default to "running"
	mock := &executor.MockExecutor{
		RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
			if args[0] == "is-active" {
				return []byte("active"), nil, 0, nil
			}
			return []byte("disabled"), nil, 1, nil
		},
	}

	needsChange, _ := handler.NeedsChange(context.Background(), mock)
	if needsChange {
		t.Error("Default state should be 'running', so active service should not need change")
	}
}
