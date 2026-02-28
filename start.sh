#!/bin/bash

# Ni Bot 必开功能启动脚本
# 确保所有核心功能默认启用

echo "🚀 启动 Ni Bot (必开功能模式)..."

# 设置必开环境变量
export NIBOT_ENABLE_EXEC=1  # 必开：启用执行能力
export GOPROXY=https://goproxy.cn,direct  # 必开：国内镜像加速

# 自动创建工作目录
mkdir -p workspace/logs
mkdir -p workspace/memory
mkdir -p workspace/data

# 设置LLM API基础地址（避免使用OpenAI默认地址）
if [ -z "$LLM_API_BASE" ]; then
    export LLM_API_BASE=""  # 清空默认值，避免误入OpenAI
fi

echo "✅ 环境配置完成:"
echo "   - NIBOT_ENABLE_EXEC=1 (执行能力已启用)"
echo "   - GOPROXY=https://goproxy.cn (国内镜像加速)"
echo "   - workspace目录结构已创建"
echo "   - LLM_API_BASE已清空（避免误请求）"

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
