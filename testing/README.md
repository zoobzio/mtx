# Testing Infrastructure

This directory contains testing utilities and infrastructure for mtx.

## Structure

```
testing/
├── README.md           # This file
├── helpers.go          # Domain-specific test helpers
├── helpers_test.go     # Tests for helpers themselves
├── integration/        # Integration tests
│   └── README.md
└── benchmarks/         # Performance benchmarks
    └── README.md
```

## Running Tests

```bash
# All tests
make test

# Unit tests only (fast)
make test-unit

# Integration tests
make test-integration

# Benchmarks
make test-bench
```

## Writing Tests

### Test Files

- Maintain 1:1 relationship between source files and test files
- `api.go` → `api_test.go`
- Exception: delegation-only files may omit tests

### Using Helpers

Import the testing package:

```go
import "github.com/zoobzio/mtx/testing"
```

### Coverage Targets

- Project floor: 70%
- New code (patch): 80%
