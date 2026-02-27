# Ni bot

Ni bot 是一个极简、文件驱动、可自进化的 AI Agent 原型：身份/记忆/技能都落在 `workspace/`，运行时自动扫描并注入提示词；模型通过输出 `[EXEC:...]` 触发工具，工具执行结果回灌给模型继续推理。

## 运行

### 前置条件

- 已安装 Go（建议 1.21+），并确保在当前终端 `go version` 可用
- 可选：已安装 Git（用于从 GitHub 克隆仓库）

### 安装 Go（Windows / macOS / Linux）

Windows：

- 方式 1：官方下载并安装（推荐）：访问 Go 官网下载页面安装（go.dev/dl）
- 方式 2：如果已安装但终端找不到 `go`，可将 Go 加入 PATH（示例）：

```powershell
$env:Path += ";C:\Program Files\Go\bin"
go version
```

macOS：

- Apple Silicon（M1/M2/M3）与 Intel 均可使用 Homebrew：

```bash
brew install go
go version
```

- 如果不使用 Homebrew：从 go.dev/dl 下载 macOS 安装包（pkg）安装后，重新打开终端再执行 `go version`

Linux：

- Debian/Ubuntu：

```bash
sudo apt update
sudo apt install -y golang
go version
```

- Fedora：

```bash
sudo dnf install -y golang
go version
```

- Arch：

```bash
sudo pacman -S --noconfirm go
go version
```

如果发行版仓库的 Go 版本偏旧，建议改用 go.dev/dl 的官方包安装。

### 安装 Git（可选）

- Windows：安装 Git for Windows（安装后在 PowerShell 里执行 `git --version`）
- macOS：执行 `xcode-select --install` 或使用 Homebrew：`brew install git`
- Linux：`sudo apt install -y git` / `sudo dnf install -y git` / `sudo pacman -S git`

### 下载代码（GitHub）

```bash
git clone https://github.com/minxunice-sketch/ni-bot.git
cd ni-bot
```

### 运行（Windows / macOS / Linux）

Windows（PowerShell）：

```powershell
go run .\cmd\nibot
```

macOS / Linux（bash/zsh）：

```bash
go run ./cmd/nibot
```

## 配置（环境变量）

Ni bot 支持 OpenAI 兼容接口（含 NVIDIA NIM），以及 Ollama。

注意：不要在终端/截图/日志里粘贴完整 API Key。Ni bot 会对常见 key/token 做脱敏，但仍建议只展示前后各 4 位用于排障。

### NVIDIA NIM（moonshotai/kimi-k2.5）

说明：

- `LLM_BASE_URL` 不要包含反引号或空格
- 直接在浏览器打开 `https://integrate.api.nvidia.com/v1` 显示 404 属于正常（API 请求会带具体路径）
- `LLM_API_KEY` 也可用等价变量 `NVIDIA_API_KEY`

Windows（PowerShell）：

```powershell
$env:LLM_PROVIDER="nvidia"
$env:LLM_BASE_URL="https://integrate.api.nvidia.com/v1"
$env:LLM_MODEL_NAME="moonshotai/kimi-k2.5"
$env:LLM_API_KEY="<YOUR_NVIDIA_API_KEY>"
go run .\cmd\nibot
```

macOS / Linux（bash/zsh）：

```bash
export LLM_PROVIDER="nvidia"
export LLM_BASE_URL="https://integrate.api.nvidia.com/v1"
export LLM_MODEL_NAME="moonshotai/kimi-k2.5"
export LLM_API_KEY="<YOUR_NVIDIA_API_KEY>"
go run ./cmd/nibot
```

### Mock 模式（无 Key）

不设置 `LLM_API_KEY` 即进入 Mock 模式，可用于验证工具调用闭环（fs.read/fs.write 等）。

## Workspace 结构

- `workspace/AGENT.md`：身份（人格与工作循环）
- `workspace/memory/`：长期记忆与反思
- `workspace/skills/{name}/SKILL.md`：技能描述（注入提示词）
- `workspace/skills/{name}/scripts/`：技能脚本（可被执行为工具）
- `workspace/logs/`：会话审计日志（自动生成）

## 工具

模型通过在回复中输出以下标签来调用工具：

