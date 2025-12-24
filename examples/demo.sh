#!/usr/bin/env bash
#
# Protato Demo Script
# 
# This script demonstrates the complete workflow with:
# 1. A registry repository (central storage)
# 2. A producer service (owns and pushes protos)
# 3. A consumer service (pulls and uses protos)
#
# Usage: ./demo.sh
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_step() {
    echo -e "\n${BLUE}==>${NC} ${GREEN}$1${NC}"
}

log_info() {
    echo -e "    ${YELLOW}→${NC} $1"
}

# Get the protato binary path
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTATO="${SCRIPT_DIR}/../protato"

if [[ ! -x "$PROTATO" ]]; then
    echo -e "${RED}Error: protato binary not found at $PROTATO${NC}"
    echo "Please run 'make build' first"
    exit 1
fi

# Create demo directory
DEMO_DIR=$(mktemp -d)
echo -e "${GREEN}Creating demo in: ${DEMO_DIR}${NC}"

cleanup() {
    echo -e "\n${YELLOW}Demo directory: ${DEMO_DIR}${NC}"
    echo "To clean up: rm -rf ${DEMO_DIR}"
}
trap cleanup EXIT

cd "$DEMO_DIR"

# =============================================================================
# Step 1: Create the Registry Repository
# =============================================================================
log_step "Creating registry repository"

# Create a temporary working repo first
mkdir -p registry-work
cd registry-work
git init --initial-branch=main

log_info "Creating registry config"
cat > protato.registry.yaml << 'EOF'
ignores:
  - '**/BUILD'
  - '**/*.bak'
committer:
  name: Proto Bot
  email: proto-bot@example.com
EOF

mkdir -p protos

git add .
git commit --no-verify -m "Initialize protato registry"

cd "$DEMO_DIR"

# Clone as bare repository (so it can accept pushes)
git clone --bare registry-work registry.git
rm -rf registry-work

REGISTRY_URL="file://${DEMO_DIR}/registry.git"
log_info "Registry URL: ${REGISTRY_URL}"

# =============================================================================
# Step 2: Create the Producer Service
# =============================================================================
log_step "Creating producer service repository"

mkdir -p producer-service
cd producer-service
git init --initial-branch=main

# Create a remote so protato can get the origin URL
git remote add origin "file://${DEMO_DIR}/producer-service-origin"

log_info "Initializing protato"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" init --projects payments/api

log_info "Creating proto files"
mkdir -p protos/payments/api/v1

cat > protos/payments/api/v1/payment.proto << 'EOF'
syntax = "proto3";

package payments.api.v1;

option go_package = "github.com/example/payments/api/v1;paymentsv1";

// PaymentRequest represents a payment request
message PaymentRequest {
  string payment_id = 1;
  int64 amount_cents = 2;
  string currency = 3;
  string description = 4;
}

// PaymentResponse represents a payment response
message PaymentResponse {
  string payment_id = 1;
  PaymentStatus status = 2;
  string transaction_id = 3;
}

// PaymentStatus represents the status of a payment
enum PaymentStatus {
  PAYMENT_STATUS_UNSPECIFIED = 0;
  PAYMENT_STATUS_PENDING = 1;
  PAYMENT_STATUS_COMPLETED = 2;
  PAYMENT_STATUS_FAILED = 3;
}

// PaymentService handles payment operations
service PaymentService {
  rpc ProcessPayment(PaymentRequest) returns (PaymentResponse);
  rpc GetPaymentStatus(GetPaymentStatusRequest) returns (PaymentResponse);
}

message GetPaymentStatusRequest {
  string payment_id = 1;
}
EOF

cat > protos/payments/api/v1/types.proto << 'EOF'
syntax = "proto3";

package payments.api.v1;

option go_package = "github.com/example/payments/api/v1;paymentsv1";

// Money represents a monetary amount
message Money {
  int64 units = 1;
  int32 nanos = 2;
  string currency_code = 3;
}

