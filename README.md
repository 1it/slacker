# Slacker - Configuration Management Tool

A lightweight configuration management tool written in Go, inspired by Puppet/Chef/Ansible. Slacker applies declarative YAML configurations to local or remote servers idempotently.

## Features

- **Idempotent operations** - Resources are only modified when needed
- **Local and remote execution** - Apply configurations locally or via SSH
- **Parallel host processing** - Process multiple hosts concurrently with `--parallel`
- **SSH key auto-detection** - Automatically uses `~/.ssh/` keys when no password is set
- **SSH host key verification** - Strict mode, accept-new mode, or insecure (default)
- **YAML-based manifests** - Declarative configuration format with validation
- **Notification system** - Trigger service restarts on config changes
- **Dry-run mode** - Preview changes before applying
- **Post-deployment verification** - Validate deployments with custom checks (curl, etc.)
- **User management** - Create, modify, and delete system users

## Architecture

```
┌─────────────────────────────────────────┐
│           CLI / Entry Point             │
│  (parse args, load manifest)            │
├─────────────────────────────────────────┤
│          Runner Layer                   │
│  (orchestrates resources, notifications)│
├─────────────────────────────────────────┤
│          Executor Layer                 │
│  LocalExecutor  │  SSHExecutor          │
│  (implements same interface)            │
├─────────────────────────────────────────┤
│          Resource Layer                 │
│  File  │  Package  │  Service  │  Exec  │
│  (idempotent operations)                │
└─────────────────────────────────────────┘
```

### Key Components

| Component | Description |
|-----------|-------------|
| **Executor** | Abstracts local vs remote command execution |
| **Resource** | Represents configurable system resources (File, Package, Service, Exec) |
| **Runner** | Orchestrates resource application and notifications |
| **Manifest** | YAML configuration defining hosts and resources |

## Installation

### Prerequisites

- Go 1.25+ (for building from source)
- SSH access to target servers (for remote execution)
- Ubuntu/Debian target systems (uses apt-get and systemctl)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/1it/slacker.git
cd slacker

# Build for current OS
make build

# Build for Linux (for remote deployment)
make build-linux
```

### Pre-built Binaries

Download from releases or build locally:

```bash
./slacker --help
```

## Usage

### Local Execution

Apply a manifest to the local machine:

```bash
# Apply configuration
sudo ./slacker local -c manifest.yaml

# Dry-run (preview changes)
sudo ./slacker local -c manifest.yaml --dry-run

# With custom timeout
sudo ./slacker local -c manifest.yaml --timeout 20m
```

### Remote Execution

Apply a manifest to remote hosts via SSH:

```bash
# Apply to all hosts defined in manifest (sequential)
./slacker remote -c manifest.yaml

# Dry-run
./slacker remote -c manifest.yaml --dry-run

# Process hosts in parallel (5 concurrent connections)
./slacker remote -c manifest.yaml --parallel 5

# Parallel with fail-fast (stop on first failure)
./slacker remote -c manifest.yaml --parallel 10 --fail-fast
```

#### Parallel Processing

By default, hosts are processed sequentially. Use `--parallel` to process multiple hosts concurrently:

- **`--parallel N`** - Process up to N hosts simultaneously (default: 1 = sequential)
- **`--fail-fast`** - Stop processing all hosts when the first host fails (only effective with `--parallel > 1`)

**Example**: Processing 20 hosts with parallelism=5 will create 5 concurrent SSH connections, completing in roughly 1/4 the time of sequential processing.

#### Host Key Verification

By default, host key verification is disabled for backward compatibility. Enable it for production:

```bash
# Strict mode - hosts must be in known_hosts
./slacker remote -c manifest.yaml --strict-host-keys

# Accept new hosts, reject changed keys (recommended)
./slacker remote -c manifest.yaml --accept-new-keys

# Custom known_hosts file
./slacker remote -c manifest.yaml --strict-host-keys --known-hosts ~/.ssh/known_hosts_prod
```

## SSH Authentication

Slacker supports multiple SSH authentication methods with automatic fallback:

### Authentication Priority

1. **Password** - If `password` is specified in the host config
2. **Explicit key** - If `key` path is specified in the host config
3. **Auto-detect** - Automatically finds keys in `~/.ssh/` (if no password/key specified)

### Auto-detected Key Locations

When no password or key is provided, Slacker searches for keys in this order:
- `~/.ssh/id_ed25519`
- `~/.ssh/id_ecdsa`
- `~/.ssh/id_rsa`
- `~/.ssh/id_dsa`

### Host Configuration Examples

```yaml
hosts:
  # Using password authentication
  - address: "192.168.1.100:22"
    user: "root"
    password: "secret"

  # Using explicit key path
  - address: "192.168.1.101:22"
    user: "deploy"
    key: "~/.ssh/deploy_key"

  # Auto-detect local SSH keys (no password or key specified)
  - address: "192.168.1.102:22"
    user: "root"
```

> **Note**: Passphrase-protected keys are not currently supported for auto-detection.

## Manifest Format

Manifests are YAML files defining hosts and resources:

```yaml
# manifest.yaml
hosts:
  # Password auth
  - address: "192.168.1.100:22"
    user: "root"
    password: "secret"
  # Key-based auth (auto-detect)
  - address: "192.168.1.101:22"
    user: "root"

