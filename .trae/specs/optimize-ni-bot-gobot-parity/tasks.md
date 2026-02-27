# Tasks
- [x] Task 1: 抽取工具调用适配层并支持原生 Tool Calling
  - [x] 1.1 盘点现有 `[EXEC:...]` 执行链路与工具定义来源
  - [x] 1.2 为 OpenAI 兼容接口增加原生 tool call 的请求/响应解析
  - [x] 1.3 将 policy 与审批流程复用到两种调用通道
  - [x] 1.4 增加单元测试：原生 tool call 解析、审批与回退行为

- [x] Task 2: 增加可选 SQLite 存储后端
  - [x] 2.1 定义存储接口（会话/消息/审计最小字段集）
  - [x] 2.2 实现 SQLite 版本并默认关闭
  - [x] 2.3 保持现有文件会话与 markdown 审计日志不变
  - [x] 2.4 增加单元测试：启用 SQLite 时能写入并可查询

- [x] Task 3: 支持 skills 继承/覆盖与安装来源记录
  - [x] 3.1 定义继承/覆盖规则与目录约定（不引入破坏性变更）
  - [x] 3.2 扩展技能发现与执行：同名技能优先本地覆写，其余回退上游
  - [x] 3.3 增加最小验证用例：覆盖单脚本与读取描述

- [x] Task 4: 更新文档并清理对比文档
  - [x] 4.1 README/QUICKSTART 补充 Tool Calling/SQLite/继承的启用方式与注意事项
  - [x] 4.2 删除 `比较ni bot与go bot两个哪个更好.md`

- [x] Task 5: 端到端验证与回归
  - [x] 5.1 `go test ./...` 全量通过
  - [x] 5.2 CLI 启动验证（Mock 模式与启用工具的最小路径）

# Task Dependencies
- Task 2 depends on Task 1（SQLite 记录需复用工具调用审计结构）
- Task 3 can run in parallel with Task 2（接口对齐后集成）
