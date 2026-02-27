# Ni bot 快速启动指南

## 🚀 一分钟快速开始

### 1. 环境准备
确保已安装 Go 1.21+：
```bash
go version
# 应该显示: go version go1.21+ ...
```

### 2. 下载和运行
```bash
# 克隆项目（如果尚未克隆）
git clone <repository-url>
cd ai-agent

# 直接运行（Mock模式，无需API密钥）
go run .\cmd\nibot
```

### 3. 首次对话
Ni bot启动后，您可以开始对话：
```
你好，请帮我查看当前目录的文件列表
```

## 📋 完整配置指南

### 基础配置（必需）

#### 方法1：使用环境变量
```powershell
# 设置OpenAI兼容API（推荐NVIDIA NIM）
$env:LLM_PROVIDER="nvidia"
$env:LLM_BASE_URL="https://integrate.api.nvidia.com/v1"
$env:LLM_MODEL_NAME="moonshotai/kimi-k2.5"
$env:LLM_API_KEY="nvapi-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# 或者使用OpenAI
$env:LLM_PROVIDER="openai"
$env:LLM_BASE_URL="https://api.openai.com/v1"
$env:LLM_MODEL_NAME="gpt-4-turbo"
$env:LLM_API_KEY="sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# 运行Ni bot
go run .\cmd\nibot
```

#### 方法2：使用.env文件
在 `workspace/.env` 中配置：
```bash
LLM_PROVIDER=nvidia
LLM_BASE_URL=https://integrate.api.nvidia.com/v1
LLM_MODEL_NAME=moonshotai/kimi-k2.5
LLM_API_KEY=nvapi-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### 安全配置（推荐）
```powershell
# 启用严格安全模式
$env:NIBOT_SECURITY_MODE="strict"

# API密钥脱敏保护
$env:NIBOT_REDACT_KEYS="true"

# 启用沙箱执行
$env:NIBOT_EXEC_SANDBOX="1"

# 配置审计日志
$env:NIBOT_AUDIT_LOG="workspace/logs/audit.log"
```

### 功能启用配置
```powershell
# 允许执行命令（谨慎使用）
$env:NIBOT_ENABLE_EXEC="1"

# 允许执行技能脚本
$env:NIBOT_ENABLE_SKILLS="1"

# 允许Git操作
$env:NIBOT_ENABLE_GIT="1"

# 启用技能进化功能（实验性）
$env:NIBOT_EVOLUTION_ENABLED="true"
$env:NIBOT_EVOLUTION_STRATEGY="balanced"
```

## 🔧 工具使用说明

### 可用工具命令
```
# 读取文件
[EXEC:fs.read {"path":"memory/facts.md"}]

# 写入文件
[EXEC:fs.write {"path":"memory/notes.md","content":"笔记内容","mode":"append"}]

# 执行命令（需要启用）
[EXEC:runtime.exec {"command":"dir","timeoutSeconds":30}]

# 执行技能脚本
[EXEC:skill.exec {"skill":"weather","script":"weather.ps1","args":["Beijing"],"timeoutSeconds":30}]

# 健康监控
[EXEC:health.status {}]
[EXEC:health.metrics {}]
[EXEC:health.stats {}]
```

### 健康监控端点
Ni bot提供以下监控端点：
- **健康检查**: http://localhost:8080/health
- **性能指标**: http://localhost:8080/metrics (JSON格式)
- **统计信息**: http://localhost:8080/stats (可读格式)

## 🛠️ 高级功能配置

### 会话持久化
Ni bot自动保存和恢复会话状态，无需额外配置。

### 技能管理
```bash
# 查看可用技能
ls workspace/skills/

# 安装新技能（将技能文件夹放入）
cp -r new-skill/ workspace/skills/
```

### 策略配置
在 `workspace/data/policy.toml` 中配置细粒度权限：
```toml
allow_fs_write = true
allow_runtime_exec = true
allow_skill_exec = true

require_approval_fs_write = true
require_approval_runtime_exec = true
require_approval_skill_exec = true

allowed_runtime_prefixes = "go,git"
allowed_write_prefixes = "memory/,skills/,logs/"
allowed_skill_names = "*"
allowed_skill_scripts = "*"
```

## 🚨 故障排除

### 常见问题

#### Q: 找不到go命令？
A: 临时添加Go到PATH：
```powershell
$env:Path += ";C:\Program Files\Go\bin"
```

#### Q: API密钥无效？
A: 检查密钥格式和环境变量名称

#### Q: 工具调用被拒绝？
A: 需要手动审批或修改策略配置

#### Q: 健康端点无法访问？
A: 确保端口8080未被占用

### 日志检查
```bash
# 查看会话日志
ls workspace/logs/

# 查看审计日志（如果配置）
cat workspace/logs/audit.log
```

## 📊 监控和优化

### 监控指标
Ni bot提供以下关键指标：
- 运行时间、活跃会话数
- 消息处理速率、工具调用速率
- 审批通过率、拒绝率
- 错误率和性能指标

### 性能优化
```powershell
# 调整并发限制
$env:NIBOT_EXEC_MAX_CONCURRENT="4"

# 调整输出限制
$env:NIBOT_EXEC_MAX_OUTPUT_BYTES="524288"

# 启用性能监控
$env:NIBOT_PERF_MONITORING="true"
```

## 🔄 更新和维护

### 更新Ni bot
```bash
# 拉取最新代码
git pull

# 重新构建
go build .\cmd\nibot

# 或直接运行
go run .\cmd\nibot
```

### 数据备份
```bash
# 备份重要数据
cp -r workspace/data/ backup/
cp -r workspace/memory/ backup/
cp -r workspace/logs/ backup/
```

## 📞 获取帮助

1. 查看 `README.md` 获取详细文档
2. 检查 `SECURITY_NOTES.md` 了解安全注意事项
3. 查看会话日志了解运行详情
4. 使用健康端点监控系统状态

---
*祝您使用愉快！如有问题请查看文档或日志。*