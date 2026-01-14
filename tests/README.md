# Protato Test Suite

[![Go Test](https://github.com/rahulagarwal0605/protato/actions/workflows/test.yml/badge.svg)](https://github.com/rahulagarwal0605/protato/actions/workflows/test.yml)
[![Coverage](https://img.shields.io/badge/coverage-46.9%25-yellow)](#test-coverage)

This directory contains integration and end-to-end tests for the Protato project. Unit tests are located beside their source files following Go best practices.

## Overview

The Protato test suite follows a three-tier testing strategy:
- **Unit Tests**: Fast, isolated tests beside source files
- **Integration Tests**: Component interaction tests
- **E2E Tests**: Complete workflow tests

All tests are designed to run reliably in CI/CD environments and support parallel execution.

## Test Structure

```
protato/
├── cmd/
│   └── *_test.go              # Unit tests for commands
├── internal/
│   ├── utils/
│   │   └── *_test.go          # Unit tests for utilities
│   ├── local/
│   │   └── *_test.go          # Unit tests for workspace
│   └── ...                    # Other package unit tests
└── tests/
    ├── integration/           # Integration tests for component interactions
    ├── e2e/                   # End-to-end tests for complete workflows
    └── testhelpers/           # Shared test utilities and helpers
```

## Running Tests

### Run all tests
```bash
go test ./...
```

### Run unit tests only (beside source files)
```bash
make test-unit
# or
go test ./internal/... ./cmd/...
```

### Run integration tests only
```bash
make test-integration
# or
go test ./tests/integration/...
```

### Run e2e tests only
```bash
make test-e2e
# or
go test ./tests/e2e/...
```

### Run with coverage
```bash
make test-coverage
# or
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run specific test
```bash
go test ./internal/utils -run TestValidateProjectPath
```

### Run tests in short mode (skips e2e tests)
```bash
go test -short ./...
```

## Test Categories

### Unit Tests (beside source files)

Unit tests focus on testing individual functions and methods in isolation. They are located next to their source files following Go conventions.

**Location:** `cmd/*_test.go`, `internal/*/*_test.go`

**Coverage:**
- Utility functions (`internal/utils/`) - **95.2%** ✅
- Workspace operations (`internal/local/`) - **80.9%** ✅
- Git operations (`internal/git/`) - **87.2%** ✅
- Registry operations (`internal/registry/`) - **81.1%** ✅
- Protoc operations (`internal/protoc/`) - **32.0%** ⚠️
- Command validation (`cmd/`) - **5.5%** ⚠️

**Example:**
```go
// internal/utils/validation_test.go
func TestValidateProjectPath(t *testing.T) {
    // Test individual function with various inputs
}
```

### Integration Tests (`tests/integration/`)

Integration tests verify that multiple components work together correctly. They may use real file systems and temporary directories but typically avoid network operations.

**Coverage:**
- Workspace initialization and operations
- Command execution workflows
- File system operations
- Configuration file handling
- Project discovery and management

**Example:**
```go
func TestWorkspace_CompleteWorkflow(t *testing.T) {
    // Test multiple operations together
}
```

### End-to-End Tests (`tests/e2e/`)

E2E tests verify complete user workflows from start to finish. They may require external dependencies and are typically slower.

**Coverage:**
- Complete command-line workflows
- Full initialization to usage scenarios
- Multi-command interactions

**Example:**
```go
func TestE2E_CompleteWorkflow(t *testing.T) {
    // Test complete user workflow
}
```

## Test Helpers

The `testhelpers/` package provides utilities for:
- Setting up test workspaces
- Creating test proto files
- Managing temporary directories
- File system operations

**Note:** Unit tests use inline helpers to avoid import cycles.

## Best Practices

1. **Table-driven tests**: Use table-driven tests for multiple test cases
2. **Clear test names**: Test names should clearly describe what is being tested
3. **Isolation**: Each test should be independent and not rely on other tests
4. **Cleanup**: Use `t.TempDir()` for temporary files and directories
5. **Error checking**: Always check errors and use `t.Fatalf()` for setup failures
6. **Coverage**: Aim for high test coverage, especially for critical paths

## Continuous Integration

Tests are designed to run in CI environments:
- All tests should be deterministic
- No external network dependencies (unless explicitly e2e)
- Fast execution for unit and integration tests
- E2E tests can be skipped with `-short` flag

## Adding New Tests

When adding new functionality:

1. **Unit tests**: Add tests beside the source file (e.g., `internal/utils/newfile.go` → `internal/utils/newfile_test.go`)
2. **Integration tests**: Add tests in `tests/integration/` for component interactions
3. **E2E tests**: Add tests in `tests/e2e/` for complete workflows
4. **Test helpers**: Add reusable utilities to `testhelpers/` (but avoid import cycles)

## Test Coverage {#test-coverage}

### Overall Coverage: **46.9%**

### Coverage by Package

| Package | Coverage | Status |
|---------|----------|--------|
| `internal/utils` | 95.2% | ✅ Excellent |
| `internal/git` | 87.2% | ✅ Excellent |
| `internal/registry` | 81.1% | ✅ Good |
| `internal/local` | 80.9% | ✅ Good |
| `internal/protoc` | 32.0% | ⚠️ Needs improvement |
| `cmd` | 5.5% | ⚠️ Needs improvement |
| `internal/logger` | 100.0% | ✅ Perfect |

### Coverage Goals

- **Core utilities** (`internal/utils`): ✅ **95.2%** - Exceeds target
- **Git operations** (`internal/git`): ✅ **87.2%** - Exceeds target
- **Registry operations** (`internal/registry`): ✅ **81.1%** - Meets target
- **Workspace operations** (`internal/local`): ✅ **80.9%** - Meets target
- **Protoc operations** (`internal/protoc`): ⚠️ **32.0%** - Below target
- **Command layer** (`cmd`): ⚠️ **5.5%** - Below target

## Test Statistics

| Category | Count | Status |
|----------|-------|--------|
| Unit Tests | 16 files | ✅ Passing |
| Integration Tests | 9 files | ✅ Passing |
| E2E Tests | 1 file | ✅ Passing |
| Overall Coverage | 46.9% | ⚠️ Improving |

## Current Status

- ✅ **16 unit test files** - All passing, located beside source files
- ✅ **9 integration test files** - Component interaction tests
- ✅ **1 e2e test file** - Complete workflow tests
- ⚠️ **Overall coverage: 46.9%** - Core packages well covered, command layer needs improvement
- ✅ **Parallel execution** - All tests support parallel execution

### Coverage Highlights

- **Excellent coverage** (>85%): `internal/utils` (95.2%), `internal/git` (87.2%)
- **Good coverage** (>80%): `internal/registry` (81.1%), `internal/local` (80.9%)
- **Needs improvement**: `internal/protoc` (32.0%), `cmd` (5.5%)

## Troubleshooting

### Tests Failing Locally

```bash
# Run with verbose output
go test -v ./tests/integration/...

# Run specific test
go test -v ./tests/integration -run TestRegistryCache_Snapshot

# Check for race conditions
go test -race ./...
```

### CI/CD Issues

- Ensure Go version matches CI (1.24+)
- Check for environment-specific issues
- Review test logs for detailed error messages

## Related Documentation

- [Contributing Guide](../CONTRIBUTING.md#testing-guidelines) - Testing guidelines for contributors
- [Architecture Docs](../docs/ARCHITECTURE.md#testing-strategy) - Testing strategy overview