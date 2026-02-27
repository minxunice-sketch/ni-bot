---
name: "security-hardening"
description: "增强Ni bot安全防护能力，防止OpenClaw类漏洞。包含API密钥保护、输入验证、会话隔离和安全审计功能。"
---

# 安全加固技能

## 功能特性

### 1. API密钥保护
- 自动检测和脱敏API密钥（仅显示前后4位）
- 防止密钥泄露到日志和终端输出
- 环境变量加密存储支持

### 2. 输入验证和过滤
- 防止提示注入攻击
- SQL注入和命令注入防护
- 恶意URL和文件路径检测

### 3. 会话隔离
- 严格的会话边界控制
- 防止跨会话数据泄露
- 独立的工具执行上下文

### 4. 安全审计
- 完整的操作审计日志
- 异常行为检测
- 实时安全事件监控

## 使用方法

```bash
# 启用安全模式
export NIBOT_SECURITY_MODE=strict

# 配置审计日志
export NIBOT_AUDIT_LOG=/path/to/audit.log

# 启用API密钥保护
export NIBOT_REDACT_KEYS=true
```

## 安全最佳实践

1. **永远不要**在日志或终端输出完整API密钥
2. 使用环境变量或加密配置文件存储敏感信息
3. 定期审查审计日志中的异常活动
4. 为不同用途创建独立的会话上下文
5. 限制第三方技能的权限范围

## 集成检查

此技能与Ni bot现有的以下安全功能集成：
- 策略引擎（policy.go）
- 审计系统（audit.go）
- 脱敏功能（redact.go）
- 沙箱执行（sandbox.go）