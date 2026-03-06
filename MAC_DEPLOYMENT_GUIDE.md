# Ni bot macOS 部署与配置指南

本指南专门面向 macOS，覆盖从搭建、配置、开关选择、技能安装到排错与日常使用。

## 1. 安装与启动

### 1.1 前置条件

- macOS
- Go 1.24+
- Git

```bash
go version
git --version
```

### 1.2 拉取项目并运行

```bash
cd /Users/mac
git clone https://github.com/minxunice-sketch/ni-bot.git
cd /Users/mac/ni-bot
go run ./cmd/nibot -workspace /Users/mac/ni-bot/workspace
```

### 1.3 启动 Web 界面

```bash
# 启动 Web 服务
go run ./cmd/web
```
启动后访问：[http://localhost:8080](http://localhost:8080)

## 2. 配置应在终端还是对话里设置

- 环境变量开关必须在终端设置（`export`）
- 模型基础配置可写 `workspace/data/config.yaml`
- 与 AI 对话用于执行任务，不用于修改系统环境变量

## 3. 模型基础配置

### 3.1 方案 A：NVIDIA + Qwen (Agent 能力强)
```bash
export LLM_PROVIDER="nvidia"
export LLM_BASE_URL="https://integrate.api.nvidia.com/v1"
export LLM_MODEL_NAME="qwen/qwen3-next-80b-a3b-instruct"
export LLM_API_KEY="<YOUR_NVIDIA_API_KEY>"
export NIBOT_LOG_LEVEL="full"
```

### 3.2 方案 B：Kimi (Moonshot)
```bash
export LLM_PROVIDER="moonshot"
export LLM_BASE_URL="https://api.moonshot.cn/v1"
export LLM_MODEL_NAME="moonshot-v1-8k"
export LLM_API_KEY="<YOUR_MOONSHOT_API_KEY>"
export NIBOT_LOG_LEVEL="full"
```
（注：Ni bot 默认将 moonshot 识别为 openai 兼容协议，provider 写 moonshot 或 openai 均可）

### 3.3 方案 C：DeepSeek
```bash
export LLM_PROVIDER="deepseek"
export LLM_BASE_URL="https://api.deepseek.com/v1"
export LLM_MODEL_NAME="deepseek-chat"
export LLM_API_KEY="<YOUR_DEEPSEEK_API_KEY>"
export NIBOT_LOG_LEVEL="full"
```
（注：DeepSeek V3 使用 `deepseek-chat`；R1 推理模型使用 `deepseek-reasoner`）


## 4. 功能开关总表（按需启用）

### 4.1 执行与技能

- `NIBOT_ENABLE_EXEC=1`：允许 `runtime.exec`
- `NIBOT_ENABLE_SKILLS=1`：允许 `skill.exec`
- `NIBOT_ENABLE_GIT=1`：允许 `skills install git <https-url>`
- `NIBOT_ENABLE_NATIVE_TOOLS=1`：启用原生 tool calling 转译

```bash
export NIBOT_ENABLE_EXEC="1"
export NIBOT_ENABLE_SKILLS="1"
export NIBOT_ENABLE_GIT="1"
export NIBOT_ENABLE_NATIVE_TOOLS="1"
```

### 4.2 存储与记忆

- `NIBOT_STORAGE=sqlite`：启用 SQLite 会话与审计
- `NIBOT_MEMORY_DB=sqlite`：启用记忆库（与上者二选一也可）
- `NIBOT_AUTO_RECALL`：自动召回（默认开，设为 `0` 可关）
- `NIBOT_AUTO_MEMORY`：自动提取记忆（默认关）

```bash
export NIBOT_STORAGE="sqlite"
export NIBOT_AUTO_RECALL="1"
export NIBOT_AUTO_MEMORY="1"
```

### 4.3 沙箱与资源限制

- `NIBOT_EXEC_SANDBOX=1`：启用沙箱执行（需系统可用 `trae-sandbox`）
- `NIBOT_SANDBOX_BIN`：自定义沙箱可执行路径
- `NIBOT_EXEC_MAX_OUTPUT_BYTES`：单次执行输出上限
- `NIBOT_EXEC_MAX_CONCURRENT`：并发执行上限

```bash
export NIBOT_EXEC_SANDBOX="0"
export NIBOT_EXEC_MAX_OUTPUT_BYTES="262144"
export NIBOT_EXEC_MAX_CONCURRENT="2"
```

### 4.4 健康监控

- `NIBOT_HEALTH_PORT=8082`：开启健康端点

```bash
export NIBOT_HEALTH_PORT="8082"
```

### 4.5 技能安装层

- `NIBOT_SKILLS_INSTALL_LAYER`：技能安装层（`local` / `upstream` / `overrides`）

```bash
export NIBOT_SKILLS_INSTALL_LAYER="upstream"
```

## 5. 推荐启动配置（可直接复制）

### 5.1 安装技能时推荐配置

```bash
cd /Users/mac/ni-bot
export LLM_PROVIDER="nvidia"
export LLM_BASE_URL="https://integrate.api.nvidia.com/v1"
export LLM_MODEL_NAME="qwen/qwen3-next-80b-a3b-instruct"
export LLM_API_KEY="<YOUR_NVIDIA_API_KEY>"
export NIBOT_LOG_LEVEL="full"

export NIBOT_ENABLE_EXEC="1"
export NIBOT_ENABLE_SKILLS="1"
export NIBOT_ENABLE_GIT="1"
export NIBOT_ENABLE_NATIVE_TOOLS="1"

export NIBOT_STORAGE="sqlite"
export NIBOT_HEALTH_PORT="8082"
export NIBOT_EXEC_SANDBOX="0"

go run ./cmd/nibot -workspace /Users/mac/ni-bot/workspace
```

说明：

- 本套配置用于安装或更新 GitHub skills
- `NIBOT_ENABLE_GIT="1"` 仅在安装阶段需要

### 5.2 日常运行技能推荐配置

```bash
cd /Users/mac/ni-bot
export LLM_PROVIDER="nvidia"
export LLM_BASE_URL="https://integrate.api.nvidia.com/v1"
export LLM_MODEL_NAME="qwen/qwen3-next-80b-a3b-instruct"
export LLM_API_KEY="<YOUR_NVIDIA_API_KEY>"
export NIBOT_LOG_LEVEL="full"

export NIBOT_ENABLE_SKILLS="1"
export NIBOT_ENABLE_NATIVE_TOOLS="1"
export NIBOT_ENABLE_GIT="0"

export NIBOT_STORAGE="sqlite"
export NIBOT_HEALTH_PORT="8082"
export NIBOT_EXEC_SANDBOX="0"

go run ./cmd/nibot -workspace /Users/mac/ni-bot/workspace
```

说明：

- 本套配置用于已安装 skills 的日常使用
- 保持 `NIBOT_ENABLE_SKILLS="1"` 以便执行 skill
- 关闭 `NIBOT_ENABLE_GIT` 可减少误安装和安全暴露面

## 6. 验证是否生效

### 6.1 查看开关值

```bash
echo "$NIBOT_ENABLE_EXEC"
echo "$NIBOT_ENABLE_SKILLS"
echo "$NIBOT_ENABLE_GIT"
echo "$NIBOT_ENABLE_NATIVE_TOOLS"
```

### 6.2 健康检查

```bash
curl http://localhost:8082/health
curl http://localhost:8082/metrics
curl http://localhost:8082/stats
```

### 6.3 SQLite 文件

```bash
ls -lh /Users/mac/ni-bot/workspace/data/nibot.db
```

## 7. GitHub skills 安装

在 Ni bot 对话中输入：

```text
skills install git https://github.com/<owner>/<repo>.git
reload
skills
```

注意：

- 需先开启 `NIBOT_ENABLE_GIT=1`
- 仅支持 `https://` 仓库链接
- 链接不要带反引号、空格或 markdown 包裹

## 8. 常见问题

### 8.1 `sandbox enabled but trae-sandbox not found in PATH`

原因：开启了沙箱，但系统没有 `trae-sandbox`。

处理：

- 临时排查：`export NIBOT_EXEC_SANDBOX="0"`
- 或安装沙箱并配置 `NIBOT_SANDBOX_BIN`

### 8.2 安装 skill 时出现 `Username for 'https://github.com'`

常见原因：

- URL 格式不规范
- 仓库不是公开仓库
- 链接带了多余字符导致 clone 异常

建议使用纯净链接：

```text
https://github.com/<owner>/<repo>.git
```

### 8.3 `metrics` 显示 `total_messages=0`

通常表示当前实例还未记录消息，或你查询了另一个实例。

处理：

- 在当前运行实例里先发送几条对话
- 再查询 `metrics`

## 9. 长期生效（可选）

将常用 `export` 写入 `~/.zshrc`：

```bash
echo 'export NIBOT_ENABLE_GIT="1"' >> ~/.zshrc
echo 'export NIBOT_ENABLE_SKILLS="1"' >> ~/.zshrc
echo 'export NIBOT_ENABLE_EXEC="1"' >> ~/.zshrc
source ~/.zshrc
```
