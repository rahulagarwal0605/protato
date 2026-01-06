# Protato

A CLI tool for managing protobuf definitions across distributed Git repositories.

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
curl -fsSL https://raw.githubusercontent.com/org/protato/main/dl/protato-dl | bash -s -- version

# Or install locally
curl -fsSL https://raw.githubusercontent.com/org/protato/main/dl/protato-dl -o /usr/local/bin/protato-dl
chmod +x /usr/local/bin/protato-dl
protato-dl version
```

### From Source

```bash
go install github.com/rahulagarwal0605/protato@latest
```

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

## License

MIT

