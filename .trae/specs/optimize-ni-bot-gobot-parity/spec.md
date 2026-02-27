# Ni bot 参考 Go bot 的优化 Spec

## Why
现有 Ni bot 已可运行，但工具调用依赖文本标签、会话/记忆以文件为主，扩展技能缺少继承/覆盖层，易在跨平台与协作时出现“仓库内容不完整、配置示例误复制、能力扩展不一致”等问题。
本变更参考对比文档的要点，提升工具调用稳定性、记忆存储一致性与技能扩展能力，同时保持默认行为不破坏现有用户使用方式。

## What Changes
- 新增“原生 Tool Calling（函数调用）”的可选通道：在 OpenAI 兼容接口支持的前提下，将工具定义以结构化形式提供给模型，并解析结构化 tool call 执行
- 保留并继续支持现有 `[EXEC:...]` 标签机制作为兼容/回退通道（默认行为保持不变）
- 新增可选 SQLite 存储后端，用于持久化会话/消息/工具审计（默认仍使用现有文件持久化，避免 **BREAKING**）
- 增强 skills 机制：支持技能继承/覆盖（“三方继承”）与安装来源记录，允许在不修改上游技能包的情况下本地覆写脚本或 SKILL.md
- 修正文档中与 NVIDIA NIM 相关的易错点说明（例如 base_url 误加反引号/空格、直接访问 base_url 返回 404 的误判）
- 完成优化后删除对比文档 `比较ni bot与go bot两个哪个更好.md`（作为项目交付清理项）

## Impact
- Affected specs: 工具调用可靠性、记忆/审计持久化、技能扩展与复用、跨平台可运行性
- Affected code: internal/agent/llm.go、internal/agent/tools.go、internal/agent/session.go、internal/agent/skills*.go、internal/agent/config.go、README.md、QUICKSTART.md
- Data: 新增（可选）`workspace/data/nibot.db`；默认不启用

## ADDED Requirements

### Requirement: Native Tool Calling（可选）
系统 SHALL 在满足以下条件时使用原生 Tool Calling：
- Provider 为 OpenAI 兼容接口且声明支持工具调用
- 用户显式启用（例如通过配置/环境变量）

#### Scenario: Success case（原生工具调用）
- **WHEN** 用户启用原生工具调用并请求需要读取文件的任务
- **THEN** 模型返回结构化 tool call（而非 `[EXEC:...]` 文本）
- **AND** 系统按现有 policy + 审批流程执行工具
- **AND** 将工具调用结果回填给模型继续推理直至完成

#### Scenario: Fallback（兼容回退）
- **WHEN** Provider 不支持工具调用或用户未启用
- **THEN** 系统使用现有 `[EXEC:...]` 解析与执行链路

### Requirement: SQLite 存储后端（可选）
系统 SHALL 在启用 SQLite 存储时，将会话元数据、消息与工具审计写入 `workspace/data/nibot.db`。

#### Scenario: Success case（SQLite 持久化）
- **WHEN** 用户启用 SQLite 存储并运行一次会话
- **THEN** 退出后数据库中可查询到对应 session 与消息记录
- **AND** 不影响现有 `workspace/logs/*.md` 审计日志生成（仍保留）

### Requirement: Skills 继承/覆盖（“三方继承”）
系统 SHALL 支持技能包的继承/覆盖规则，使得：
- 上游技能（第三方）可被安装到 workspace
- 本地同名技能可声明“覆盖/扩展”，在执行时优先使用本地覆写的脚本或描述

#### Scenario: Success case（本地覆写第三方技能）
- **WHEN** 用户安装第三方技能包 A
- **AND** 本地创建技能 A 的覆写层（仅替换 scripts 中某个脚本）
- **THEN** 执行技能脚本时使用本地版本
- **AND** 仍可读取第三方版本的其余脚本作为回退

## MODIFIED Requirements

### Requirement: Tool Policy 与审批一致性
系统 SHALL 对所有工具调用通道（原生 Tool Calling 与 `[EXEC:...]`）应用相同的 policy 校验与审批流程，避免绕过安全限制。

## REMOVED Requirements

### Requirement: 交付后保留对比文档
**Reason**: 对比文档包含阶段性判断，交付后易产生误导（例如“Ni bot 无法运行”的历史问题已解决）。
**Migration**: 交付前将其内容转化为可执行的改进项并落地；交付时删除该文件。
