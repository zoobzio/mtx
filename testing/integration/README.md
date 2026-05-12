# Integration Tests

This directory contains integration tests that verify mtx works correctly with external dependencies.

## Running

```bash
make test-integration
```

## Writing Integration Tests

Integration tests should:

- Test real interactions (no mocks for external services)
- Be skipped in short mode: `if testing.Short() { t.Skip("skipping integration test") }`
- Clean up resources after completion
- Use environment variables for configuration

## Conventions

- File naming: `*_integration_test.go`
- Build tag: `//go:build integration`
