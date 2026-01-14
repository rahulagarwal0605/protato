# Workflow Example (Manual Project Specification)

This example demonstrates the core Protato workflow commands **without auto-discovery**: `init`, `new`, `push`, and `pull`.

**Key Feature**: With `auto_discover: false`, you must explicitly specify all projects in `protato.yaml`. Protato will only push/pull projects that are explicitly listed.

## When to Use Manual Specification

- **Explicit Control**: You want full control over which projects are managed
- **Selective Projects**: You only want to manage specific projects, not all directories
- **Complex Structures**: Your directory structure doesn't match your project paths
- **Legacy Migration**: Migrating from a system that requires explicit project lists

## What You'll Learn

- How to initialize a Protato workspace without auto-discovery (`init --no-auto-discover`)
- How to explicitly specify projects in `protato.yaml`
- How to create and claim ownership of a new project (`new`)
- How to push only explicitly specified projects (`push`)
- How to pull proto files in another repository (`pull`)

## Prerequisites

- Go 1.24+ installed
- Git installed
- Protato built (`make build` from project root)

## Quick Start

```bash
# Run the workflow script
./workflow-advanced.sh
```

## Manual Steps

### 1. Initialize Workspace without Auto-Discovery

```bash
protato init --no-auto-discover
```

This creates a `protato.yaml` with `auto_discover: false`. Projects must be explicitly listed.

### 2. Create a New Project

```bash
protato new payments/api
```

This claims ownership of the `payments/api` project path and adds it to `protato.yaml`:

```yaml
projects:
  - payments/api
```

### 3. Create Proto Files

```bash
mkdir -p protos/payments/api/v1
cat > protos/payments/api/v1/payment.proto << 'EOF'
syntax = "proto3";
package payments.api.v1;
message PaymentRequest {
  string payment_id = 1;
  int64 amount_cents = 2;
  string currency = 3;
}
EOF
```

### 4. Push to Registry

```bash
# Commit your changes first
git add .
git commit -m "Add payment proto"

# Push to registry
protato push
```

**Important**: Only projects listed in `protato.yaml` will be pushed. Even if you have `protos/orders/api/`, it won't be pushed unless explicitly listed.

### 5. Pull in Another Repository

```bash
# Initialize workspace in consumer repository
protato init

# Pull the project
protato pull payments/api
```

## Configuration

See `protato.yaml` for the workspace configuration used in this example. Notice that `auto_discover: false` is set, and projects are explicitly listed:

**Key Configuration**:
```yaml
auto_discover: false  # Disable automatic project discovery
projects:
  - payments/api      # Explicitly specify projects
```

When you create `protos/payments/api/v1/payment.proto`, Protato will **not** automatically recognize it. You must explicitly add `payments/api` to the `projects` list (via `protato new` or manually editing `protato.yaml`).

## Comparison: Auto-Discovery vs Manual

| Feature | Auto-Discovery (`workflow-basic/`) | Manual (`workflow-advanced/`) |
|---------|-----------------------------------|------------------------------|
| Configuration | `auto_discover: true` | `auto_discover: false` |
| Project Detection | Automatic from directory structure | Must be explicitly listed |
| `protato.yaml` | No `projects` field needed | `projects` field required |
| Adding Projects | Just create directory structure | Must run `protato new` or edit YAML |
| Flexibility | Less control, more convenience | More control, more explicit |

## Files

- `workflow-advanced.sh` - Complete demonstration script
- `protato.yaml` - Example workspace configuration (manual specification)
- `README.md` - This file

## Next Steps

- Explore [Workflow Basic](../workflow-basic/README.md) - Same workflow with auto-discovery enabled
- Explore [Inspect](../inspect/README.md) - Learn about inspection commands (`mine`, `list`, `verify`)
- Explore [CI/CD Integration](../../ci-cd/README.md) - Automated workflows
- See [Complete Command Reference](../../../docs/COMMAND_REFERENCE.md) - All commands
