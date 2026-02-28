#!/bin/bash

# Ni Bot Docker Startup Script
# Auto-generated on: 02/28/2026 17:47:57

echo "🐳 Starting Ni Bot with Docker..."

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "�?Docker not installed"
    echo "📥 Installation guide: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check docker-compose
if ! command -v docker-compose &> /dev/null; then
    echo "�?docker-compose not installed"
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

echo "�?Ni Bot started successfully"
echo "🌐 Access: http://localhost:8080"
echo "📋 View logs: docker-compose logs -f"
