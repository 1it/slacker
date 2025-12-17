package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Common SSH key paths to check (in order of preference)
var defaultKeyPaths = []string{
	"~/.ssh/id_ed25519",
	"~/.ssh/id_ecdsa",
	"~/.ssh/id_rsa",
	"~/.ssh/id_dsa",
}

// SSHExecutor executes commands on remote hosts via SSH
type SSHExecutor struct {
	host       string
	user       string
	client     *ssh.Client
	sftpClient *sftp.Client
}

// NewSSHExecutor creates a new SSH executor and establishes connection
// Authentication priority:
// 1. Password (if provided)
// 2. Explicit key path (if provided)
// 3. Auto-detect keys from ~/.ssh/
func NewSSHExecutor(host, user, password, keyPath string) (*SSHExecutor, error) {
	authMethods, authDesc, err := buildAuthMethods(password, keyPath)
	if err != nil {
		return nil, err
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method available for %s@%s", user, host)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s (%s): %w", host, authDesc, err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return &SSHExecutor{
		host:       host,
		user:       user,
		client:     client,
		sftpClient: sftpClient,
	}, nil
}

// buildAuthMethods constructs SSH auth methods based on available credentials
func buildAuthMethods(password, keyPath string) ([]ssh.AuthMethod, string, error) {
	var methods []ssh.AuthMethod
	var descriptions []string

	// Password authentication
	if password != "" {
		methods = append(methods, ssh.Password(password))
		descriptions = append(descriptions, "password")
	}

	// Explicit key path
	if keyPath != "" {
		signer, err := loadPrivateKey(keyPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load key %s: %w", keyPath, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
		descriptions = append(descriptions, fmt.Sprintf("key:%s", keyPath))
	}

	// Auto-detect keys if no explicit auth provided
	if password == "" && keyPath == "" {
		for _, path := range defaultKeyPaths {
			expandedPath := expandPath(path)
			if _, err := os.Stat(expandedPath); err == nil {
				signer, err := loadPrivateKey(expandedPath)
				if err != nil {
					// Skip keys that can't be loaded (might be encrypted, wrong format, etc.)
					continue
				}
				methods = append(methods, ssh.PublicKeys(signer))
				descriptions = append(descriptions, fmt.Sprintf("key:%s", path))
				// Use first available key
				break
			}
		}
	}

	return methods, strings.Join(descriptions, ", "), nil
}

// loadPrivateKey reads and parses an SSH private key file
func loadPrivateKey(path string) (ssh.Signer, error) {
	expandedPath := expandPath(path)
	keyData, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		// Check if key is encrypted (passphrase protected)
		if strings.Contains(err.Error(), "passphrase") || strings.Contains(err.Error(), "encrypted") {
			return nil, fmt.Errorf("key is passphrase-protected (not supported): %w", err)
		}
		return nil, fmt.Errorf("failed to parse key: %w", err)
	}

	return signer, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// Close closes the SSH and SFTP connections
func (e *SSHExecutor) Close() error {
	var errs []string
	if e.sftpClient != nil {
		if err := e.sftpClient.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("sftp: %v", err))
		}
	}
	if e.client != nil {
		if err := e.client.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("ssh: %v", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Host returns the host address
func (e *SSHExecutor) Host() string {
	return e.host
}

// Run executes a command on the remote host
func (e *SSHExecutor) Run(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
	session, err := e.client.NewSession()
	if err != nil {
		return nil, nil, -1, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Build command string
	fullCmd := cmd
	if len(args) > 0 {
		// Quote arguments to handle spaces
		quotedArgs := make([]string, len(args))
		for i, arg := range args {
			quotedArgs[i] = shellQuote(arg)
		}
		fullCmd = fmt.Sprintf("%s %s", cmd, strings.Join(quotedArgs, " "))
	}

	// Set environment for non-interactive apt
	fullCmd = fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s", fullCmd)

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	// Use context for cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Run(fullCmd)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGKILL)
		return nil, nil, -1, ctx.Err()
	case err := <-done:
		stdout = stdoutBuf.Bytes()
		stderr = stderrBuf.Bytes()

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
				return stdout, stderr, exitCode, nil
			}
			return stdout, stderr, -1, err
		}
		return stdout, stderr, 0, nil
	}
}

// WriteFile writes content to a file on the remote host via SFTP
func (e *SSHExecutor) WriteFile(ctx context.Context, path string, content []byte, mode os.FileMode) error {
	f, err := e.sftpClient.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	if err := e.sftpClient.Chmod(path, mode); err != nil {
		return fmt.Errorf("failed to chmod file %s: %w", path, err)
	}

	return nil
}

// ReadFile reads the content of a file on the remote host via SFTP
func (e *SSHExecutor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	f, err := e.sftpClient.Open(path)
	if err != nil {
		// Convert to os.ErrNotExist for compatibility
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return content, nil
}

// Stat returns file info for the given path on the remote host
func (e *SSHExecutor) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	info, err := e.sftpClient.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("failed to stat %s: %w", path, err)
	}
	return info, nil
}

// Chown changes ownership of a file on the remote host
func (e *SSHExecutor) Chown(ctx context.Context, path string, uid, gid int) error {
	// SFTP doesn't have direct Chown, use command
	args := []string{fmt.Sprintf("%d:%d", uid, gid), path}
	_, stderr, exitCode, err := e.Run(ctx, "chown", args...)
	if err != nil {
		return fmt.Errorf("failed to chown %s: %w", path, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("chown %s failed (exit %d): %s", path, exitCode, string(stderr))
	}
	return nil
}

// shellQuote quotes a string for safe use in shell commands
func shellQuote(s string) string {
	// If string contains no special characters, return as-is
	if !strings.ContainsAny(s, " \t\n\"'\\$`!") {
		return s
	}
	// Use single quotes, escaping any existing single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// DetectSSHKeys returns a list of available SSH keys in the default locations
func DetectSSHKeys() []string {
	var found []string
	for _, path := range defaultKeyPaths {
		expandedPath := expandPath(path)
		if _, err := os.Stat(expandedPath); err == nil {
			found = append(found, path)
		}
	}
	return found
}
