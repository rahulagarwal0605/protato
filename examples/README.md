# Protato Examples

This directory contains example scripts and use cases demonstrating how to use Protato in real-world scenarios.

## Quick Navigation

- 🖥️ **[CLI Examples](cli/)** - Command-line usage examples
  - 🔄 **[Workflow Basic](cli/workflow-basic/)** - Core commands with auto-discovery: init, new, push, pull
  - 🔄 **[Workflow Advanced](cli/workflow-advanced/)** - Core commands without auto-discovery: explicit project specification
  - 🔍 **[Inspect](cli/inspect/)** - Inspection commands: mine, list, verify
- 🔄 **[CI/CD Integration](ci-cd/)** - Automated workflows
- 📚 **[Command Reference](../docs/COMMAND_REFERENCE.md)** - Complete command docs

## Example Use Cases

Each example is self-contained with its own directory containing:
- **README.md** - Documentation and explanation
- **\*.sh** - Executable demonstration script
- **protato.yaml** - Example configuration file

## CLI Examples

### [Workflow Basic](cli/workflow-basic/)

**Core workflow commands with auto-discovery!** Demonstrates the essential Protato commands with automatic project detection:
- Initialize workspace with auto-discovery (`init`)
- Create a new project (`new`)
- Push to registry (`push`) - projects auto-discovered from directory structure
- Pull in another repository (`pull`)

**Key Feature**: Projects are automatically discovered from your `protos/` directory structure. No need to manually specify projects in `protato.yaml`.

**Files**: `workflow-basic.sh`, `protato.yaml`, `README.md`

**Run**: `cd cli/workflow-basic && ./workflow-basic.sh`

### [Workflow Advanced](cli/workflow-advanced/)

**Core workflow commands without auto-discovery!** Demonstrates the essential Protato commands with explicit project specification:
- Initialize workspace without auto-discovery (`init --no-auto-discover`)
- Create a new project (`new`)
- Push to registry (`push`) - only explicitly specified projects
- Pull in another repository (`pull`)

**Key Feature**: Projects must be explicitly listed in `protato.yaml`. Full control over which projects are managed.

**Files**: `workflow-advanced.sh`, `protato.yaml`, `README.md`

**Run**: `cd cli/workflow-advanced && ./workflow-advanced.sh`

### [Inspect](cli/inspect/)

**Inspection and discovery commands!** Learn how to explore your workspace:
- List owned files (`mine`)
- List available projects (`list`)
- Verify workspace integrity (`verify`)

**Files**: `inspect.sh`, `protato.yaml`, `README.md`

**Run**: `cd cli/inspect && ./inspect.sh`

### [CI/CD Integration](ci-cd/)

Integrate Protato into your CI/CD pipeline:
- GitHub Actions workflows
- Automated proto pushing on changes
- Build with pulled dependencies
- Workspace verification in pipelines

**Files**: 
- `github-workflows/protato-push-with-github-app.yml` - **RECOMMENDED** - GitHub App (organizations)
- `github-workflows/protato-push-with-pat.yml` - Personal Access Token (cross-repo)
- `github-workflows/protato-push-with-ssh.yml` - SSH Key authentication (most secure)
- `github-workflows/protato-push-with-deploy-key.yml` - Deploy Key
- `github-workflows/README.md` - Complete CI/CD documentation

**Setup**: Copy workflow from `github-workflows/` to `.github/workflows/` in your repo

## Legacy Examples

### all-commands-demo.sh

Demonstrates **ALL** protato commands with various scenarios:
- **init**: 4 scenarios (basic, with project, multiple projects, custom directories)
- **new**: Claim project ownership
- **push**: Push projects to registry
- **list**: 3 scenarios (registry, local, offline)
- **pull**: 2 scenarios (single, multiple)
- **mine**: 3 scenarios (all files, projects only, absolute paths)
- **verify**: 2 scenarios (normal, detect modifications)

**Usage**:
```bash
# Build protato first
make build

# Run the comprehensive demo
./all-commands-demo.sh
```

## Use Cases

### Basic Workflow

