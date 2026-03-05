# 记忆导入 + 自动提取 + Spec 模式流水线 Spec

## Why
Ni bot 已具备可选 SQLite 长期记忆库与 Auto-Recall（按需注入）能力，但当前长期记忆主要依赖手工写入与手动维护，缺少：
- 从其他聊天系统（ChatGPT/Gemini/Manus/Claude 等）迁移“已积累的记忆/偏好”的低成本路径
- 从日常对话中自动提取“稳定且可复用的信息”并持续更新记忆库的闭环
- 统一的“需求 → 设计 → 任务 → 实施”工程化流程，避免优化工作变成不成体系的随手改动

本变更新增记忆导入与自动提取能力，并在程序内落地可开关的 Spec 工作流，使得每次提出需求时可自动生成规格文档，再按规格执行改动。

## What Changes
- 新增“记忆导入（Import Memory）”能力：支持从外部聊天系统导出的文本块中解析记忆条目，去重/合并后写入 SQLite 记忆库
- 新增“自动记忆提取（Auto Memory Extraction）”能力：每轮对话后由模型生成候选记忆变更（新增/更新/忽略/删除），经审批后写入记忆库
- 新增“Spec 模式流水线（Spec Pipeline）”能力：当启用 Spec 模式时，用户提出需求会自动生成 spec.md（需求+设计）、tasks.md（实施任务）、checklist.md（验收清单），并在用户确认后再执行实现
- 安全增强：对导入文本与自动提取结果做敏感信息脱敏与策略过滤，避免把 key/token/隐私写入记忆库

## Impact
- Affected specs: 长期记忆的迁移、自动化维护、可审计可回滚、以及工程化交付流程
- Affected code (expected): internal/agent/tools.go、internal/agent/sqlite_store.go、internal/agent/llm.go、cmd/nibot/main.go（或交互循环相关模块）
- Data: 扩展 `workspace/data/nibot.db` 中 memories 的字段/索引（向后兼容迁移）；新增 `workspace/specs/` 目录用于生成 spec 文档（默认关闭，仅在 Spec 模式下生成）

## ADDED Requirements

### Requirement: 记忆导入（Import Memory）
系统 SHALL 支持用户从其他聊天系统导出的“记忆/偏好文本块”导入到 Ni bot 的 SQLite 记忆库。

#### Constraints
- 系统 SHALL 在写入前进行脱敏（至少覆盖常见 API key/token/Authorization header）
- 系统 SHALL 对重复/高度相似内容进行去重，避免记忆膨胀
- 系统 SHALL 记录来源（source）与导入时间，便于审计与回滚
- 系统 SHALL 提供预览与审批：展示将新增/更新/删除的条目数量与示例，用户确认后才落库

#### Scenario: Success case（从 ChatGPT/Gemini/Manus 导出的文本粘贴导入）
- **WHEN** 用户提交一段导出的记忆文本块
- **THEN** 系统解析出若干条候选记忆
- **AND** 进行脱敏与过滤（剔除敏感信息）
- **AND** 进行去重/合并
- **AND** 在用户确认后写入 SQLite 记忆库

### Requirement: 自动记忆提取（Auto Memory Extraction）
系统 SHALL 支持在每轮对话结束后自动生成“记忆变更提案”，并在审批后写入 SQLite 记忆库。

#### Constraints
- 系统 SHALL 默认要求审批；仅在显式开关开启时允许自动写入
- 系统 SHALL 只提取“稳定且可复用”的信息（偏好、长期项目上下文、工作流约定等），并过滤个人隐私/密钥/一次性信息
- 系统 SHALL 支持更新语义：当新信息与旧记忆冲突时，优先以“更新/替换”而非简单叠加

#### Scenario: Success case（对话中形成新的稳定偏好）
- **WHEN** 用户多次表达同一偏好或明确声明长期偏好
- **THEN** 系统生成候选记忆（例如“输出风格：中文要点列表”）
- **AND** 用户审批后写入记忆库
- **AND** 后续对话通过 Auto-Recall 生效

### Requirement: Spec 模式流水线（Spec Pipeline）
系统 SHALL 支持在程序内启用 Spec 模式，使得“提出需求”时自动生成规格文件，并在用户确认后再执行实现。

#### Constraints
- 系统 SHALL 默认关闭 Spec 模式（避免影响日常对话）
- 系统 SHALL 提供清晰的触发方式（命令或前缀）与确认方式（例如 `approve`/`开始实施`）
- 系统 SHALL 将生成文件写入 workspace（例如 `workspace/specs/<slug>/`），不依赖 IDE 私有目录

#### Scenario: Success case（用户提出需求触发 Spec）
- **WHEN** Spec 模式已启用且用户输入一条“需求”
- **THEN** 系统生成：
  - spec.md（需求 + 设计）
  - tasks.md（实施任务）
  - checklist.md（验收清单）
- **AND** 系统提示用户审阅并确认
- **AND** 在用户确认后进入实施阶段

## MODIFIED Requirements

### Requirement: 长期记忆存储与 Auto-Recall
系统 SHALL 在新增导入/自动提取能力后，仍保持 Auto-Recall 的“按需注入”策略，避免全量注入导致 prompt 膨胀。

## Non-Goals
- 不提供对其他平台“直接 API 读取记忆”的能力（多数平台不开放或需要用户凭证）
- 不在默认模式下静默写入任何记忆（避免意外持久化）

