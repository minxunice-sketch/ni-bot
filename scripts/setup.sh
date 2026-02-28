#!/bin/bash

# Ni Bot 环境检测和启动脚本生成工具
# 自动检测运行环境并生成平台专用的启动脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示帮助信息
show_help() {
    echo "Ni Bot 环境设置工具"
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --detect-environment   自动检测运行环境"
    echo "  --platform <os>        指定平台 (macos|windows|linux)"
    echo "  --generate-scripts     生成启动脚本"
    echo "  --check-dependencies   检查系统依赖"
    echo "  --auto                 全自动安装和配置"
    echo "  --help                 显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 --detect-environment"
    echo "  $0 --platform macos --generate-scripts"
    echo "  $0 --auto"
}

# 检测操作系统
detect_os() {
    case "$(uname -s)" in
        Darwin)
            echo "macos"
            ;;
        Linux)
            echo "linux"
            ;;
        CYGWIN*|MINGW32*|MINGW64*|MSYS*)
            echo "windows"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# 检查依赖
check_dependencies() {
    log_info "检查系统依赖..."
    
    local missing_deps=()
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        missing_deps+=("Go")
    else
        log_success "Go 已安装: $(go version)"
    fi
    
    # 检查 Git
    if ! command -v git &> /dev/null; then
        missing_deps+=("Git")
    else
        log_success "Git 已安装: $(git --version)"
    fi
    
    # 检查 curl
    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    else
        log_success "curl 已安装"
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_warning "缺少依赖: ${missing_deps[*]}"
        return 1
    fi
    
    return 0
}

# 安装依赖 (macOS)
install_deps_macos() {
    log_info "安装 macOS 依赖..."
    
    # 检查 Homebrew
    if ! command -v brew &> /dev/null; then
        log_info "安装 Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi
    
    # 安装 Go
    if ! command -v go &> /dev/null; then
        log_info "安装 Go..."
        brew install go
    fi
    
    # 安装 Git
    if ! command -v git &> /dev/null; then
        log_info "安装 Git..."
        brew install git
    fi
    
    log_success "macOS 依赖安装完成"
}

# 生成 macOS 启动脚本
generate_macos_script() {
    cat > start-mac.sh << 'EOF'
#!/bin/bash

# Ni Bot macOS 启动脚本
# 自动生成于: $(date)

echo "🍎 启动 Ni Bot for macOS..."

# 检查是否在项目目录
if [ ! -f "go.mod" ]; then
    echo "❌ 错误：请在 Ni Bot 项目目录中运行此脚本"
    echo "💡 提示：先执行 cd /path/to/ni-bot"
    exit 1
fi

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装: brew install go"
    exit 1
fi

# 设置环境变量
export GOPROXY=https://goproxy.cn,direct
export NIBOT_ENABLE_EXEC=1
export NIBOT_AUTO_APPROVE=true

# 显示配置信息
echo "🔧 环境配置:"
echo "   - Go 版本: $(go version)"
echo "   - 项目目录: $(pwd)"
echo "   - GOPROXY: $GOPROXY"

# 安装依赖（如果需要）
if [ ! -d "vendor" ] && [ ! -f "go.sum" ]; then
    echo "📦 安装依赖..."
    go mod download
fi

# 启动 Ni Bot
echo "🚀 启动 Ni Bot..."
go run ./cmd/nibot
EOF

    chmod +x start-mac.sh
    log_success "已生成 macOS 启动脚本: start-mac.sh"
}

# 生成 Linux 启动脚本
generate_linux_script() {
    cat > start-linux.sh << 'EOF'
#!/bin/bash

# Ni Bot Linux 启动脚本
# 自动生成于: $(date)

echo "🐧 启动 Ni Bot for Linux..."

# 检查是否在项目目录
if [ ! -f "go.mod" ]; then
    echo "❌ 错误：请在 Ni Bot 项目目录中运行此脚本"
    echo "💡 提示：先执行 cd /path/to/ni-bot"
    exit 1
fi

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装:"
    echo "   Ubuntu/Debian: sudo apt install golang git"
    echo "   CentOS/RHEL: sudo yum install golang git"
    echo "   Fedora: sudo dnf install golang git"
    exit 1
fi

# 设置环境变量
export GOPROXY=https://goproxy.cn,direct
export NIBOT_ENABLE_EXEC=1
export NIBOT_AUTO_APPROVE=true

# 显示配置信息
echo "🔧 环境配置:"
echo "   - Go 版本: $(go version)"
echo "   - 项目目录: $(pwd)"
echo "   - GOPROXY: $GOPROXY"

# 安装依赖（如果需要）
if [ ! -d "vendor" ] && [ ! -f "go.sum" ]; then
    echo "📦 安装依赖..."
    go mod download
fi

# 启动 Ni Bot
echo "🚀 启动 Ni Bot..."
go run ./cmd/nibot
EOF

    chmod +x start-linux.sh
    log_success "已生成 Linux 启动脚本: start-linux.sh"
}