- 读取文件：
  - `[EXEC:fs.read {"path":"memory/facts.md"}]`
- 写入文件（默认 append，overwrite 受限）：
  - `[EXEC:fs.write {"path":"memory/notes.md","content":"...","mode":"append"}]`
- 执行命令（默认禁用，需要显式开启）：
  - `[EXEC:runtime.exec {"command":"dir","timeoutSeconds":30}]`
- 执行技能脚本（默认禁用，需要显式开启）：
  - `[EXEC:skill.exec {"skill":"weather","script":"weather.ps1","args":["Beijing"],"timeoutSeconds":30}]`
- 健康监控（内置功能）：
  - `[EXEC:health.status {}]` - 查看健康状态
  - `[EXEC:health.metrics {}]` - 查看性能指标
  - `[EXEC:health.stats {}]` - 查看统计信息

### 安全开关

- `runtime.exec`：默认禁用，需设置 `NIBOT_ENABLE_EXEC=1` 才允许执行
- `skill.exec`：默认禁用，需设置 `NIBOT_ENABLE_SKILLS=1` 才允许执行
- `skills install git`：默认禁用，需设置 `NIBOT_ENABLE_GIT=1` 才允许执行（仅允许 https:// URL）
- 所有写入与执行都会在 CLI 中要求 y/n 审批

### 执行隔离（sandbox）

- `NIBOT_EXEC_SANDBOX=1`：让 `runtime.exec` 与 `skill.exec` 通过 sandbox 运行（默认 off）
- `NIBOT_SANDBOX_BIN`：sandbox 可执行文件名或绝对路径（默认 `trae-sandbox` / `trae-sandbox.exe`）
- sandbox 开启但找不到可执行文件时，将直接报错，不会回退到宿主机执行

### 执行资源限制

- `NIBOT_EXEC_MAX_OUTPUT_BYTES`（默认 262144）：单次执行 stdout/stderr 的最大捕获字节数，超出会截断并追加 `[TRUNCATED]`
- `NIBOT_EXEC_MAX_CONCURRENT`（默认 2）：并发执行上限（超出会排队等待）

## 生产级特性

### 会话持久化

Ni bot 支持完整的会话持久化功能：
- 自动保存和恢复对话状态
- 会话数据原子写入防止损坏
- 支持会话级别的工具调用审批跟踪

### 健康监控

内置健康监控系统提供生产级可观测性：
- **健康检查端点**: `http://localhost:8080/health`
- **性能指标端点**: `http://localhost:8080/metrics` (JSON格式)
- **统计信息端点**: `http://localhost:8080/stats` (可读格式)

监控指标包括：
- 运行时间、活跃会话数、总消息数
- 工具调用率、消息率、审批率
- 总批准数、总拒绝数、审批成功率

### 安全增强

针对OpenClaw类漏洞的防护措施：
- **API密钥保护**: 自动检测和脱敏敏感信息
- **输入验证**: 防止提示注入和命令注入攻击
- **会话隔离**: 严格的会话边界控制防止数据泄露
- **安全审计**: 完整的操作审计日志和异常检测

### 技能进化能力（实验性）

基于GEP协议的自我进化功能：
- **自动错误分析**: 实时检测运行时错误和性能瓶颈
- **自我修复**: 自动修改代码直到测试通过
- **能力继承**: 跨会话共享验证过的解决方案
- **安全进化**: 受控的变异和严格的修改限制

启用进化功能：
```powershell
$env:NIBOT_EVOLUTION_ENABLED="true"
$env:NIBOT_EVOLUTION_STRATEGY="balanced"  # balanced|innovate|harden|repair-only
```

## 安全最佳实践

1. **密钥管理**: 永远不要在日志或终端输出完整API密钥
2. **环境隔离**: 为不同用途创建独立的会话上下文
3. **权限控制**: 限制第三方技能的权限范围
4. **审计监控**: 定期审查审计日志中的异常活动
5. **及时更新**: 保持Ni bot版本最新以获取安全修复

### 工具策略（policy.toml）

可在 `workspace/data/policy.toml` 配置更细粒度的允许/审批策略（可选）：

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

也可以从 `workspace/data/policy.toml.example` 复制一份开始改。

字段说明：

