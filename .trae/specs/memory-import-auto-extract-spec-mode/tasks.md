# Tasks
- [ ] Task 1: 记忆库 schema 扩展与去重/更新能力
  - [ ] 1.1 为 memories 增加 fingerprint/source/updated_at 等字段与索引（向后兼容迁移）
  - [ ] 1.2 提供按 scope + fingerprint 的 upsert/merge 能力
  - [ ] 1.3 增加单元测试：迁移、去重、更新语义

- [ ] Task 2: 记忆导入（Import Memory）
  - [ ] 2.1 定义导入输入格式的解析器（支持多种文本块风格）
  - [ ] 2.2 增加导入入口（CLI 命令或工具调用），输出预览与审批
  - [ ] 2.3 脱敏与过滤（密钥/隐私/一次性信息）
  - [ ] 2.4 增加单元测试：解析、脱敏、去重、落库

- [ ] Task 3: 自动记忆提取（Auto Memory Extraction）
  - [ ] 3.1 定义记忆提取提示词与结构化输出格式（add/update/delete/ignore）
  - [ ] 3.2 在对话轮次结束后生成提案并走审批
  - [ ] 3.3 将审批后的变更写入记忆库并可回滚
  - [ ] 3.4 增加单元测试：提案解析、过滤、审批路径

- [ ] Task 4: Spec 模式流水线（程序内落地）
  - [ ] 4.1 增加 Spec 模式开关与触发方式（命令/前缀）
  - [ ] 4.2 生成 spec.md/tasks.md/checklist.md 到 `workspace/specs/<slug>/`
  - [ ] 4.3 增加“确认后实施”的 gating（未确认不执行改动类工具）
  - [ ] 4.4 增加单元测试：触发、生成、确认状态机

- [ ] Task 5: 文档与回归验证
  - [ ] 5.1 README/QUICKSTART 补充导入与自动提取的开关与使用方式
  - [ ] 5.2 `go test ./...` 与 `go vet ./...` 全量通过

# Task Dependencies
- Task 2/3 depend on Task 1（需要稳定的去重/更新语义）
- Task 4 can run in parallel with Task 1（接口对齐后集成）
