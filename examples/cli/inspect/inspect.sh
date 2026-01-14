#!/usr/bin/env bash
#
# Protato Inspect Example
#
# This script demonstrates inspection and discovery commands:
# 1. List owned files (mine)
# 2. List available projects (list)
# 3. Verify workspace integrity (verify)
#
# Usage: ./inspect.sh
#

set -euo pipefail

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
echo -e "${GREEN}Inspect Demo: ${DEMO_DIR}${NC}"

cleanup() {
    echo -e "\n${YELLOW}Demo directory: ${DEMO_DIR}${NC}"
    echo "Cleanup: rm -rf ${DEMO_DIR}"
}
trap cleanup EXIT

cd "$DEMO_DIR"

# Setup Registry
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

# Create producer
echo -e "\n${BLUE}Creating producer...${NC}"
mkdir -p producer
cd producer
git init --initial-branch=main --quiet
git remote add origin "file://${DEMO_DIR}/producer-origin"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" init --project payments/api --skip-prompts

mkdir -p protos/payments/api/v1
cat > protos/payments/api/v1/payment.proto << 'EOF'
syntax = "proto3";
package payments.api.v1;
message PaymentRequest {
  string payment_id = 1;
  int64 amount_cents = 2;
}
EOF

git add .
git commit --no-verify -m "v1: Initial payment proto" --quiet

# Push v1
echo -e "\n${BLUE}Pushing v1 (creating snapshot)...${NC}"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" push
SNAPSHOT_V1=$(cd "$DEMO_DIR/registry.git" && git rev-parse HEAD)
echo "✓ Snapshot v1: ${SNAPSHOT_V1:0:7}"

# Consumer pulls v1
echo -e "\n${BLUE}Consumer pulling v1...${NC}"
cd "$DEMO_DIR"
mkdir -p consumer
cd consumer
git init --initial-branch=main --quiet
git remote add origin "file://${DEMO_DIR}/consumer-origin"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" init --skip-prompts
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" pull payments/api
echo "✓ Pulled v1"
echo "Lock file:"
cat protos/payments/api/protato.lock

# Step 1: List Owned Files (mine)
echo -e "\n${BLUE}Step 1: List owned files (mine)${NC}"
cd "$DEMO_DIR/producer"
echo "Owned project paths:"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" mine --projects
echo -e "\nOwned files:"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" mine
echo "✓ Listed owned files"

# Step 2: List Available Projects (list)
echo -e "\n${BLUE}Step 2: List available projects (list)${NC}"
cd "$DEMO_DIR/consumer"
echo "Projects in registry:"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" list
echo -e "\nLocal projects:"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" list --local
echo "✓ Listed projects"

# Producer updates to v2
echo -e "\n${BLUE}Producer updating to v2...${NC}"
cd "$DEMO_DIR/producer"
echo "// Updated in v2" >> protos/payments/api/v1/payment.proto
git add .
git commit --no-verify -m "v2: Update payment proto" --quiet

# Push v2
echo -e "\n${BLUE}Pushing v2 (new snapshot)...${NC}"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" push
SNAPSHOT_V2=$(cd "$DEMO_DIR/registry.git" && git rev-parse HEAD)
echo "✓ Snapshot v2: ${SNAPSHOT_V2:0:7}"

# Verify consumer still has v1
echo -e "\n${BLUE}Verifying consumer has v1...${NC}"
cd "$DEMO_DIR/consumer"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" verify
echo "✓ Consumer verified (has v1)"

# Consumer updates to v2
echo -e "\n${BLUE}Consumer updating to v2...${NC}"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" pull payments/api
echo "✓ Updated to v2"
echo "New lock file:"
cat protos/payments/api/protato.lock

# Step 3: Verify Workspace (verify)
echo -e "\n${BLUE}Step 3: Verify workspace integrity (verify)${NC}"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" verify
echo "✓ Consumer verified (has v2)"

echo -e "\n${GREEN}✓ Inspect Demo Complete!${NC}"
echo "Snapshots:"
echo "  v1: ${SNAPSHOT_V1:0:7}"
echo "  v2: ${SNAPSHOT_V2:0:7}"