// Address represents a billing/shipping address
message Address {
  string line1 = 1;
  string line2 = 2;
  string city = 3;
  string state = 4;
  string postal_code = 5;
  string country = 6;
}
EOF

log_info "Files created:"
"$PROTATO" mine

git add .
git commit --no-verify -m "Add payment proto files"

log_info "Pushing to registry"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" push

echo -e "${GREEN}✓ Producer service set up and pushed to registry${NC}"

cd "$DEMO_DIR"

# =============================================================================
# Step 3: Verify Registry Contents
# =============================================================================
log_step "Verifying registry contents"

cd registry.git
git log --oneline -3
echo ""
log_info "Registry tree:"
git ls-tree -r --name-only HEAD | head -20

cd "$DEMO_DIR"

# =============================================================================
# Step 4: Create the Consumer Service
# =============================================================================
log_step "Creating consumer service repository"

mkdir -p consumer-service
cd consumer-service
git init --initial-branch=main
git remote add origin "file://${DEMO_DIR}/consumer-service-origin"

log_info "Initializing protato"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" init

log_info "Listing available projects"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" list

log_info "Pulling payments/api"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" pull payments/api

log_info "Pulled files:"
find protos -type f

log_info "Lock file content:"
cat protos/payments/api/protato.lock

git add .
git commit --no-verify -m "Pull payment protos"

echo -e "${GREEN}✓ Consumer service set up with pulled protos${NC}"

cd "$DEMO_DIR"

# =============================================================================
# Step 5: Verify Consumer Can Access Protos
# =============================================================================
log_step "Verifying consumer has the proto files"

cd consumer-service
echo ""
echo "Proto files available to consumer:"
echo "=================================="
find protos -name "*.proto" -exec echo "  {}" \;

echo ""
echo "Sample content (payment.proto):"
echo "================================"
head -20 protos/payments/api/v1/payment.proto

cd "$DEMO_DIR"

# =============================================================================
# Step 6: Demonstrate Update Flow
# =============================================================================
log_step "Demonstrating update flow"

cd producer-service

log_info "Producer adds a new proto file"
cat > protos/payments/api/v1/refund.proto << 'EOF'
syntax = "proto3";

package payments.api.v1;

option go_package = "github.com/example/payments/api/v1;paymentsv1";

// RefundRequest represents a refund request
message RefundRequest {
  string payment_id = 1;
  int64 amount_cents = 2;
  string reason = 3;
}

// RefundResponse represents a refund response  
message RefundResponse {
  string refund_id = 1;
  string status = 2;
}

// RefundService handles refund operations
service RefundService {
  rpc ProcessRefund(RefundRequest) returns (RefundResponse);
}
EOF

git add .
git commit --no-verify -m "Add refund proto"

log_info "Pushing update to registry"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" push

cd "$DEMO_DIR/consumer-service"

log_info "Consumer pulls latest"
PROTATO_REGISTRY_URL="$REGISTRY_URL" "$PROTATO" pull payments/api

log_info "Consumer now has:"
find protos -name "*.proto"

git add .
git commit --no-verify -m "Update payment protos"

echo -e "${GREEN}✓ Update flow complete${NC}"

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}                    DEMO COMPLETE!${NC}"
echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
echo ""
echo "Demo directory structure:"
echo ""
echo "  ${DEMO_DIR}/"
echo "  ├── registry.git/          # Central proto registry (bare)"
echo "  │   └── (git objects)"
echo "  ├── producer-service/      # Owns payments/api"
echo "  │   └── protos/"
echo "  │       └── payments/api/"
echo "  │           └── v1/*.proto"
echo "  └── consumer-service/      # Consumes payments/api"
echo "      └── protos/"
echo "          └── payments/api/  # Pulled from registry"
echo "              ├── protato.lock"
echo "              └── v1/*.proto"
echo ""
echo "To explore:"
echo "  cd ${DEMO_DIR}"
echo ""
echo "To clean up:"
echo "  rm -rf ${DEMO_DIR}"

