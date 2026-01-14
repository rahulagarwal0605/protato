# Workflow Example (Auto-Discovery)

This example demonstrates the core Protato workflow commands with **auto-discovery enabled**: `init`, `new`, `push`, and `pull`.

**Key Feature**: With `auto_discover: true`, Protato automatically discovers projects from your `owned` directory structure. You don't need to manually specify projects in `protato.yaml`.

## What You'll Learn

- How to initialize a Protato workspace with auto-discovery (`init`)
- How to create and claim ownership of a new project (`new`)
- How auto-discovery automatically detects projects from directory structure
- How to push proto files to the registry (`push`)
- How to pull proto files in another repository (`pull`)

## Prerequisites

- Go 1.24+ installed
- Git installed
- Protato built (`make build` from project root)

## Quick Start

```bash
# Run the workflow script
./workflow-basic.sh
```

## Manual Steps

### 1. Initialize Workspace

```bash
protato init
```

This creates a `protato.yaml` configuration file with default settings.

### 2. Create a New Project

```bash
protato new payments/api
```

This claims ownership of the `payments/api` project path. You can create multiple projects:

```bash
protato new payments/api orders/api
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

### 5. Pull in Another Repository

```bash
# Initialize workspace in consumer repository
protato init

# Pull the project
protato pull payments/api
```

## Configuration

See `protato.yaml` for the workspace configuration used in this example. Notice that `auto_discover: true` is set, which means projects are automatically discovered from the `protos/` directory structure.

**Key Configuration**:
```yaml
auto_discover: true  # Automatically discover projects from owned directory
```

When you create `protos/payments/api/v1/payment.proto`, Protato automatically recognizes `payments/api` as a project without needing to specify it in `protato.yaml`.

## Files

- `workflow-basic.sh` - Complete demonstration script
- `protato.yaml` - Example workspace configuration
- `README.md` - This file

## Next Steps

- Explore [Workflow Advanced](../workflow-advanced/README.md) - Same workflow without auto-discovery
- Explore [Inspect](../inspect/README.md) - Learn about inspection commands (`mine`, `list`, `verify`)
- Explore [CI/CD Integration](../../ci-cd/README.md) - Automated workflows
- See [Complete Command Reference](../../../docs/COMMAND_REFERENCE.md) - All commands
