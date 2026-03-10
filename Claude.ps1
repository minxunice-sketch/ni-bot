# Claude Code-like startup script for Ni bot
# Usage: ./Claude.ps1

$ErrorActionPreference = "Stop"

# Ensure we are in the project root
$scriptPath = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $scriptPath

# Check for Go
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is not installed or not in PATH."
    exit 1
}

# Set environment variables
$env:NIBOT_ENABLE_SKILLS = "1"

Write-Output "Starting Ni bot (with skills enabled)..."
Write-Output "Use Ctrl+C to stop."

# Run the web server
go run cmd/web/main.go
