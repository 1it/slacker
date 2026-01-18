# Contributing to Slacker

Thank you for your interest in contributing to Slacker! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/slacker.git`
3. Create a branch: `git checkout -b feature/your-feature-name`

## Development Setup

### Prerequisites

- Go 1.25 or later
- Make (optional, but recommended)
- Docker (for e2e tests)

### Building

```bash
# Build for current OS
make build

# Build for Linux
make build-linux

# Run tests
make test
```

## Code Style

- Follow standard Go conventions and `gofmt`
- Run `golangci-lint` before submitting (optional but recommended)
- Add tests for new functionality
- Keep commits focused and atomic

## Adding New Resource Types

To add a new resource type (e.g., `cron`):

1. Create `internal/resource/cron.go` implementing the `Handler` interface:
   ```go
   type CronHandler struct { ... }
   func (c *CronHandler) ID() string
   func (c *CronHandler) Type() string
   func (c *CronHandler) NeedsChange(ctx, exec) (bool, error)
   func (c *CronHandler) Apply(ctx, exec) (bool, error)
   func (c *CronHandler) Notifies() string
   ```

2. Add the constructor `NewCronHandler(r manifest.Resource)`

3. Register in `internal/resource/resource.go`:
   ```go
   case "cron":
       return NewCronHandler(r), nil
   ```

4. Add any new manifest fields to `internal/manifest/manifest.go`

5. Add validation in `Manifest.Validate()`

6. Add tests in `internal/resource/cron_test.go`

7. Update README.md with documentation

## Testing

### Unit Tests

```bash
make test
```

Tests use `MockExecutor` from `internal/executor/mock.go` to simulate command execution.

### E2E Tests

```bash
# Run in Docker container
make test-e2e

# Test idempotency
make docker-test-idempotent
```

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Add a clear description of changes
4. Reference any related issues

## Reporting Issues

When reporting bugs, please include:

- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
- Relevant manifest snippets (sanitize any sensitive data!)

## Security

If you discover a security vulnerability, please do NOT open a public issue. Instead, email the maintainer directly.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
