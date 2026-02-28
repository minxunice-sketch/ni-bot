# Ni Bot Windows Environment Setup Tool
# Automatically detects environment and generates platform-specific startup scripts

param(
    [switch]$DetectEnvironment,
    [string]$Platform,
    [switch]$GenerateScripts,
    [switch]$CheckDependencies,
    [switch]$Auto,
    [switch]$Help
)

# Show help information
function Show-Help {
    Write-Host "Ni Bot Windows Environment Setup Tool" -ForegroundColor Cyan
    Write-Host "Usage: .\setup.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -DetectEnvironment   Automatically detect runtime environment"
    Write-Host "  -Platform <os>       Specify platform (windows|linux|macos)"
    Write-Host "  -GenerateScripts     Generate startup scripts"
    Write-Host "  -CheckDependencies  Check system dependencies"
    Write-Host "  -Auto                Full automatic installation and configuration"
    Write-Host "  -Help                Show help information"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\setup.ps1 -DetectEnvironment"
    Write-Host "  .\setup.ps1 -Platform windows -GenerateScripts"
    Write-Host "  .\setup.ps1 -Auto"
}

# Log function
function Write-Log {
    param(
        [string]$Message,
        [string]$Level = "INFO"
    )
    
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    
    switch ($Level) {
        "INFO" { 
            Write-Host "[$timestamp INFO] $Message" -ForegroundColor Blue
        }
        "SUCCESS" { 
            Write-Host "[$timestamp SUCCESS] $Message" -ForegroundColor Green
        }
        "WARNING" { 
            Write-Host "[$timestamp WARNING] $Message" -ForegroundColor Yellow
        }
        "ERROR" { 
            Write-Host "[$timestamp ERROR] $Message" -ForegroundColor Red
        }
    }
}

# Detect operating system
function Get-OSPlatform {
    if ($env:OS -like "Windows*") {
        return "windows"
    }
    
    # If running on Windows but detecting through WSL
    try {
        $uname = wsl uname -s 2>$null
        if ($uname -like "Linux*") {
            return "linux"
        }
        elseif ($uname -like "Darwin*") {
            return "macos"
        }
    }
    catch {
        # Ignore errors
    }
    
    return "windows"
}

# Check dependencies
function Test-Dependencies {
    Write-Log "Checking system dependencies..."
    
    $missingDeps = @()
    
    # Check Go
    if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
        $missingDeps += "Go"
    }
    else {
        $goVersion = go version
        Write-Log "Go installed: $goVersion" "SUCCESS"
    }
    
    # Check Git
    if (-not (Get-Command "git" -ErrorAction SilentlyContinue)) {
        $missingDeps += "Git"
    }
    else {
        $gitVersion = git --version
        Write-Log "Git installed: $gitVersion" "SUCCESS"
    }
    
    if ($missingDeps.Count -gt 0) {
        Write-Log "Missing dependencies: $($missingDeps -join ', ')" "WARNING"
        return $false
    }
    
    return $true
}

# Install dependencies (Windows)
function Install-WindowsDependencies {
    Write-Log "Installing Windows dependencies..."
    
    # Check Chocolatey
    if (-not (Get-Command "choco" -ErrorAction SilentlyContinue)) {
        Write-Log "Installing Chocolatey..."
        
        # Set execution policy
        Set-ExecutionPolicy Bypass -Scope Process -Force
        
        # Install Chocolatey
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
        
        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    }
    
    # Install Go
    if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
        Write-Log "Installing Go..."
        choco install golang -y
    }
    
    # Install Git
    if (-not (Get-Command "git" -ErrorAction SilentlyContinue)) {
        Write-Log "Installing Git..."
        choco install git -y
    }
    
    Write-Log "Windows dependencies installed successfully" "SUCCESS"
}

# Generate Windows PowerShell startup script
function New-WindowsStartScript {
    $scriptContent = @"
# Ni Bot Windows PowerShell Startup Script
# Auto-generated on: $(Get-Date)

Write-Host "🪟 Starting Ni Bot for Windows..." -ForegroundColor Green

# Check if in project directory
if (-not (Test-Path "go.mod")) {
    Write-Host "❌ Error: Please run this script in the Ni Bot project directory" -ForegroundColor Red
    Write-Host "💡 Tip: First execute cd C:\path\to\ni-bot" -ForegroundColor Yellow
    exit 1
}

# Check Go environment
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Go not installed, please install Go for Windows" -ForegroundColor Red
    Write-Host "📥 Download: https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Set environment variables
`$env:GOPROXY = "https://goproxy.cn,direct"
`$env:NIBOT_ENABLE_EXEC = "1"
`$env:NIBOT_AUTO_APPROVE = "true"

# Display configuration info
Write-Host "🔧 Environment Configuration:" -ForegroundColor Cyan
Write-Host "   - Go Version: `$(go version)"
Write-Host "   - Project Directory: `$(Get-Location)"
Write-Host "   - GOPROXY: `$env:GOPROXY"

# Install dependencies (if needed)
if (-not (Test-Path "vendor") -and -not (Test-Path "go.sum")) {
    Write-Host "📦 Installing dependencies..." -ForegroundColor Cyan
    go mod download
}

# Start Ni Bot
Write-Host "🚀 Starting Ni Bot..." -ForegroundColor Green
go run ./cmd/nibot
"@
    
    Set-Content -Path "start-windows.ps1" -Value $scriptContent
    Write-Log "Generated Windows startup script: start-windows.ps1" "SUCCESS"
}

