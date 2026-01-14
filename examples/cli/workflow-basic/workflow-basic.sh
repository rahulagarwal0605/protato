#!/usr/bin/env bash
#
# Protato Workflow Example (Auto-Discovery)
#
# This script demonstrates the core Protato workflow commands with auto-discovery:
# 1. Initialize workspace with auto-discovery (init)
# 2. Create a new project (new)
# 3. Push to registry (push) - projects auto-discovered from directory structure
# 4. Pull in another repository (pull)
#
# Usage: ./workflow-basic.sh
#

set -euo pipefail

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTATO="${SCRIPT_DIR}/../../protato"

if [[ ! -x "$PROTATO" ]]; then
    echo "Error: protato binary not found. Run 'make build' first."
    exit 1
fi

DEMO_DIR=$(mktemp -d)
echo -e "${GREEN}Workflow Demo (Auto-Discovery): ${DEMO_DIR}${NC}"

cleanup() {
    echo -e "\n${YELLOW}Demo directory: ${DEMO_DIR}${NC}"
    echo "Cleanup: rm -rf ${DEMO_DIR}"
}
trap cleanup EXIT

cd "$DEMO_DIR"

# Setup Registry
echo -e "\n${BLUE}Setting up registry...${NC}"
mkdir -p registry-work
cd registry-work
git init --initial-branch=main --quiet
mkdir -p protos
git add .
git commit --no-verify -m "Initialize registry" --quiet
cd "$DEMO_DIR"
git clone --bare registry-work registry.git --quiet
rm -rf registry-work
REGISTRY_URL="file://${DEMO_DIR}/registry.git"

# Step 1: Initialize Workspace with Auto-Discovery
echo -e "\n${BLUE}Step 1: Initialize workspace with auto-discovery (init)${NC}"
mkdir -p producer
cd producer
git init --initial-branch=main --quiet
git remote add origin "file://${DEMO_DIR}/producer-origin"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" init --skip-prompts
echo "✓ Workspace initialized with auto_discover: true"
echo "  Projects will be automatically discovered from protos/ directory"

# Step 2: Create a New Project
echo -e "\n${BLUE}Step 2: Create a new project (new)${NC}"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" new payments/api
echo "✓ Project created"

# Step 3: Create Proto Files
echo -e "\n${BLUE}Step 3: Create proto files${NC}"
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
echo "✓ Proto file created"

# Step 4: Push to Registry (Auto-Discovery)
echo -e "\n${BLUE}Step 4: Push to registry (push)${NC}"
echo "  With auto-discovery, payments/api is automatically detected from protos/payments/api/"
git add .
git commit --no-verify -m "Add payment proto" --quiet
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" push
echo "✓ Pushed to registry (project auto-discovered)"

# Step 5: Pull in Consumer
echo -e "\n${BLUE}Step 5: Pull in consumer repository (pull)${NC}"
cd "$DEMO_DIR"
mkdir -p consumer
cd consumer
git init --initial-branch=main --quiet
git remote add origin "file://${DEMO_DIR}/consumer-origin"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" init --skip-prompts
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" pull payments/api
echo "✓ Pulled from registry"

echo -e "\n${GREEN}✓ Workflow Demo (Auto-Discovery) Complete!${NC}"
echo "Files pulled:"
find protos -type f
echo -e "\n${YELLOW}Note: With auto-discovery, projects are automatically detected from directory structure.${NC}"
echo -e "${YELLOW}See ../workflow-advanced/ for the same workflow without auto-discovery.${NC}"
