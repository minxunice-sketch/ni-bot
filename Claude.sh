#!/bin/bash
# Claude Code-like startup script for Ni bot (Mac/Linux)
# Usage: ./Claude.sh

set -e

# Ensure we are in the project root
cd "$(dirname "$0")"

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH."
    exit 1
fi

# Set environment variables
export NIBOT_ENABLE_SKILLS="1"
export NIBOT_POLICY_ALLOW_FS_WRITE="true"
export NIBOT_POLICY_ALLOW_RUNTIME_EXEC="true"
export NIBOT_POLICY_ALLOW_SKILL_EXEC="true"
export NIBOT_POLICY_ALLOW_SKILL_INSTALL="true"
export NIBOT_POLICY_ALLOW_MEMORY="true"

echo "Starting Ni bot (with skills enabled)..."
echo "Use Ctrl+C to stop."

# Run the web server
go run cmd/web/main.go
