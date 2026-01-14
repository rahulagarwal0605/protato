# Inspect Example

This example demonstrates Protato's inspection and discovery commands: `mine`, `list`, and `verify`.

## What You'll Learn

- How to list files owned by your repository (`mine`)
- How to list available projects in the registry (`list`)
- How to verify workspace integrity (`verify`)

## Concepts

### Mine Command

The `mine` command lists all files owned by your repository. It helps you understand what your repository is responsible for publishing to the registry.

### List Command

The `list` command shows available projects. You can:
- List projects in the registry (default)
- List local projects (owned and pulled)
- List from cache without refreshing (`--offline`)

### Verify Command

The `verify` command checks that all pulled projects match their registry snapshots. It ensures workspace integrity and detects modifications.

## Quick Start

```bash
./inspect.sh
```

## Scenarios

### Scenario 1: List Owned Files (mine)

```bash
# List all owned files
protato mine

# List only project paths
protato mine --projects

# List with absolute paths
protato mine --absolute
```

### Scenario 2: List Available Projects (list)

```bash
# List all projects in registry
protato list

# List local projects (owned and pulled)
protato list --local

# List from cache without refreshing
protato list --offline
```

### Scenario 3: Verify Workspace (verify)

```bash
# Verify all pulled projects match their snapshots
protato verify

# If verification fails, it means:
# - Files were modified locally
# - Lock files are out of sync
# - Registry has been updated
```

## Manual Steps

### 1. List Owned Files

```bash
# After initializing and creating projects
protato mine --projects
# Output: payments/api

protato mine
# Output: protos/payments/api/v1/payment.proto
```

### 2. List Projects

```bash
# List projects in registry
protato list
# Output:
# payments/api
# orders/api

# List local projects
protato list --local
# Output:
# Owned projects:
#   payments/api
# Pulled projects:
#   orders/api (snapshot: abc1234)
```

### 3. Verify Workspace

```bash
# After pulling projects
protato verify
# Checks all protato.lock files match current registry state

# If you modify a pulled file
echo "# Modified" >> protos/orders/api/v1/order.proto
protato verify
# Output: Verification failed - files modified
```

## Files

- `inspect.sh` - Complete demonstration
- `protato.yaml` - Example configuration
- `README.md` - This file

## Best Practices

1. **Use `mine` regularly**: Understand what your repository owns
2. **Use `list` before pulling**: See what's available in the registry
3. **Verify in CI/CD**: Run `protato verify` in your pipelines
4. **Don't modify pulled files**: Always pull updates instead

## Related Examples

- [Workflow Basic](../workflow-basic/README.md) - Core workflow commands with auto-discovery (`init`, `new`, `push`, `pull`)
- [Workflow Advanced](../workflow-advanced/README.md) - Core workflow commands without auto-discovery (explicit project specification)
- [CI/CD Integration](../../ci-cd/README.md) - Automated verification
- [Command Reference](../../../docs/COMMAND_REFERENCE.md) - Complete command reference
