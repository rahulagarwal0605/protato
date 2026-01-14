# CLI Examples

This directory contains command-line examples demonstrating how to use Protato in various scenarios.

## Available Examples

### [Workflow Basic](workflow-basic/)

**Core workflow commands with auto-discovery!** Demonstrates the essential Protato commands with automatic project detection:
- Initialize workspace with auto-discovery (`init`)
- Create a new project (`new`)
- Push to registry (`push`) - projects auto-discovered from directory structure
- Pull in another repository (`pull`)

**Key Feature**: Projects are automatically discovered from your `protos/` directory structure. No need to manually specify projects in `protato.yaml`.

**Run**: `cd workflow-basic && ./workflow-basic.sh`

### [Workflow Advanced](workflow-advanced/)

**Core workflow commands without auto-discovery!** Demonstrates the essential Protato commands with explicit project specification:
- Initialize workspace without auto-discovery (`init --no-auto-discover`)
- Create a new project (`new`)
- Push to registry (`push`) - only explicitly specified projects
- Pull in another repository (`pull`)

**Key Feature**: Projects must be explicitly listed in `protato.yaml`. Full control over which projects are managed.

**Run**: `cd workflow-advanced && ./workflow-advanced.sh`

### [Inspect](inspect/)

**Inspection and discovery commands!** Learn how to explore your workspace:
- List owned files (`mine`)
- List available projects (`list`)
- Verify workspace integrity (`verify`)

**Run**: `cd inspect && ./inspect.sh`

## Quick Comparison

| Example | Focus | Key Feature |
|---------|-------|-------------|
| **Workflow Basic** | Core workflow | Auto-discovery enabled |
| **Workflow Advanced** | Core workflow | Explicit project specification |
| **Inspect** | Discovery & verification | Workspace exploration |

## Prerequisites

All examples require:
- Go 1.24+ installed
- Git installed
- Protato built (`make build` from project root)

## Running Examples

Each example directory contains:
- **README.md** - Detailed documentation
- **\*.sh** - Executable demonstration script
- **protato.yaml** - Example configuration file

To run an example:
```bash
cd examples/cli/<example-name>
./<script-name>.sh
```

## Related Documentation

- [Main Examples README](../README.md) - All examples overview
- [CI/CD Integration](../ci-cd/README.md) - Automated workflows
- [Command Reference](../../../docs/COMMAND_REFERENCE.md) - Complete command reference
