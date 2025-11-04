#!/bin/bash

# template-replace.sh - Replace template variables in module files
# Usage: ./scripts/template-replace.sh <module-dir> <key> <value>

set -e

if [ $# -ne 3 ]; then
    echo "Usage: $0 <module-dir> <key> <value>"
    echo "Example: $0 stream-capture MODULE_NAME \"Stream Capture\""
    exit 1
fi

MODULE_DIR="$1"
KEY="$2"
VALUE="$3"

WORKSPACE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE_PATH="$WORKSPACE_ROOT/modules/$MODULE_DIR"

if [ ! -d "$MODULE_PATH" ]; then
    echo "Error: Module not found: $MODULE_PATH"
    exit 1
fi

# Replace in all markdown files
find "$MODULE_PATH" -type f \( -name "*.md" -o -name "*.go" \) -exec sed -i "s/{{${KEY}}}/${VALUE}/g" {} +

echo "✅ Replaced {{$KEY}} → $VALUE in modules/$MODULE_DIR/"
