# Ni Bot 多系统安装指南

## 🎯 概述

本文档提供 Ni Bot 在不同操作系统上的详细安装指南，包括：
- macOS 安装指南
- Windows 安装指南  
- Linux 安装指南
- Docker 容器化部署
- 智能启动脚本生成

## 🍎 macOS 安装指南

### 系统要求
- macOS 10.15+ (Catalina 或更高版本)
- Go 1.18+ 
- Git

### 自动安装脚本
```bash
# 下载并运行自动安装脚本
curl -fsSL https://raw.githubusercontent.com/minxunice-sketch/ni-bot/main/scripts/install-mac.sh | bash

# 或者手动安装
cd ~/Documents
git clone https://github.com/minxunice-sketch/ni-bot.git
cd ni-bot

# 安装 Homebrew (如果未安装)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 安装依赖
brew install go git

# 配置 Go 环境
go env -w GOPROXY=https://goproxy.cn,direct
go mod download

# 生成启动脚本
./scripts/setup.sh --generate-scripts

# 启动 Ni Bot
./start-mac.sh
```

### 手动配置步骤
1. 安装 Xcode Command Line Tools: `xcode-select --install`
2. 安装 Homebrew: `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`
3. 安装 Go: `brew install go`
4. 安装 Git: `brew install git`
5. 克隆项目: `git clone https://github.com/minxunice-sketch/ni-bot.git`
6. 进入目录: `cd ni-bot`
7. 安装依赖: `go mod download`
8. 启动: `go run ./cmd/nibot`

## 🪟 Windows 安装指南

### 系统要求
- Windows 10/11
- Go 1.18+
- Git for Windows

### 自动安装 (PowerShell)
```powershell
# 以管理员身份运行 PowerShell
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser

# 下载并运行安装脚本
irm https://raw.githubusercontent.com/minxunice-sketch/ni-bot/main/scripts/install-windows.ps1 | iex

# 或者手动安装
cd $HOME\Documents
git clone https://github.com/minxunice-sketch/ni-bot.git
cd ni-bot

# 安装 Chocolatey (包管理器)
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# 安装依赖
choco install golang git -y

# 配置环境
$env:GOPROXY = "https://goproxy.cn,direct"
go mod download

# 生成启动脚本
.\scripts\setup.ps1 -GenerateScripts

# 启动 Ni Bot
.\start-windows.ps1
```