**With Auto-Discovery** (recommended):
```bash
# 1. Initialize workspace with auto-discovery
protato init

# 2. Create a new project
protato new team/service

# 3. Create proto files
mkdir -p protos/team/service/v1
cat > protos/team/service/v1/api.proto << 'EOF'
syntax = "proto3";
package team.service.v1;
// ... your proto definitions
EOF

# 4. Push to registry (project auto-discovered)
protato push

# 5. In another repository, pull the project
protato pull team/service
```

**Without Auto-Discovery** (explicit control):
```bash
# 1. Initialize workspace without auto-discovery
protato init --no-auto-discover

# 2. Create a new project (adds to protato.yaml)
protato new team/service

# 3. Create proto files
mkdir -p protos/team/service/v1
cat > protos/team/service/v1/api.proto << 'EOF'
syntax = "proto3";
package team.service.v1;
// ... your proto definitions
EOF

# 4. Push to registry (only explicitly listed projects)
protato push

# 5. In another repository, pull the project
protato pull team/service
```

See [Workflow Basic](cli/workflow-basic/) and [Workflow Advanced](cli/workflow-advanced/) for complete examples.


### Using Custom Registry

```bash
# Set registry URL via environment variable
export PROTATO_REGISTRY_URL=https://github.com/your-org/proto-registry.git

# Or use file:// for local registry
export PROTATO_REGISTRY_URL=file:///path/to/registry.git

# All commands will use this registry
protato pull team/service
```

### Workspace Configuration

Create `protato.yaml` in your repository:

```yaml
# Service name for registry namespacing
service: my-service

# Directory configuration
directories:
  owned: protos
  vendor: vendor-proto

# Auto-discovery (automatically discovers projects from owned directory)
auto_discover: true

# Files to ignore during push/pull
ignores:
  - '**/BUILD'
  - '**/*.swp'
  - '**/generated/**'
```

### CI/CD Integration

#### GitHub Actions

See [ci-cd/github-workflows/](ci-cd/github-workflows/) for complete CI/CD examples and documentation.

**Important**: Copy workflow files from `examples/ci-cd/github-workflows/` to `.github/workflows/` in your repository root.

**Setup**:
1. Add `PROTATO_REGISTRY_URL` to your repository secrets
2. Copy the workflow file to `.github/workflows/`
3. Customize as needed

**Example**:
```yaml
- name: Push protos
  run: protato push
  env:
    PROTATO_REGISTRY_URL: ${{ secrets.PROTATO_REGISTRY_URL }}
```

#### Generic CI/CD

```bash
# In your CI pipeline
#!/bin/bash
set -e

# Pull dependencies
protato pull team/service1 team/service2

# Verify workspace integrity
protato verify

# Build your project (protoc will find the pulled protos)
protoc --proto_path=protos --go_out=. protos/**/*.proto
```

## Advanced Examples

### Custom Cache Directory

```bash
# Use custom cache location
export PROTATO_REGISTRY_CACHE=/tmp/protato-cache
protato pull team/service
```

### Verbose Logging

```bash
# Enable debug logging
protato -vvv pull team/service
```

### Listing Projects

```bash
# List all projects in registry
protato list

# List local projects only
protato list --local

# List owned projects
protato mine --projects
```

## Example Requirements

All examples should:
- ✅ **Be clear and well-commented** - Easy to understand
- ✅ **Be self-contained** - Minimal external dependencies
- ✅ **Demonstrate real-world usage** - Practical scenarios
- ✅ **Include usage instructions** - How to run them
- ✅ **Follow best practices** - Show recommended patterns

## Contributing Examples

We welcome example contributions! If you have an interesting use case:

1. **Create a new directory** under `examples/`
2. **Add README.md** with documentation
3. **Add demonstration script** (`.sh` file)
4. **Add protato.yaml** if applicable
5. **Update this README** with your example
6. **Submit a pull request**

See [CONTRIBUTING.md](../CONTRIBUTING.md) for more details.

## Command Reference

For detailed command reference with all options and scenarios, see [COMMAND_REFERENCE.md](../docs/COMMAND_REFERENCE.md).

## Related Documentation

- [Main README](../README.md) - Project overview and installation
- [Architecture Docs](../docs/ARCHITECTURE.md) - System design
- [Contributing Guide](../CONTRIBUTING.md) - How to contribute
- [Command Reference](../docs/COMMAND_REFERENCE.md) - Complete command reference
