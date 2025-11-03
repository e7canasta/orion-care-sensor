#!/bin/bash
# Wrapper script to run Python worker with venv

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="$SCRIPT_DIR/../venv"

# Activate venv if exists
if [ -d "$VENV_DIR" ]; then
    source "$VENV_DIR/bin/activate"
fi

# Run Python worker with all arguments
exec python3 "$SCRIPT_DIR/person_detector.py" "$@"
