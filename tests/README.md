# Protato Test Suite

This directory contains integration and end-to-end tests for the Protato project. Unit tests are located beside their source files following Go best practices.

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
- Utility functions (`internal/utils/`)
- Workspace operations (`internal/local/`)
- Git operations (`internal/git/`)
- Registry operations (`internal/registry/`)
- Protoc operations (`internal/protoc/`)
- Command validation (`cmd/`)

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

## Test Coverage Goals

- **Unit tests**: >80% coverage for utility and core logic
- **Integration tests**: Cover all major workflows
- **E2E tests**: Cover critical user paths

## Current Status

- ✅ **16 unit test files** - All passing, located beside source files
- ✅ **5 integration test files** - Component interaction tests
- ✅ **1 e2e test file** - Complete workflow tests
- ✅ **Comprehensive coverage** - Major functionality covered
