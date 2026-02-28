# Ni Bot Windows PowerShell Startup Script
# Auto-generated on: 02/28/2026 17:47:57

Write-Host "🪟 Starting Ni Bot for Windows..." -ForegroundColor Green

# Check if in project directory
if (-not (Test-Path "go.mod")) {
    Write-Host "�?Error: Please run this script in the Ni Bot project directory" -ForegroundColor Red
    Write-Host "💡 Tip: First execute cd C:\path\to\ni-bot" -ForegroundColor Yellow
    exit 1
}

# Check Go environment
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "�?Go not installed, please install Go for Windows" -ForegroundColor Red
    Write-Host "📥 Download: https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Set environment variables
$env:GOPROXY = "https://goproxy.cn,direct"
$env:NIBOT_ENABLE_EXEC = "1"
$env:NIBOT_AUTO_APPROVE = "true"

# Display configuration info
Write-Host "🔧 Environment Configuration:" -ForegroundColor Cyan
Write-Host "   - Go Version: $(go version)"
Write-Host "   - Project Directory: $(Get-Location)"
Write-Host "   - GOPROXY: $env:GOPROXY"

# Install dependencies (if needed)
if (-not (Test-Path "vendor") -and -not (Test-Path "go.sum")) {
    Write-Host "📦 Installing dependencies..." -ForegroundColor Cyan
    go mod download
}

# Start Ni Bot
Write-Host "🚀 Starting Ni Bot..." -ForegroundColor Green
go run ./cmd/nibot
