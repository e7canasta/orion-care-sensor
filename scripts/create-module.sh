#!/bin/bash

# create-module.sh - Generate new module structure from templates
# Usage: ./scripts/create-module.sh <module-name> <bounded-context>

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
error() {
    echo -e "${RED}❌ Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${GREEN}✅ $1${NC}"
}

warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Validate arguments
if [ $# -lt 2 ]; then
    error "Usage: $0 <module-name> <bounded-context> [sprint-number]"
fi

MODULE_NAME="$1"
BOUNDED_CONTEXT="$2"
SPRINT_NUMBER="${3:-Sprint X.Y}"

# Convert module-name to directory format (e.g., "Stream Capture" -> "stream-capture")
MODULE_DIR=$(echo "$MODULE_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')

# Workspace root
WORKSPACE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE_PATH="$WORKSPACE_ROOT/modules/$MODULE_DIR"
TEMPLATE_PATH="$WORKSPACE_ROOT/BACKLOG/TEMPLATES/module"

# Check if module already exists
if [ -d "$MODULE_PATH" ]; then
    error "Module already exists: $MODULE_PATH"
fi

info "Creating module: $MODULE_NAME"
info "Bounded Context: $BOUNDED_CONTEXT"
info "Directory: modules/$MODULE_DIR"
echo ""

# Create directory structure
info "Creating directory structure..."
mkdir -p "$MODULE_PATH"
mkdir -p "$MODULE_PATH/internal"
mkdir -p "$MODULE_PATH/docs/proposals"

# Create go.mod
info "Generating go.mod..."
cat > "$MODULE_PATH/go.mod" <<EOF
module github.com/e7canasta/orion-care-sensor/modules/$MODULE_DIR

go 1.21
EOF

# Copy and customize templates
info "Generating CLAUDE.md..."
cp "$TEMPLATE_PATH/CLAUDE.md.template" "$MODULE_PATH/CLAUDE.md"

info "Generating README.md..."
cp "$TEMPLATE_PATH/README.md.template" "$MODULE_PATH/README.md"

info "Generating BACKLOG.md..."
cp "$TEMPLATE_PATH/BACKLOG.md.template" "$MODULE_PATH/BACKLOG.md"

info "Generating docs/DESIGN.md..."
cp "$TEMPLATE_PATH/docs/DESIGN.md.template" "$MODULE_PATH/docs/DESIGN.md"

# Create placeholder files
info "Creating placeholder files..."

cat > "$MODULE_PATH/types.go" <<EOF
package ${MODULE_DIR//-/}

// Types for $MODULE_NAME module
EOF

cat > "$MODULE_PATH/${MODULE_DIR}_test.go" <<EOF
package ${MODULE_DIR//-/}_test

import "testing"

func TestPlaceholder(t *testing.T) {
    t.Skip("TODO: Implement tests")
}
EOF

# Create .gitkeep in internal/
touch "$MODULE_PATH/internal/.gitkeep"
touch "$MODULE_PATH/docs/proposals/.gitkeep"

# Update go.work
info "Updating go.work..."
GO_WORK="$WORKSPACE_ROOT/go.work"

if [ ! -f "$GO_WORK" ]; then
    warn "go.work not found, creating..."
    cat > "$GO_WORK" <<EOF
go 1.21

use (
    ./modules/$MODULE_DIR
)
EOF
else
    # Check if module already in go.work
    if ! grep -q "./modules/$MODULE_DIR" "$GO_WORK"; then
        # Add to go.work (before closing parenthesis)
        sed -i "/^)/i\\    ./modules/$MODULE_DIR" "$GO_WORK"
        info "Added module to go.work"
    else
        warn "Module already in go.work"
    fi
fi

# Summary
echo ""
info "Module created successfully! 🚀"
echo ""
echo "📁 Structure created:"
echo "   modules/$MODULE_DIR/"
echo "   ├── go.mod"
echo "   ├── CLAUDE.md"
echo "   ├── README.md"
echo "   ├── BACKLOG.md"
echo "   ├── types.go"
echo "   ├── ${MODULE_DIR}_test.go"
echo "   ├── docs/"
echo "   │   ├── DESIGN.md"
echo "   │   └── proposals/"
echo "   └── internal/"
echo ""
echo "📝 Next steps:"
echo "   1. Edit modules/$MODULE_DIR/CLAUDE.md with module-specific info"
echo "   2. Edit modules/$MODULE_DIR/BACKLOG.md with sprint tasks"
echo "   3. Edit modules/$MODULE_DIR/docs/DESIGN.md with design decisions"
echo "   4. Implement module in modules/$MODULE_DIR/"
echo "   5. Run: cd modules/$MODULE_DIR && go test ./..."
echo ""
echo "🎸 Template variables to replace in generated files:"
echo "   {{MODULE_NAME}} → $MODULE_NAME"
echo "   {{MODULE_DIR}} → $MODULE_DIR"
echo "   {{BOUNDED_CONTEXT}} → $BOUNDED_CONTEXT"
echo "   {{SPRINT_NUMBER}} → $SPRINT_NUMBER"
echo "   {{DATE}} → $(date +%Y-%m-%d)"
echo ""
warn "Remember to replace template placeholders ({{...}}) in generated files!"
