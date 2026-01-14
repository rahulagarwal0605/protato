# Protato Architecture

This document provides an overview of Protato's architecture and design decisions.

## Overview

Protato is a CLI tool for managing protobuf definitions across distributed Git repositories. It uses a Git-based registry approach with a local-first workflow.

## Core Concepts

### Registry

A **registry** is a Git repository that serves as the central storage for protobuf definitions. It's typically a bare Git repository that accepts pushes from multiple sources.

### Workspace

A **workspace** is a local Git repository that uses Protato. It contains:
- `protato.yaml`: Configuration file
- `protos/`: Directory containing proto files (owned and consumed)

### Project

A **project** is a logical grouping of proto files identified by a path (e.g., `team/service`). Each project has exactly one owner repository.

### Ownership

**Ownership** determines which repository is responsible for maintaining a project's proto files. Only the owner can push updates to the registry.

## Architecture Components

### Command Layer (`cmd/`)

The command layer implements the CLI interface using [Kong](https://github.com/alecthomas/kong). Each command is a separate struct implementing the command interface.

**Commands**:
- `init`: Initialize workspace
- `new`: Claim project ownership
- `pull`: Download projects from registry
- `push`: Publish owned projects to registry
- `verify`: Verify workspace integrity
- `list`: List available projects
- `mine`: List owned files

### Local Workspace (`internal/local/`)

Manages the local workspace configuration and operations:

- **Workspace**: Main workspace manager
- **Config**: Configuration file (`protato.yaml`) handling
- **Project Discovery**: Auto-discovery of projects in workspace

**Key Operations**:
- Read/write `protato.yaml`
- Discover projects in `protos/` directory
- Manage owned vs consumed projects

### Registry Cache (`internal/registry/`)

Manages the local cache of the registry repository:

- **Cache**: Local Git clone of registry
- **Snapshot**: Git commit hash tracking
- **Project Lookup**: Finding projects in registry

**Key Operations**:
- Clone/update registry cache
- Get registry snapshots
- Lookup projects and files
- Read project files from cache

### Git Operations (`internal/git/`)

Abstraction layer for Git operations:

- **Repository**: Git repository operations
- **Clone/Fetch**: Repository synchronization
- **Refs**: Reference management
- **Tree**: File tree operations

**Key Operations**:
- Clone repositories (bare and non-bare)
- Fetch updates
- Read file trees
- Manage refs and commits

### Protoc Integration (`internal/protoc/`)

Integration with protoc for import resolution:

- **Resolver**: Import path resolver
- **File Cache**: Cached proto file content
- **Path Transformation**: Service/import prefix handling

**Key Operations**:
- Resolve import paths
- Preload project files
- Transform paths for protoc

### Utilities (`internal/utils/`)

Shared utility functions:

- **File Operations**: File system utilities
- **Path Handling**: Path manipulation and validation
- **Pattern Matching**: Glob pattern matching
- **YAML**: YAML parsing
- **Validation**: Input validation

## Data Flow

### Pull Flow

```
User: protato pull team/service
  ↓
1. Resolve registry URL (env/config)
  ↓
2. Open/Create registry cache (internal/registry)
  ↓
3. Get latest snapshot (Git commit hash)
  ↓
4. Lookup project in registry (internal/registry)
  ↓
5. Read project files from cache (internal/git)
  ↓
6. Write files to workspace (internal/local)
  ↓
7. Create/update protato.lock (snapshot tracking)
```

### Push Flow

```
User: protato push
  ↓
1. Read workspace config (internal/local)
  ↓
2. Discover owned projects (internal/local)
  ↓
3. Read project files from workspace
  ↓
4. Open registry cache (internal/registry)
  ↓
5. Write files to cache (internal/git)
  ↓
6. Commit changes to cache
  ↓
7. Push to registry (internal/git)
```

### Verify Flow

```
User: protato verify
  ↓
1. Read workspace config (internal/local)
  ↓
2. For each consumed project:
  ↓
3. Read protato.lock (snapshot hash)
  ↓
4. Compare with registry snapshot
  ↓
5. Report mismatches
```

## Design Decisions

### Git-Based Registry

**Why Git?**
- Leverages existing infrastructure
- Built-in versioning and history
- Distributed and decentralized
- Familiar to developers

**Trade-offs**:
- Requires Git knowledge
- Network dependency for operations
- Large repositories can be slow

### Local-First Approach

**Why Local-First?**
- Works offline (after initial pull)
- Fast local operations
- No external service dependency
- Developer control

**Trade-offs**:
- Requires manual sync
- Potential for conflicts
- Cache management needed

### Ownership Model

**Why Single Owner?**
- Clear responsibility
- Prevents conflicts
- Simpler conflict resolution
- Clear maintenance path

**Trade-offs**:
- Less flexible than multi-owner
- Requires coordination for transfers

### Shallow Clones

**Why Shallow Clones?**
- Faster clone operations
- Less disk space
- Sufficient for proto files

**Trade-offs**:
- Limited history access
- HEAD detection complexity
- Some Git operations unavailable

## File Structure

```
workspace/
├── protato.yaml           # Workspace configuration
├── protos/
│   ├── owned_project/    # Owned by this repo
│   │   ├── v1/
│   │   │   └── api.proto
│   │   └── v2/
│   │       └── api.proto
│   └── consumed_project/  # Pulled from registry
│       ├── protato.lock   # Snapshot hash
│       ├── .gitattributes # Mark as generated
│       └── v1/
│           └── api.proto
```

## Registry Structure

```
registry.git/
├── protos/
│   └── team/
│       └── service/
│           ├── v1/
│           │   └── api.proto
│           └── v2/
│               └── api.proto
└── (Git objects)
```

## Error Handling

Protato uses structured error types (`internal/errors/`) for consistent error handling:

- **ErrNotGitRepository**: Not a Git repository
- **ErrProjectNotFound**: Project not found in registry
- **ErrProjectExists**: Project already exists
- **ErrInvalidProjectPath**: Invalid project path format

## Logging

Structured logging using [zerolog](https://github.com/rs/zerolog):

- **Levels**: Debug, Info, Warn, Error, Fatal
- **Context**: Logger injected into context
- **Format**: JSON (production) or human-readable (development)

## Testing Strategy

- **Unit Tests**: Test individual functions (beside source files)
- **Integration Tests**: Test component interactions (`tests/integration/`)
- **E2E Tests**: Test complete workflows (`tests/e2e/`)

## Future Considerations

- **Multi-owner support**: Allow multiple repositories to own a project
- **Conflict resolution**: Better handling of concurrent updates
- **Performance**: Optimize for large registries
- **Caching**: More aggressive caching strategies
- **Plugins**: Extensibility via plugins