- `allowed_write_prefixes`：进一步限制 `fs.write` 的相对路径前缀（仍然只允许 memory/skills/logs 三类目录）；支持 `*`
- `allowed_skill_names`：允许执行的 skill 名称列表；支持 `*`
- `allowed_skill_scripts`：允许执行的脚本名（`script` 或 `skill/script`）；支持 `*`

### 日志级别

- `NIBOT_LOG_LEVEL=full`（默认）：session log 记录完整（已脱敏的）Tool Results 与 System Prompt
- `NIBOT_LOG_LEVEL=meta`：session log 仅记录元数据（长度/预览等），更适合分享与排障截图
- session log 额外包含 Audit 段：记录审批（允许/拒绝）与每次工具调用的摘要（不包含完整输出）

### Skills 导入限制

- `NIBOT_SKILLS_MAX_FILE_BYTES`（默认 5242880）：导入 skills 时单个文件允许的最大字节数，避免误导入超大依赖目录
- `NIBOT_SKILLS_MAX_TOTAL_BYTES`（默认 20971520）：导入 zip 时允许解压的总字节数上限
- `NIBOT_SKILLS_MAX_ZIP_BYTES`（默认 52428800）：允许导入的 zip 文件本身大小上限

### 自动 Reload

- `NIBOT_AUTO_RELOAD=1`：开启轮询检测 workspace 变更，自动重载 system prompt 与 policy
- `NIBOT_AUTO_RELOAD_INTERVAL_MS`（默认 1000）：轮询间隔（200–10000）

## 创建一个技能（示例）

创建目录：

```
workspace/skills/weather/
  SKILL.md
  scripts/
    weather.ps1
```

示例 `SKILL.md`：

```markdown
---
name: Weather
description: 查询城市天气（示例技能）
---

使用 skill.exec 调用 scripts/weather.ps1，参数为城市名。
```

示例 `scripts/weather.ps1`（PowerShell）：

```powershell
param(
  [string]$City = "Beijing"
)
curl "https://wttr.in/$City?format=3"
```

运行时开启技能脚本执行：

```powershell
.\run.ps1 -EnableSkills
```

## Skill 兼容性

Ni bot 在发现/展示技能时，会自动读取以下任一元数据文件（无需手动改代码）：

- `SKILL.md` 或 `skill.md`（YAML Frontmatter：name/description + Markdown 正文）
- `skill.json` / `manifest.json`（字段：name / display_name / description）
- `skill.yaml` / `manifest.yaml`（字段：name / display_name / description）
- `package.json`（字段：name / description）

安装时（`skills install <path>`）支持：

- 一个技能目录（包含 `scripts/`）
- 一个仓库根目录（包含 `skills/`，会批量导入）
- 仅提供 `scripts/` 目录（会自动补一个默认 `SKILL.md`）

导入后，可在 `>` 输入 `reload` 让模型立即加载新技能（无需重启）。

## 交互命令

在 `>` 提示符下可用：

- `help` / `/help` / `?`：显示帮助
- `skills` / `/skills`：列出可用技能脚本
- `skills show <name>`：查看单个技能的文档与脚本
- `skills search <keyword>`：按关键词搜索技能
- `skills install <path>`：从本地目录安装技能（支持多种常见目录结构）
- `skills install <path.zip>`：从 zip 导入技能（解压后按同样规则导入）
- `skills install git <https-url>`：从 git 仓库导入（默认禁用，需显式开启）
- `skills doctor`：检查已安装技能是否可执行
- `skills test <name>`：对某个技能做非执行检查（脚本存在性/OS 兼容性/大小限制等）
- `reload` / `/reload`：重新加载 system prompt（读取最新 skills/memory，无需重启）
- `clear` / `/clear`：清屏（打印多行空行）
- `reset` / `/reset`：清空会话 history（不删除文件）

## 非交互模式（CI/脚本）

支持在启动时用参数执行命令并退出，便于脚本化测试：

```powershell
go run .\cmd\nibot -cmd "skills" -cmd "skills doctor"
```

支持指定 workspace 路径：

```powershell
go run .\cmd\nibot -workspace "D:\path\to\workspace" -cmd "skills"
```

macOS / Linux（bash/zsh）：

```bash
go run ./cmd/nibot -workspace "/path/to/workspace" -cmd "skills"
```

