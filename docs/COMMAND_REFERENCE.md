# Protato Command Reference

Complete reference for all Protato commands with examples and scenarios.

## Table of Contents

- [init](#init) - Initialize workspace
- [new](#new) - Claim project ownership
- [pull](#pull) - Pull projects from registry
- [push](#push) - Push projects to registry
- [verify](#verify) - Verify workspace integrity
- [list](#list) - List projects
- [mine](#mine) - List owned files

## init

Initialize a Protato workspace in the current repository.

### Basic Usage

```bash
# Interactive initialization
protato init

# Non-interactive with defaults
protato init --skip-prompts
```

### Scenarios

#### Scenario 1: Basic Initialization
```bash
protato init
# Creates protato.yaml with default settings
```

#### Scenario 2: Initialize with Project
```bash
protato init --project payments/api
# Creates workspace and claims ownership of payments/api
```

#### Scenario 3: Multiple Projects
```bash
protato init --project team/service1 --project team/service2
# Claims ownership of multiple projects
```

#### Scenario 4: Custom Directories
```bash
protato init --owned-dir my-protos --vendor-dir dependencies
# Uses custom directories instead of defaults
```

#### Scenario 5: With Ignores
```bash
protato init --ignore '**/BUILD' --ignore '**/*.swp'
# Sets up ignore patterns
```

#### Scenario 6: Disable Auto-discovery
```bash
protato init --no-auto-discover --project team/**
# Disables auto-discovery, uses explicit patterns
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `--project` | Project patterns to own | Auto-discovered |
| `--ignore` | Ignore patterns | None |
| `--service` | Service name for namespacing | Repository name |
| `--owned-dir` | Directory for owned protos | `protos` |
| `--vendor-dir` | Directory for consumed protos | `protos` |
| `--skip-prompts` | Use defaults, skip prompts | `false` |
| `--no-auto-discover` | Disable auto-discovery | `false` |
| `--force` | Overwrite existing config | `false` |

## new

Claim ownership of a new project path.

### Basic Usage

```bash
protato new team/service
```

### Scenarios

#### Scenario 1: Claim Single Project
```bash
protato new payments/api
# Adds payments/api to owned projects
```

#### Scenario 2: Claim Multiple Projects
```bash
protato new team/service1 team/service2
# Claims multiple projects at once
```

### Options

None - project path(s) are positional arguments.

## pull

Pull projects from the registry.

### Basic Usage

```bash
# Pull single project
protato pull team/service

# Pull multiple projects
protato pull team/service1 team/service2
```

### Scenarios

#### Scenario 1: Pull Single Project
```bash
protato pull payments/api
# Pulls payments/api and its dependencies
```

#### Scenario 2: Pull Multiple Projects
```bash
protato pull payments/api orders/api
# Pulls multiple projects
```

#### Scenario 3: Pull with Dependencies
```bash
protato pull team/service
# Automatically pulls transitive dependencies
```

### Options

None - project path(s) are positional arguments.

## push

Push owned projects to the registry.

### Basic Usage

```bash
# Push all owned projects
protato push
```

### Scenarios

#### Scenario 1: Push All Projects
```bash
protato push
# Pushes all projects listed in protato.yaml
```

#### Scenario 2: Push After Changes
```bash
# Make changes to proto files
vim protos/payments/api/v1/payment.proto

# Commit changes
git add .
git commit -m "Update payment proto"

# Push to registry
protato push
```

#### Scenario 3: Push in CI/CD
```bash
# In GitHub Actions or CI pipeline
protato push
# Uses PROTATO_REGISTRY_URL environment variable
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `--retries` | Number of push retries | 5 |
| `--retry-delay` | Delay between retries | 200ms |

### Environment Variables

- `PROTATO_PUSH_RETRIES`: Override retry count
- `PROTATO_PUSH_RETRY_DELAY`: Override retry delay

## verify

Verify workspace integrity.

### Basic Usage

```bash
protato verify
```

### Scenarios

#### Scenario 1: Verify Workspace
```bash
protato verify
# Checks all pulled projects match registry snapshots
```

#### Scenario 2: Detect Modifications
```bash
# Modify a pulled file
echo "# Modified" >> protos/payments/api/v1/payment.proto

# Verify detects the change
protato verify
# Output: Verification failed - files modified
```

### Options

None.

## list

List available projects.

### Basic Usage

```bash
# List registry projects
protato list

# List local projects
protato list --local
```

### Scenarios

#### Scenario 1: List Registry Projects
```bash
protato list
# Lists all projects available in registry
```

#### Scenario 2: List Local Projects
```bash
protato list --local
# Lists owned and pulled projects in workspace
```

#### Scenario 3: List Offline
```bash
protato list --offline
# Lists from cache without refreshing registry
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `--local` | List local projects only | `false` |
| `--offline` | Don't refresh registry | `false` |

## mine

List files owned by this repository.

### Basic Usage

```bash
# List all owned files
protato mine

# List project paths only
protato mine --projects

# List with absolute paths
protato mine --absolute
```

### Scenarios

#### Scenario 1: List All Files
```bash
protato mine
# Lists all proto files in owned projects
```

#### Scenario 2: List Project Paths
```bash
protato mine --projects
# Lists only project paths (e.g., payments/api)
```

#### Scenario 3: Absolute Paths
```bash
protato mine --absolute
# Lists files with absolute paths
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `--projects` | List project paths only | `false` |
| `--absolute` | Print absolute paths | `false` |

## Global Options

All commands support these global options:

| Option | Description | Default |
|--------|-------------|---------|
| `-v, --verbosity` | Increase verbosity (can repeat) | 0 |
| `-C, --dir` | Change directory before running | Current dir |
| `--version` | Print version information | N/A |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PROTATO_REGISTRY_URL` | Registry Git URL | Required |
| `PROTATO_REGISTRY_CACHE` | Cache directory | `~/.cache/protato/registry` |
| `PROTATO_VERBOSITY` | Verbosity level (0-3) | 0 |
| `PROTATO_PUSH_RETRIES` | Push retry count | 5 |
| `PROTATO_PUSH_RETRY_DELAY` | Push retry delay | 200ms |

## Examples

See [examples/README.md](../examples/README.md) and the [CLI examples](../examples/cli/) for comprehensive examples.