# Generate Linux startup script
function New-LinuxStartScript {
    $scriptContent = @"
#!/bin/bash

# Ni Bot Linux Startup Script
# Auto-generated on: $(Get-Date)

echo "🐧 Starting Ni Bot for Linux..."

# Check if in project directory
if [ ! -f "go.mod" ]; then
    echo "❌ Error: Please run this script in the Ni Bot project directory"
    echo "💡 Tip: First execute cd /path/to/ni-bot"
    exit 1
fi

# Check Go environment
if ! command -v go &> /dev/null; then
    echo "❌ Go not installed, please install:"
    echo "   Ubuntu/Debian: sudo apt install golang git"
    echo "   CentOS/RHEL: sudo yum install golang git"
    echo "   Fedora: sudo dnf install golang git"
    exit 1
fi

# Set environment variables
export GOPROXY=https://goproxy.cn,direct
export NIBOT_ENABLE_EXEC=1
export NIBOT_AUTO_APPROVE=true

# Display configuration info
echo "🔧 Environment Configuration:"
echo "   - Go Version: `$(go version)`"
echo "   - Project Directory: `$(pwd)`"
echo "   - GOPROXY: `$GOPROXY`"

# Install dependencies (if needed)
if [ ! -d "vendor" ] && [ ! -f "go.sum" ]; then
    echo "📦 Installing dependencies..."
    go mod download
fi

# Start Ni Bot
echo "🚀 Starting Ni Bot..."
go run ./cmd/nibot
"@
    
    Set-Content -Path "start-linux.sh" -Value $scriptContent
    Write-Log "Generated Linux startup script: start-linux.sh" "SUCCESS"
}

# Generate Docker startup script
function New-DockerStartScript {
    $scriptContent = @"
#!/bin/bash

# Ni Bot Docker Startup Script
# Auto-generated on: $(Get-Date)

echo "🐳 Starting Ni Bot with Docker..."

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not installed"
    echo "📥 Installation guide: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check docker-compose
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose not installed"
    echo "📥 Installation guide: https://docs.docker.com/compose/install/"
    exit 1
fi

# Create necessary directories
mkdir -p workspace/data

# Check config file
if [ ! -f "config.yaml" ]; then
    echo "📋 Generating default config file..."
    cat > config.yaml << 'CONFIG_EOF'
llm:
  provider: deepseek
  base_url: https://api.deepseek.com/v1
  model: deepseek-chat
  api_key: ""
  log_level: full
CONFIG_EOF
fi

# Check docker-compose.yml
if [ ! -f "docker-compose.yml" ]; then
    echo "📋 Generating docker-compose.yml..."
    cat > docker-compose.yml << 'COMPOSE_EOF'
version: '3.8'
services:
  ni-bot:
    image: minxunice/ni-bot:latest
    ports:
      - "8080:8080"
    volumes:
      - ./workspace:/app/workspace
      - ./config.yaml:/app/workspace/data/config.yaml
    environment:
      - GOPROXY=https://goproxy.cn,direct
    restart: unless-stopped
COMPOSE_EOF
fi

# Start services
echo "🚀 Starting Docker containers..."
docker-compose up -d

echo "✅ Ni Bot started successfully"
echo "🌐 Access: http://localhost:8080"
echo "📋 View logs: docker-compose logs -f"
"@
    
    Set-Content -Path "start-docker.sh" -Value $scriptContent
    Write-Log "Generated Docker startup script: start-docker.sh" "SUCCESS"
}

# Main function
function Main {
    # Show help
    if ($Help) {
        Show-Help
        return
    }
    
    # Auto detect environment
    if ($DetectEnvironment -or $Auto) {
        $script:Platform = Get-OSPlatform
    }
    
    # If no platform specified, use default
    if ([string]::IsNullOrEmpty($Platform)) {
        $script:Platform = "windows"
    }
    
    Write-Log "Detected operating system: $Platform"
    
    # Check dependencies
    if ($CheckDependencies -or $Auto) {
        $depsOk = Test-Dependencies
        if (-not $depsOk -and $Auto) {
            if ($Platform -eq "windows") {
                Install-WindowsDependencies
            }
        }
    }
    
    # Generate startup scripts
    if ($GenerateScripts -or $Auto) {
        Write-Log "Generating startup scripts..."
        
        switch ($Platform) {
            "windows" {
                New-WindowsStartScript
            }
            "linux" {
                New-LinuxStartScript
            }
            "macos" {
                New-LinuxStartScript  # macOS uses similar script
                Rename-Item "start-linux.sh" "start-mac.sh" -Force
                Write-Log "Generated macOS startup script: start-mac.sh" "SUCCESS"
            }
            default {
                New-WindowsStartScript
            }
        }
        
        # Always generate Docker script
        New-DockerStartScript
        
        Write-Log "Startup scripts generated successfully!" "SUCCESS"
        Write-Host ""
        Write-Host "🚀 Use the following commands to start Ni Bot:" -ForegroundColor Cyan
        switch ($Platform) {
            "windows" { Write-Host "   .\start-windows.ps1" }
            "linux" { Write-Host "   ./start-linux.sh" }
            "macos" { Write-Host "   ./start-mac.sh" }
        }
        Write-Host "   ./start-docker.sh (Docker)"
    }
    
    # Auto mode: install Go dependencies
    if ($Auto) {
        Write-Log "Installing Go module dependencies..."
        go mod download
        Write-Log "Dependencies installed successfully!" "SUCCESS"
    }
}

# Run main function
Main