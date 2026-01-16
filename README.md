# Protato

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/rahulagarwal0605/protato)](https://goreportcard.com/report/github.com/rahulagarwal0605/protato)
[![Build](https://img.shields.io/github/actions/workflow/status/rahulagarwal0605/protato/build.yml?branch=main&label=build)](https://github.com/rahulagarwal0605/protato/actions/workflows/build.yml)
[![Tests](https://img.shields.io/github/actions/workflow/status/rahulagarwal0605/protato/test.yml?branch=main&label=tests)](https://github.com/rahulagarwal0605/protato/actions/workflows/test.yml)
[![Coverage](https://img.shields.io/badge/coverage-46.9%25-yellow)](./tests/README.md#test-coverage)

A CLI tool for managing protobuf definitions across distributed Git repositories.

**Protato** (Proto + Potato) helps teams share and version protobuf definitions using Git as the distribution mechanism, maintaining a local-first workflow while leveraging centralized registry benefits.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [Documentation](#documentation)
- [Requirements](#requirements)
- [Common Use Cases](#common-use-cases)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)
- [Support](#support)

## Overview

Protato solves the protobuf distribution problem through a centralized Git-based registry while maintaining a local-first workflow.

### Key Features

- **Local First**: Proto files stored alongside code
- **Git Native**: Leverages existing Git infrastructure
- **Ownership Model**: Each project has exactly one owner
- **Dependency Resolution**: Automatic transitive dependency discovery
- **Version Tracking**: Git commits as version identifiers

## Installation

### Using the Downloader Script

```bash
# Download and run
curl -fsSL https://raw.githubusercontent.com/rahulagarwal0605/protato/main/dl/protato.sh | bash -s -- version

# Or install locally
curl -fsSL https://raw.githubusercontent.com/rahulagarwal0605/protato/main/dl/protato.sh -o /usr/local/bin/protato
chmod +x /usr/local/bin/protato
protato version
```

### Download from GitHub Releases

Download a specific version directly from [GitHub releases](https://github.com/rahulagarwal0605/protato/releases):

**macOS (Apple Silicon):**
```bash
curl -fsSL https://github.com/rahulagarwal0605/protato/releases/download/v1.0.0/protato-darwin-arm64.tar.gz | tar xz
chmod +x protato
sudo mv protato /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -fsSL https://github.com/rahulagarwal0605/protato/releases/download/v1.0.0/protato-darwin-amd64.tar.gz | tar xz
chmod +x protato
sudo mv protato /usr/local/bin/
```

**Linux (amd64):**
```bash
curl -fsSL https://github.com/rahulagarwal0605/protato/releases/download/v1.0.0/protato-linux-amd64.tar.gz | tar xz
chmod +x protato
sudo mv protato /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -fsSL https://github.com/rahulagarwal0605/protato/releases/download/v1.0.0/protato-linux-arm64.tar.gz | tar xz
chmod +x protato
sudo mv protato /usr/local/bin/
```

Replace `v1.0.0` with your desired version. See all available versions at [releases](https://github.com/rahulagarwal0605/protato/releases).

### From Source

#### Quick Install

```bash
go install github.com/rahulagarwal0605/protato@latest
```

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/rahulagarwal0605/protato.git
cd protato

# Build the binary
make build
# or
go build -o protato .

# Install to $GOPATH/bin
make install
# or
go install .

# Run tests
make test
```

#### Build Options

```bash
# Development build (fast, no optimizations)
make dev

# Build with race detector
make build-race

# Build for all platforms
make build-all

# Create release archives
make release
```

See [Makefile](Makefile) for all available build targets.

## Usage

### Initialize Workspace

```bash
# Initialize protato in your repository
protato init

# Initialize with a project
protato init --project team/service
```

### Create a New Project

```bash
# Claim ownership of a project path
protato new team/service
```

### Pull Projects

```bash
# Pull a project and its dependencies
protato pull team/service

# Pull multiple projects
protato pull team/service1 team/service2
```

### Push Projects

```bash
# Push all owned projects to the registry
protato push
```

### Verify Workspace

```bash
# Verify workspace integrity
protato verify
```

### List Projects

```bash
# List all projects in the registry
protato list

# List local projects
protato list --local
```

### List Owned Files

```bash
# List all files owned by this repository
protato mine

# List project paths only
protato mine --projects

# List absolute paths
protato mine --absolute
```

## Configuration

### protato.yaml (Workspace Configuration)

```yaml
# Projects owned by this repository
projects:
  - team/service1
  - team/service2

# Files to ignore during push/pull
ignores:
  - '**/BUILD'
  - '**/*.swp'
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `PROTATO_REGISTRY_URL` | Override registry URL |
| `PROTATO_REGISTRY_CACHE` | Override cache directory |
| `PROTATO_VERBOSITY` | Set verbosity level (0-3) |
| `PROTATO_PUSH_RETRIES` | Number of push retries (default: 5) |
| `PROTATO_PUSH_RETRY_DELAY` | Delay between retries (default: 200ms) |

## Project Structure

```
workspace/
├── protato.yaml           # Configuration
├── protos/
│   ├── owned_project/     # Owned by this repo
│   │   ├── v1/
│   │   │   └── service.proto
│   │   └── v2/
│   │       └── service.proto
│   └── consumed_project/  # Pulled from registry
│       ├── protato.lock   # Snapshot tracking
│       ├── .gitattributes # Mark as generated
│       └── v1/
│           └── api.proto
```

## Quick Start

```bash
# Install
go install github.com/rahulagarwal0605/protato@latest

# Initialize workspace
protato init --project team/service

# Create proto files
mkdir -p protos/team/service/v1
# ... create your .proto files ...

# Push to registry
protato push

# In another repo, pull the project
protato pull team/service
```

See [examples/](examples/) for more detailed examples.

## Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - System design and architecture
- **[Contributing](CONTRIBUTING.md)** - How to contribute
- **[Code of Conduct](CODE_OF_CONDUCT.md)** - Community guidelines
- **[Security Policy](SECURITY.md)** - Security reporting
- **[Changelog](CHANGELOG.md)** - Version history
- **[Tests](tests/README.md)** - Testing documentation

## Requirements

- **Go 1.24+** (for building from source)
- **Git** (for registry operations)
- **protoc** (optional, for proto compilation)

## Common Use Cases

### Sharing Protos Across Services

```bash
# Service A: Owns and publishes protos
protato init --project payments/api
protato push

# Service B: Consumes protos
protato init
protato pull payments/api
```

### CI/CD Integration

#### GitHub Actions

See [examples/ci-cd/github-workflows/protato-push-with-pat.yml](examples/ci-cd/github-workflows/protato-push-with-pat.yml) for a complete GitHub Actions workflow example (recommended for cross-repository pushes).

**Setup**: Copy the workflow file to `.github/workflows/` in your repository root.

```yaml
# Example GitHub Actions workflow
- name: Push protos to registry
  run: protato push
  env:
    PROTATO_REGISTRY_URL: ${{ secrets.PROTATO_REGISTRY_URL }}
```

#### Generic CI/CD

```bash
# In your CI pipeline
protato pull team/service1 team/service2
protato verify
protoc --proto_path=protos --go_out=. protos/**/*.proto
```


## Troubleshooting

### Registry Connection Issues

```bash
# Check registry URL
echo $PROTATO_REGISTRY_URL

# Test with verbose logging
protato -vvv pull team/service
```

### Cache Issues

```bash
# Clear cache (default: ~/.cache/protato/registry)
rm -rf ~/.cache/protato/registry

# Use custom cache location
export PROTATO_REGISTRY_CACHE=/tmp/protato-cache
```

### Workspace Verification

```bash
# Verify workspace integrity
protato verify

# List owned projects
protato mine --projects

# List all local projects
protato list --local
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Go](https://golang.org/)
- CLI powered by [Kong](https://github.com/alecthomas/kong)
- Logging with [zerolog](https://github.com/rs/zerolog)
- Proto compilation support via [protocompile](https://github.com/bufbuild/protocompile)
- Developed with support from [slice small finance bank](https://slice.bank.in/)

## Support

- **Issues**: [GitHub Issues](https://github.com/rahulagarwal0605/protato/issues)
- **Discussions**: [GitHub Discussions](https://github.com/rahulagarwal0605/protato/discussions)
- **Security**: See [SECURITY.md](SECURITY.md) for reporting security vulnerabilities