# 生成 Windows PowerShell 脚本
generate_windows_script() {
    cat > start-windows.ps1 << 'EOF'
# Ni Bot Windows PowerShell 启动脚本
# 自动生成于: $(Get-Date)

Write-Host "🪟 启动 Ni Bot for Windows..." -ForegroundColor Green

# 检查是否在项目目录
if (-not (Test-Path "go.mod")) {
    Write-Host "❌ 错误：请在 Ni Bot 项目目录中运行此脚本" -ForegroundColor Red
    Write-Host "💡 提示：先执行 cd C:\path\to\ni-bot" -ForegroundColor Yellow
    exit 1
}

# 检查 Go 环境
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Go 未安装，请先安装 Go for Windows" -ForegroundColor Red
    Write-Host "📥 下载: https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# 设置环境变量
$env:GOPROXY = "https://goproxy.cn,direct"
$env:NIBOT_ENABLE_EXEC = "1"
$env:NIBOT_AUTO_APPROVE = "true"

# 显示配置信息
Write-Host "🔧 环境配置:" -ForegroundColor Cyan
Write-Host "   - Go 版本: $(go version)"
Write-Host "   - 项目目录: $(Get-Location)"
Write-Host "   - GOPROXY: $env:GOPROXY"

# 安装依赖（如果需要）
if (-not (Test-Path "vendor") -and -not (Test-Path "go.sum")) {
    Write-Host "📦 安装依赖..." -ForegroundColor Cyan
    go mod download
}

# 启动 Ni Bot
Write-Host "🚀 启动 Ni Bot..." -ForegroundColor Green
go run ./cmd/nibot
EOF

    log_success "已生成 Windows 启动脚本: start-windows.ps1"
}

# 生成 Docker 启动脚本
generate_docker_script() {
    cat > start-docker.sh << 'EOF'
#!/bin/bash

# Ni Bot Docker 启动脚本
# 自动生成于: $(date)

echo "🐳 启动 Ni Bot with Docker..."

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装"
    echo "📥 安装指南: https://docs.docker.com/get-docker/"
    exit 1
fi

# 检查 docker-compose
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose 未安装"
    echo "📥 安装指南: https://docs.docker.com/compose/install/"
    exit 1
fi

# 创建必要的目录
mkdir -p workspace/data

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "📋 生成默认配置文件..."
    cat > config.yaml << 'CONFIG_EOF'
llm:
  provider: deepseek
  base_url: https://api.deepseek.com/v1
  model: deepseek-chat
  api_key: ""
  log_level: full
CONFIG_EOF
fi

# 检查 docker-compose.yml
if [ ! -f "docker-compose.yml" ]; then
    echo "📋 生成 docker-compose.yml..."
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

# 启动服务
echo "🚀 启动 Docker 容器..."
docker-compose up -d

echo "✅ Ni Bot 已启动"
echo "🌐 访问: http://localhost:8080"
echo "📋 查看日志: docker-compose logs -f"
EOF

    chmod +x start-docker.sh
    log_success "已生成 Docker 启动脚本: start-docker.sh"
}

# 主函数
main() {
    local platform=""
    local generate_scripts=false
    local check_deps=false
    local auto_mode=false
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --platform)
                platform="$2"
                shift 2
                ;;
            --generate-scripts)
                generate_scripts=true
                shift
                ;;
            --check-dependencies)
                check_deps=true
                shift
                ;;
            --detect-environment)
                platform=$(detect_os)
                shift
                ;;
            --auto)
                auto_mode=true
                platform=$(detect_os)
                generate_scripts=true
                check_deps=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 如果没有指定平台，自动检测
    if [ -z "$platform" ]; then
        platform=$(detect_os)
    fi
    
    log_info "检测到操作系统: $platform"
    
    # 检查依赖
    if [ "$check_deps" = true ]; then
        check_dependencies
    fi
    
    # 自动模式安装依赖
    if [ "$auto_mode" = true ]; then
        case $platform in
            macos)
                install_deps_macos
                ;;
            linux)
                log_info "请手动安装 Linux 依赖"
                echo "Ubuntu/Debian: sudo apt install golang git"
                echo "CentOS/RHEL: sudo yum install golang git"
                echo "Fedora: sudo dnf install golang git"
                ;;
            windows)
                log_info "请手动安装 Windows 依赖"
                echo "下载 Go: https://golang.org/dl/"
                echo "下载 Git: https://git-scm.com/download/win"
                ;;
        esac
    fi
    
    # 生成启动脚本
    if [ "$generate_scripts" = true ]; then
        log_info "生成启动脚本..."
        
        case $platform in
            macos)
                generate_macos_script
                ;;
            linux)
                generate_linux_script
                ;;
            windows)
                generate_windows_script
                ;;
            *)
                log_warning "未知平台: $platform，生成通用脚本"
                generate_linux_script
                ;;
        esac
        
        # 总是生成 Docker 脚本
        generate_docker_script
        
        log_success "启动脚本生成完成!"
        echo ""
        echo "🚀 使用以下命令启动 Ni Bot:"
        case $platform in
            macos) echo "   ./start-mac.sh" ;;
            linux) echo "   ./start-linux.sh" ;;
            windows) echo "   .\\start-windows.ps1" ;;
        esac
        echo "   ./start-docker.sh (Docker)"
    fi
    
    # 自动模式安装 Go 依赖
    if [ "$auto_mode" = true ]; then
        log_info "安装 Go 模块依赖..."
        go mod download
        log_success "依赖安装完成!"
    fi
}

# 运行主函数
main "$@"