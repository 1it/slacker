package resource

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestFileHandler_NeedsChange(t *testing.T) {
	tests := []struct {
		name           string
		existingData   []byte
		existsErr      error
		desiredContent string
		desiredMode    string
		existingMode   os.FileMode
		want           bool
		wantErr        bool
	}{
		{
			name:           "file doesn't exist",
			existingData:   nil,
			existsErr:      os.ErrNotExist,
			desiredContent: "hello world",
			want:           true,
		},
		{
			name:           "content matches",
			existingData:   []byte("hello world"),
			existsErr:      nil,
			desiredContent: "hello world",
			want:           false,
		},
		{
			name:           "content differs",
			existingData:   []byte("hello world"),
			existsErr:      nil,
			desiredContent: "goodbye world",
			want:           true,
		},
		{
			name:           "mode differs",
			existingData:   []byte("hello world"),
			existsErr:      nil,
			desiredContent: "hello world",
			desiredMode:    "0755",
			existingMode:   0644,
			want:           true,
		},
		{
			name:           "mode matches",
			existingData:   []byte("hello world"),
			existsErr:      nil,
			desiredContent: "hello world",
			desiredMode:    "0644",
			existingMode:   0644,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &executor.MockExecutor{
				ReadFileFunc: func(ctx context.Context, path string) ([]byte, error) {
					if tt.existsErr != nil {
						return nil, tt.existsErr
					}
					return tt.existingData, nil
				},
				StatFunc: func(ctx context.Context, path string) (os.FileInfo, error) {
					return mockFileInfo{mode: tt.existingMode}, nil
				},
			}

			handler := NewFileHandler(manifest.Resource{
				Type:    "file",
				Path:    "/test/path",
				Content: tt.desiredContent,
				Mode:    tt.desiredMode,
			})

			got, err := handler.NeedsChange(context.Background(), mock)
			if (err != nil) != tt.wantErr {
				t.Errorf("NeedsChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NeedsChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileHandler_Apply(t *testing.T) {
	tests := []struct {
		name           string
		existingData   []byte
		existsErr      error
		desiredContent string
		desiredMode    string
		wantChanged    bool
		wantErr        bool
	}{
		{
			name:           "creates new file",
			existsErr:      os.ErrNotExist,
			desiredContent: "new content",
			wantChanged:    true,
		},
		{
			name:           "updates existing file",
			existingData:   []byte("old content"),
			desiredContent: "new content",
			wantChanged:    true,
		},
		{
			name:           "no change needed",
			existingData:   []byte("same content"),
			desiredContent: "same content",
			wantChanged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var writtenContent []byte
			var writtenMode os.FileMode

			mock := &executor.MockExecutor{
				ReadFileFunc: func(ctx context.Context, path string) ([]byte, error) {
					if tt.existsErr != nil {
						return nil, tt.existsErr
					}
					return tt.existingData, nil
				},
				WriteFileFunc: func(ctx context.Context, path string, content []byte, mode os.FileMode) error {
					writtenContent = content
					writtenMode = mode
					return nil
				},
				StatFunc: func(ctx context.Context, path string) (os.FileInfo, error) {
					return mockFileInfo{mode: 0644}, nil
				},
			}

			handler := NewFileHandler(manifest.Resource{
				Type:    "file",
				Path:    "/test/path",
				Content: tt.desiredContent,
				Mode:    tt.desiredMode,
			})

			changed, err := handler.Apply(context.Background(), mock)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if changed != tt.wantChanged {
				t.Errorf("Apply() changed = %v, want %v", changed, tt.wantChanged)
			}

			if tt.wantChanged {
				if string(writtenContent) != tt.desiredContent {
					t.Errorf("Apply() wrote %q, want %q", writtenContent, tt.desiredContent)
				}
				if tt.desiredMode == "" && writtenMode != 0644 {
					t.Errorf("Apply() mode = %o, want 0644 (default)", writtenMode)
				}
			}
		})
	}
}

func TestFileHandler_ID(t *testing.T) {
	handler := NewFileHandler(manifest.Resource{
		Path: "/etc/nginx/nginx.conf",
	})
	if got := handler.ID(); got != "file:/etc/nginx/nginx.conf" {
		t.Errorf("ID() = %v, want file:/etc/nginx/nginx.conf", got)
	}
}

func TestFileHandler_Type(t *testing.T) {
	handler := NewFileHandler(manifest.Resource{})
	if got := handler.Type(); got != "file" {
		t.Errorf("Type() = %v, want file", got)
	}
}
