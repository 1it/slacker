package resource

import (
	"context"
	"testing"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

func TestExecHandler_NeedsChange(t *testing.T) {
	tests := []struct {
		name           string
		unless         string
		unlessExitCode int
		want           bool
	}{
		{
			name:   "no unless condition",
			unless: "",
			want:   true, // always needs to run
		},
		{
			name:           "unless succeeds",
			unless:         "test -f /some/file",
			unlessExitCode: 0,
			want:           false, // skip execution
		},
		{
			name:           "unless fails",
			unless:         "test -f /some/file",
			unlessExitCode: 1,
			want:           true, // needs to run
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "sh" && len(args) >= 2 && args[0] == "-c" {
						return nil, nil, tt.unlessExitCode, nil
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewExecHandler(manifest.Resource{
				Type:    "exec",
				Name:    "test-exec",
				Command: "echo hello",
				Unless:  tt.unless,
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

func TestExecHandler_Apply(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		unless         string
		unlessExitCode int
		cmdExitCode    int
		wantChanged    bool
		wantErr        bool
	}{
		{
			name:        "executes command",
			command:     "apt-get update",
			wantChanged: true,
		},
		{
			name:           "skips due to unless",
			command:        "apt-get update",
			unless:         "test -f /var/cache/apt/pkgcache.bin",
			unlessExitCode: 0,
			wantChanged:    false,
		},
		{
			name:           "runs when unless fails",
			command:        "apt-get update",
			unless:         "test -f /nonexistent",
			unlessExitCode: 1,
			wantChanged:    true,
		},
		{
			name:        "command fails",
			command:     "false",
			cmdExitCode: 1,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var commandExecuted bool

			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "sh" && len(args) >= 2 && args[0] == "-c" {
						cmdArg := args[1]
						// Check if this is the unless command or the main command
						if tt.unless != "" && cmdArg == tt.unless {
							return nil, nil, tt.unlessExitCode, nil
						}
						// Main command
						commandExecuted = true
						return nil, []byte("error output"), tt.cmdExitCode, nil
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewExecHandler(manifest.Resource{
				Type:    "exec",
				Name:    "test-exec",
				Command: tt.command,
				Unless:  tt.unless,
			})

			changed, err := handler.Apply(context.Background(), mock)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && changed != tt.wantChanged {
				t.Errorf("Apply() changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.wantChanged && !commandExecuted {
				t.Error("Apply() should have executed the command")
			}
		})
	}
}

func TestExecHandler_ID(t *testing.T) {
	handler := NewExecHandler(manifest.Resource{
		Name: "apt-update",
	})
	if got := handler.ID(); got != "exec:apt-update" {
		t.Errorf("ID() = %v, want exec:apt-update", got)
	}
}

func TestExecHandler_Notifies(t *testing.T) {
	handler := NewExecHandler(manifest.Resource{
		Name:     "config-update",
		Notifies: "service:nginx:restart",
	})
	if got := handler.Notifies(); got != "service:nginx:restart" {
		t.Errorf("Notifies() = %v, want service:nginx:restart", got)
	}
}