resources:
  # Execute command (with idempotency check)
  - type: exec
    name: apt-update
    command: "apt-get update"
    unless: "test -f /var/cache/apt/pkgcache.bin"

  # Install packages
  - type: package
    name: nginx
    state: installed
    notifies: service:nginx:restart

  # Manage files
  - type: file
    path: /var/www/html/index.html
    content: |
      <h1>Hello, world!</h1>
    owner: www-data
    group: www-data
    mode: "0644"
    notifies: service:nginx:restart

  # Manage services
  - type: service
    name: nginx
    state: running
    enabled: true
```

## Resource Types

### User Resource

Manages system users.

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Username | Yes |
| `state` | `present` or `absent` | No (default: present) |
| `uid` | User ID | No (auto-assigned) |
| `gid` | Primary group ID | No (auto-assigned) |
| `home` | Home directory path | No (default: /home/<name>) |
| `shell` | Login shell | No (default: /bin/bash) |
| `system` | Create as system user | No (default: false) |
| `notifies` | Notification target | No |

**Idempotency**: Checks if user exists and if properties match.

```yaml
resources:
  - type: user
    name: appuser
    uid: 1500
    home: /opt/app
    shell: /bin/bash
    state: present
```

### File Resource

Manages file content and permissions.

| Field | Description | Required |
|-------|-------------|----------|
| `path` | File path | Yes |
| `content` | File content | Yes |
| `owner` | File owner (name) | No |
| `group` | File group (name) | No |
| `mode` | File mode (octal string) | No (default: 0644) |
| `notifies` | Notification target | No |

**Idempotency**: Compares SHA256 hash of content and file mode.

### Package Resource

Manages system packages via apt-get.

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Package name | Yes |
| `state` | `installed` or `absent` | No (default: installed) |
| `notifies` | Notification target | No |

**Idempotency**: Checks `dpkg-query -W -f='${Status}'`.

### Service Resource

Manages systemd services.

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Service name | Yes |
| `state` | `running` or `stopped` | No (default: running) |
| `enabled` | Enable on boot | No (default: false) |
| `notifies` | Notification target | No |

**Idempotency**: Checks `systemctl is-active` and `systemctl is-enabled`.

### Exec Resource

Executes arbitrary commands with optional idempotency check.

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Resource name | Yes |
| `command` | Command to execute | Yes |
| `unless` | Skip if this command succeeds | No |
| `notifies` | Notification target | No |

**Idempotency**: Runs `unless` command; skips if exit code is 0.

## Notification System

Resources can trigger notifications when they change:

```yaml
- type: file
  path: /etc/nginx/sites-available/default
  content: |
    server { ... }
  notifies: service:nginx:restart
```

**Format**: `<type>:<name>:<action>`

Supported actions for services:
- `restart`
- `reload`
- `start`
- `stop`

Notifications are **deduplicated** - if multiple resources notify the same target, the action executes only once after all resources are applied.

## Verification Checks

After successful deployment, you can run verification commands to validate the deployment worked:

```yaml
verify:
  - name: "HTTP response check"
    command: "curl -sf http://${HOST}/index.php"
    expect: "Hello, world!"
  
  - name: "HTTP status code"
    command: "curl -sf -o /dev/null -w '%{http_code}' http://${HOST}/"
    expect: "200"
```

### Verify Fields

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Human-readable check name | Yes |
| `command` | Command to run locally (supports `${HOST}` placeholder) | Yes |
| `expect` | Expected substring in output | No |

### Features

- **`${HOST}` placeholder** - Replaced with target host IP/hostname (port stripped)
- **Local execution** - Commands run on your machine, not the remote host
- **Output matching** - Optionally check for expected strings in output
- **Per-host checks** - Each verify runs against all successfully configured hosts

### Skipping Verification

```bash
# Skip verification checks
./slacker remote -c manifest.yaml --skip-verify

# Verification auto-skipped in dry-run mode
./slacker remote -c manifest.yaml --dry-run
```

## Testing

### Unit Tests

```bash
make test
```

### Docker E2E Tests

```bash
# Run full e2e test suite
make test-e2e

# Quick test with basic manifest
make docker-test

# Test idempotency (runs twice)
make docker-test-idempotent
```

## Example: PHP + nginx Setup

See `e2e/nginx-php.yaml` for a complete example that:

1. Updates apt cache
2. Installs nginx and php-fpm
3. Deploys a PHP application
4. Configures nginx for PHP
5. Ensures services are running

```bash
# Apply to remote servers
./slacker remote -c e2e/nginx-php.yaml

# Verify
curl http://<server-ip>/
# Expected: Hello, world!
```

## Project Structure

```
slacker/
├── cmd/
│   ├── root.go          # CLI root command
│   ├── local.go         # Local execution command
│   └── remote.go        # Remote SSH execution command
├── internal/
│   ├── executor/
│   │   ├── executor.go  # Executor interface
│   │   ├── local.go     # Local command execution
│   │   └── ssh.go       # SSH + SFTP execution
│   ├── resource/
│   │   ├── resource.go  # Resource interface
│   │   ├── file.go      # File resource
│   │   ├── package.go   # Package resource
│   │   ├── service.go   # Service resource
│   │   └── exec.go      # Exec resource
│   ├── runner/
│   │   └── runner.go    # Resource orchestration
│   ├── manifest/
│   │   └── manifest.go  # YAML parsing
│   └── logger/
│       └── logger.go    # Colored logging
├── e2e/
│   ├── basic.yaml       # Basic test manifest
│   └── nginx-php.yaml   # Full nginx+PHP test
├── Dockerfile.test      # E2E test container
├── Makefile             # Build and test targets
└── README.md            # This file
```

## License

See [LICENSE](LICENSE) file.

