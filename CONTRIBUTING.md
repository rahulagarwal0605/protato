# Contributing to Protato

Thank you for your interest in contributing to Protato! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the [issue list](https://github.com/rahulagarwal0605/protato/issues) to see if the problem has already been reported. When creating a bug report, include:

- **Clear title and description**
- **Steps to reproduce** the behavior
- **Expected behavior** vs **actual behavior**
- **Environment details**: OS, Go version, Protato version
- **Relevant logs** (if applicable)
- **Minimal reproduction case** (if possible)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

- **Clear description** of the enhancement
- **Use case**: Why is this useful?
- **Proposed solution** (if you have one)
- **Alternatives considered** (if any)

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Make your changes** following our coding standards
3. **Add tests** for new functionality
4. **Update documentation** as needed
5. **Ensure all tests pass**: `make test`
6. **Submit a pull request** with a clear description

#### Pull Request Checklist

- [ ] Code follows the project's style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex logic
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] All tests pass locally
- [ ] No new warnings introduced
- [ ] Commit messages follow [conventional commits](https://www.conventionalcommits.org/)

## Development Setup

### Prerequisites

- **Go 1.24+** (check with `go version`)
- **Git** for version control
- **Make** (optional, for convenience commands)

### Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/rahulagarwal0605/protato.git
   cd protato
   ```

2. **Install dependencies**:
   ```bash
   make deps
   # or
   go mod download
   ```

3. **Build the project**:
   ```bash
   # Quick build
   make build
   # or
   go build -o protato .
   
   # Development build (faster, no optimizations)
   make dev
   
   # Build with race detector
   make build-race
   ```

4. **Install to $GOPATH/bin**:
   ```bash
   make install
   # or
   go install .
   ```

5. **Run tests**:
   ```bash
   make test
   # or
   go test ./...
   ```

6. **Verify installation**:
   ```bash
   ./protato --version
   # or if installed
   protato --version
   ```

### Development Workflow

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and commit:
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

3. **Run tests and linters**:
   ```bash
   make test
   make lint
   ```

4. **Push and create PR**:
   ```bash
   git push origin feature/your-feature-name
   ```

## Coding Standards

### Go Style Guide

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` or `gofumpt` for formatting
- Run `make fmt` before committing
- Follow the existing code style in the project

### Code Organization

- **Commands**: `cmd/` - CLI command implementations
- **Internal**: `internal/` - Internal packages (not for external use)
- **Tests**: `tests/` - Integration and e2e tests
- **Examples**: `examples/` - Example scripts and usage

### Testing

- **Unit tests**: Place `*_test.go` files beside source files
- **Integration tests**: Place in `tests/integration/`
- **E2E tests**: Place in `tests/e2e/`
- **Coverage**: Aim for >80% coverage on new code
- **Run tests**: `make test-coverage` to see coverage report

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples**:
```
feat(registry): add support for custom branch names

fix(cache): resolve HEAD detection issue in parallel tests

docs(readme): update installation instructions
```

## Project Structure

```
protato/
├── cmd/              # CLI commands
├── internal/         # Internal packages
│   ├── git/         # Git operations
│   ├── local/       # Workspace management
│   ├── registry/    # Registry operations
│   ├── protoc/      # Protoc integration
│   └── utils/       # Utility functions
├── tests/           # Integration and e2e tests
├── examples/        # Example scripts
└── main.go         # Entry point
```

## Testing Guidelines

### Running Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# Integration tests only
make test-integration

# E2E tests only
make test-e2e

# With coverage
make test-coverage
```

### Writing Tests

- Use table-driven tests for multiple test cases
- Use descriptive test names: `TestFunctionName_Scenario_ExpectedResult`
- Clean up resources (use `t.TempDir()`)
- Test error cases, not just happy paths
- Mock external dependencies when appropriate

### Example Test

```go
func TestValidateProjectPath(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        wantErr bool
    }{
        {
            name:    "valid path",
            path:    "team/service",
            wantErr: false,
        },
        {
            name:    "invalid empty path",
            path:    "",
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateProjectPath(tt.path)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateProjectPath() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Documentation

### Updating Documentation

- **README.md**: Main project documentation
- **CONTRIBUTING.md**: This file
- **CODE_OF_CONDUCT.md**: Community standards
- **CHANGELOG.md**: Version history
- **docs/**: Additional documentation (if needed)

### Code Comments

- Export public APIs with godoc comments
- Explain "why" not "what" in comments
- Keep comments up-to-date with code changes

Example:
```go
// ValidateProjectPath validates a project path according to protato naming conventions.
// Project paths must be non-empty and follow the format: team/service or team/service/version.
func ValidateProjectPath(path string) error {
    // ...
}
```

## Release Process

1. Update `CHANGELOG.md` with new changes
2. Update version in relevant files
3. Create a git tag: `git tag v1.0.0`
4. Push tag: `git push origin v1.0.0`
5. Create GitHub release

## Getting Help

- **Documentation**: Check [README.md](README.md) and [tests/README.md](tests/README.md)
- **Issues**: Search existing [issues](https://github.com/rahulagarwal0605/protato/issues)
- **Discussions**: Use GitHub Discussions for questions
- **Email**: Contact maintainers directly (if needed)

## Recognition

Contributors will be recognized in:
- `CONTRIBUTORS.md` (if created)
- Release notes
- Project documentation

Thank you for contributing to Protato! 🎉
