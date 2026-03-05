#!/bin/bash

# Ni Bot 启动脚本（安全默认）

echo "🚀 启动 Ni Bot..."

# 可选：国内依赖镜像加速（如不需要可自行覆盖）
export GOPROXY=https://goproxy.cn,direct

# 自动创建工作目录
mkdir -p workspace/logs
mkdir -p workspace/memory
mkdir -p workspace/data

echo "✅ 环境配置完成:"
echo "   - GOPROXY=https://goproxy.cn (国内镜像加速)"
echo "   - workspace目录结构已创建"

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "❌ Go未安装，请从 https://go.dev/dl/ 安装"
    exit 1
fi

# 显示Go版本
echo "🔧 Go版本: $(go version)"

# 运行Ni Bot
echo "🎯 启动Ni Bot..."
go run ./cmd/nibot
