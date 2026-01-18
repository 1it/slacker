package resource

import (
	"context"
	"testing"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

func TestUserHandler_NeedsChange(t *testing.T) {
	tests := []struct {
		name         string
		userName     string
		desiredState string
		userExists   bool
		want         bool
	}{
		{
			name:         "user doesn't exist, want present",
			userName:     "appuser",
			desiredState: "present",
			userExists:   false,
			want:         true,
		},
		{
			name:         "user exists, want present",
			userName:     "appuser",
			desiredState: "present",
			userExists:   true,
			want:         false,
		},
		{
			name:         "user exists, want absent",
			userName:     "appuser",
			desiredState: "absent",
			userExists:   true,
			want:         true,
		},
		{
			name:         "user doesn't exist, want absent",
			userName:     "appuser",
			desiredState: "absent",
			userExists:   false,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "id" && len(args) >= 2 && args[0] == "-u" {
						if tt.userExists {
							return []byte("1000"), nil, 0, nil
						}
						return nil, []byte("no such user"), 1, nil
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewUserHandler(manifest.Resource{
				Type:  "user",
				Name:  tt.userName,
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

func TestUserHandler_Apply(t *testing.T) {
	tests := []struct {
		name         string
		userName     string
		desiredState string
		uid          int
		home         string
		shell        string
		system       bool
		userExists   bool
		wantCmd      string
		wantChanged  bool
	}{
		{
			name:         "create user",
			userName:     "appuser",
			desiredState: "present",
			userExists:   false,
			wantCmd:      "useradd",
			wantChanged:  true,
		},
		{
			name:         "create user with options",
			userName:     "appuser",
			desiredState: "present",
			uid:          1500,
			home:         "/opt/app",
			shell:        "/bin/zsh",
			userExists:   false,
			wantCmd:      "useradd",
			wantChanged:  true,
		},
		{
			name:         "create system user",
			userName:     "daemon",
			desiredState: "present",
			system:       true,
			userExists:   false,
			wantCmd:      "useradd",
			wantChanged:  true,
		},
		{
			name:         "delete user",
			userName:     "olduser",
			desiredState: "absent",
			userExists:   true,
			wantCmd:      "userdel",
			wantChanged:  true,
		},
		{
			name:         "user already exists",
			userName:     "appuser",
			desiredState: "present",
			userExists:   true,
			wantChanged:  false,
		},
		{
			name:         "user already absent",
			userName:     "appuser",
			desiredState: "absent",
			userExists:   false,
			wantChanged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCmd string
			var capturedArgs []string

			mock := &executor.MockExecutor{
				RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
					if cmd == "id" && len(args) >= 2 && args[0] == "-u" {
						if tt.userExists {
							return []byte("1000"), nil, 0, nil
						}
						return nil, []byte("no such user"), 1, nil
					}
					if cmd == "useradd" || cmd == "userdel" || cmd == "usermod" {
						capturedCmd = cmd
						capturedArgs = args
						return nil, nil, 0, nil
					}
					return nil, nil, 0, nil
				},
			}

			handler := NewUserHandler(manifest.Resource{
				Type:   "user",
				Name:   tt.userName,
				State:  tt.desiredState,
				UID:    tt.uid,
				Home:   tt.home,
				Shell:  tt.shell,
				System: tt.system,
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
				t.Errorf("Apply() called %s, want %s", capturedCmd, tt.wantCmd)
			}

			// Verify specific options were passed
			if tt.wantChanged && tt.wantCmd == "useradd" {
				if tt.uid > 0 {
					found := false
					for i, arg := range capturedArgs {
						if arg == "-u" && i+1 < len(capturedArgs) {
							found = true
							break
						}
					}
					if !found {
						t.Error("Apply() should have passed -u flag for uid")
					}
				}
				if tt.system {
					found := false
					for _, arg := range capturedArgs {
						if arg == "-r" {
							found = true
							break
						}
					}
					if !found {
						t.Error("Apply() should have passed -r flag for system user")
					}
				}
			}
		})
	}
}

func TestUserHandler_DefaultState(t *testing.T) {
	handler := NewUserHandler(manifest.Resource{
		Type: "user",
		Name: "testuser",
		// State not specified
	})

	// Should default to "present"
	mock := &executor.MockExecutor{
		RunFunc: func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
			if cmd == "id" {
				return []byte("1000"), nil, 0, nil // user exists
			}
			return nil, nil, 0, nil
		},
	}

	needsChange, _ := handler.NeedsChange(context.Background(), mock)
	if needsChange {
		t.Error("Default state should be 'present', so existing user should not need change")
	}
}

func TestUserHandler_ID(t *testing.T) {
	handler := NewUserHandler(manifest.Resource{
		Name: "appuser",
	})
	if got := handler.ID(); got != "user:appuser" {
		t.Errorf("ID() = %v, want user:appuser", got)
	}
}