### 手动安装步骤
1. 安装 [Go for Windows](https://golang.org/dl/)
2. 安装 [Git for Windows](https://git-scm.com/download/win)
3. 添加 Go 到 PATH: `setx PATH "%PATH%;C:\Go\bin"`
4. 克隆项目: `git clone https://github.com/minxunice-sketch/ni-bot.git`
5. 进入目录: `cd ni-bot`
6. 设置代理: `set GOPROXY=https://goproxy.cn,direct`
7. 安装依赖: `go mod download`
8. 启动: `go run ./cmd/nibot`

## 🐧 Linux 安装指南

### 支持的发⾏版
- Ubuntu 18.04+
- Debian 10+
- CentOS 8+
- Fedora 30+

### Ubuntu/Debian
```bash
# 自动安装脚本
curl -fsSL https://raw.githubusercontent.com/minxunice-sketch/ni-bot/main/scripts/install-ubuntu.sh | bash

# 手动安装
sudo apt update
sudo apt install -y golang git

cd ~
git clone https://github.com/minxunice-sketch/ni-bot.git
cd ni-bot

go env -w GOPROXY=https://goproxy.cn,direct
go mod download

# 生成启动脚本
./scripts/setup.sh --generate-scripts

# 启动
./start-linux.sh
```

### CentOS/RHEL/Fedora
```bash
# CentOS/RHEL
sudo yum install -y golang git

# Fedora  
sudo dnf install -y golang git

cd ~
git clone https://github.com/minxunice-sketch/ni-bot.git
cd ni-bot

go env -w GOPROXY=https://goproxy.cn,direct
go mod download

./scripts/setup.sh --generate-scripts
./start-linux.sh
```

## 🐳 Docker 部署

### 使用 Docker Compose (推荐)
```yaml
# docker-compose.yml
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
```

### 运行命令
```bash
# 创建配置目录
mkdir -p workspace/data

# 下载 docker-compose.yml
curl -o docker-compose.yml https://raw.githubusercontent.com/minxunice-sketch/ni-bot/main/docker-compose.yml

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 直接使用 Docker
```bash
# 拉取镜像
docker pull minxunice/ni-bot:latest

# 运行容器
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/workspace:/app/workspace \
  -v $(pwd)/config.yaml:/app/workspace/data/config.yaml \
  --name ni-bot \
  minxunice/ni-bot:latest
```

## 🤖 智能启动脚本生成

Ni Bot 会自动检测运行环境并生成合适的启动脚本：

### 生成启动脚本
```bash
# 自动检测环境并生成脚本
./scripts/setup.sh --detect-environment

# 或手动指定平台
./scripts/setup.sh --platform macos
./scripts/setup.sh --platform windows  
./scripts/setup.sh --platform linux
```

### 生成的脚本文件
- `start-mac.sh` - macOS 启动脚本
- `start-windows.ps1` - Windows PowerShell 脚本  
- `start-linux.sh` - Linux 启动脚本
- `start-docker.sh` - Docker 启动脚本

### 环境检测功能
脚本会自动检测：
- 操作系统类型 (macOS/Windows/Linux)
- Go 版本和安装状态
- Git 可用性
- 网络代理设置
- 必要的依赖包

## 🔧 高级配置

### 自定义 API 配置
在 `workspace/data/config.yaml` 中配置：

```yaml
llm:
  provider: deepseek
  base_url: https://api.deepseek.com/v1
  model: deepseek-chat
  api_key: "your_api_key_here"
  log_level: full
```

### 环境变量配置
```bash
# API 配置
export LLM_PROVIDER=deepseek
export LLM_BASE_URL=https://api.deepseek.com/v1
export LLM_MODEL_NAME=deepseek-chat
export LLM_API_KEY="your_api_key_here"

# 可选配置
export NIBOT_ENABLE_NATIVE_TOOLS="1"
export NIBOT_STORAGE="sqlite"
```

## 🐦 飞书机器人配置指南

### 飞书应用创建步骤

#### 1. 访问飞书开放平台
- 打开 [飞书开放平台](https://open.feishu.cn/)
- 使用企业账号登录（个人账号无法创建企业自建应用）

#### 2. 创建企业自建应用
1. 点击"创建企业自建应用"
2. 填写应用名称（如：Ni Bot 助手）
3. 选择应用描述（如：AI 助手机器人）
4. 点击"创建"

#### 3. 获取应用凭证
1. 在应用详情页的"凭证与基础信息"中获取：
   - **App ID**
   - **App Secret**
2. 复制这些值用于环境变量配置

#### 4. 配置权限
在"权限管理"中添加以下权限：
- **im:message** - 获取与发送单聊、群组消息
- **im:message.group_at_msg** - 接收群聊中@机器人的消息
- **im:message.p2p_msg** - 接收单聊消息
- **im:message:send** - 发送消息
- **contact:user.id:get** - 获取用户 user_id

##### 批量导入权限配置（JSON格式）
```json
{
  "permissions": [
    {
      "permission_name": "im:message",
      "permission_description": "获取与发送单聊、群组消息"
    },
    {
      "permission_name": "im:message.group_at_msg", 
      "permission_description": "接收群聊中@机器人的消息"
    },
    {
      "permission_name": "im:message.p2p_msg",
      "permission_description": "接收单聊消息"
    },
    {
      "permission_name": "im:message:send",
      "permission_description": "发送消息"
    },
    {
      "permission_name": "contact:user.id:get",
      "permission_description": "获取用户 user_id"
    }
  ]
}
```

##### 批量导入API调用（开发者工具）
```javascript
// 在浏览器开发者工具中执行（飞书开放平台页面）
const permissions = [
  'im:message',
  'im:message.group_at_msg', 
  'im:message.p2p_msg',
  'im:message:send',
  'contact:user.id:get'
];

permissions.forEach(permission => {
  // 模拟点击添加权限按钮
  const addButton = document.querySelector(`button[data-permission="${permission}"]`);
  if (addButton) {
    addButton.click();
    console.log(`已添加权限: ${permission}`);
  } else {
    console.warn(`未找到权限: ${permission}`);
  }
});
```

#### 5. 配置事件订阅
在"事件订阅"中配置：
1. **请求网址 URL**: `https://your-domain.com/feishu/webhook`
   - 本地开发可使用 ngrok: `ngrok http 8081`
   - 生产环境配置真实域名
2. **添加事件**: 
   - `im.message.receive_v1` - 接收消息

#### 6. 版本管理与发布
1. 在"版本管理与发布"中创建版本
2. 填写版本号和应用可用范围
3. 申请发布，等待管理员审核

### 环境变量配置

```bash
# 必需配置
export FEISHU_APP_ID="cli_xxxxxxxxxxxxxxx"
export FEISHU_APP_SECRET="xxxxxxxxxxxxxxxxxxxxxxxx"

# 可选配置
export FEISHU_VERIFICATION_TOKEN=""  # 事件订阅验证Token
export FEISHU_ENCRYPT_KEY=""         # 消息加密Key
export FEISHU_HTTP_PORT="8081"        # HTTP服务器端口
export FEISHU_TIMEOUT="30"           # 请求超时(秒)
export FEISHU_MAX_CONCURRENT="10"    # 最大并发数
export FEISHU_DEBUG="false"          # 调试模式

# 自动启用飞书模式
export NIBOT_ENABLE_FEISHU="true"
```

### 本地开发配置

#### 使用 ngrok 进行本地测试
```bash
# 安装 ngrok
npm install -g ngrok

# 启动隧道 (需要 ngrok 账号)
ngrok http 8081

# 复制生成的 https URL 到飞书事件订阅配置
```

#### 直接运行
```bash
# 设置环境变量
export FEISHU_APP_ID="your_app_id"
export FEISHU_APP_SECRET="your_app_secret"

# 启动飞书机器人
nibot -feishu

# 或使用环境变量自动启用
nibot
```

### 生产环境部署

#### Docker 部署
```yaml
# docker-compose.yml 添加环境变量
environment:
  - FEISHU_APP_ID=cli_xxxxxxxxxxxxxxx
  - FEISHU_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxx
  - FEISHU_HTTP_PORT=8081
  - FEISHU_TIMEOUT=30
```

#### 云服务器配置
1. 配置域名和 SSL 证书
2. 设置反向代理到 Ni Bot 服务
3. 配置飞书事件订阅 URL
4. 设置防火墙规则开放 8081 端口

### 故障排除

#### 常见问题
1. **收不到消息**: 检查事件订阅配置和权限设置
2. **401 错误**: 验证 App ID 和 App Secret 是否正确
3. **消息发送失败**: 检查权限是否包含 `im:message:send`
4. **webhook 验证失败**: 确认请求网址 URL 可访问

#### 调试模式
```bash
export FEISHU_DEBUG="true"
nibot -feishu
```

### 安全建议
1. 保护 App Secret，不要泄露
2. 使用加密密钥保护消息传输
3. 配置 IP 白名单（如果支持）
4. 定期轮换应用凭证

## 🚀 快速开始

### 第一次运行
```bash
# 克隆项目
git clone https://github.com/minxunice-sketch/ni-bot.git
cd ni-bot

# 自动安装和配置
./scripts/setup.sh --auto

# 启动 Ni Bot
go run ./cmd/nibot
```

### 访问 Web 界面
启动成功后，打开浏览器访问：
- http://localhost:8080

## 📊 系统要求检查

### 最低要求
- Go 1.18+
- 1GB RAM
- 100MB 磁盘空间
- 网络连接

### 推荐配置  
- Go 1.20+
- 2GB+ RAM
- 1GB 磁盘空间
- 稳定的网络连接

## 🔍 故障排除

### 常见问题

#### Q: Go 命令未找到
A: 安装 Go 并确保在 PATH 中
- macOS: `brew install go`
- Windows: 下载并安装 Go MSI 包
- Linux: `sudo apt install golang`

#### Q: 网络连接超时
A: 配置代理
```bash
export GOPROXY=https://goproxy.cn,direct
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

#### Q: 端口 8080 被占用
A: 使用其他端口
```bash
export NIBOT_HTTP_PORT=8081
go run ./cmd/nibot
```

#### Q: 权限不足
A: 确保对 workspace 目录有写权限
```bash
chmod -R 755 workspace
```

### 获取帮助
- 查看详细文档: [README.md](./README.md)
- 提交 Issue: [GitHub Issues](https://github.com/minxunice-sketch/ni-bot/issues)
- 讨论区: [GitHub Discussions](https://github.com/minxunice-sketch/ni-bot/discussions)

## 📝 更新日志

### v1.1.0 - 2026-02-28
- 新增多系统安装指南
- 添加智能环境检测功能
- 自动生成平台专用启动脚本
- 改进错误处理和用户提示

### v1.0.0 - 2026-02-27  
- 初始版本发布
- 基础对话功能
- 工具调用系统
- Web 界面支持

---

📧 如有问题，请提交 GitHub Issue 或联系维护团队。