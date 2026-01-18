package resource

import (
	"context"
	"testing"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

func TestPackageHandler_NeedsChange(t *testing.T) {
	tests := []struct {
		name         string
		packageName  string
		desiredState string
		dpkgOutput   string
		dpkgExitCode int
		want         bool
	}{
		{
			name:         "package not installed, want installed",
			packageName:  "nginx",
			desiredState: "installed",
			dpkgExitCode: 1, // package not found
			want:         true,
		},
		{
			name:         "package installed, want installed",
			packageName:  "nginx",
			desiredState: "installed",
			dpkgOutput:   "install ok installed",
			dpkgExitCode: 0,
			want:         false,
		},
		{
			name:         "package installed, want absent",
			packageName:  "nginx",
			desiredState: "absent",
			dpkgOutput:   "install ok installed",
			dpkgExitCode: 0,
			want:         true,
		},
		{
			name:         "package not installed, want absent",
			packageName:  "nginx",
			desiredState: "absent",
			dpkgExitCode: 1,
			want:         false,
		},
		{
			name:         "package deinstall state, want installed",
			packageName:  "nginx",
			desiredState: "installed",
			dpkgOutput:   "deinstall ok config-files",
			dpkgExitCode: 0,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "dpkg-query" {
						return []byte(tt.dpkgOutput), nil, tt.dpkgExitCode, nil
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewPackageHandler(manifest.Resource{
				Type:  "package",
				Name:  tt.packageName,
				State: tt.desiredState,
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

func TestPackageHandler_Apply(t *testing.T) {
	tests := []struct {
		name         string
		packageName  string
		desiredState string
		isInstalled  bool
		wantCmd      string
		wantChanged  bool
	}{
		{
			name:         "install package",
			packageName:  "nginx",
			desiredState: "installed",
			isInstalled:  false,
			wantCmd:      "install",
			wantChanged:  true,
		},
		{
			name:         "remove package",
			packageName:  "nginx",
			desiredState: "absent",
			isInstalled:  true,
			wantCmd:      "remove",
			wantChanged:  true,
		},
		{
			name:         "already installed",
			packageName:  "nginx",
			desiredState: "installed",
			isInstalled:  true,
			wantChanged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCmd string

			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "dpkg-query" {
						if tt.isInstalled {
							return []byte("install ok installed"), nil, 0, nil
						}
						return nil, nil, 1, nil
					}
					if cmd == "apt-get" && len(args) > 0 {
						capturedCmd = args[0]
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewPackageHandler(manifest.Resource{
				Type:  "package",
				Name:  tt.packageName,
				State: tt.desiredState,
			})

			changed, err := handler.Apply(context.Background(), mock)
			if err != nil {
				t.Errorf("Apply() unexpected error: %v", err)
				return
			}
			if changed != tt.wantChanged {
				t.Errorf("Apply() changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.wantChanged && capturedCmd != tt.wantCmd {
				t.Errorf("Apply() called apt-get %s, want %s", capturedCmd, tt.wantCmd)
			}
		})
	}
}

func TestPackageHandler_DefaultState(t *testing.T) {
	handler := NewPackageHandler(manifest.Resource{
		Type: "package",
		Name: "nginx",
		// State not specified
	})

	// Should default to "installed"
	mock := &executor.MockExecutor{
		RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
			return []byte("install ok installed"), nil, 0, nil
		},
	}

	needsChange, _ := handler.NeedsChange(context.Background(), mock)
	if needsChange {
		t.Error("Default state should be 'installed', so installed package should not need change")
	}
}
