---
name: Weather
description: 查询城市天气（示例技能）
---

通过 `skill.exec` 调用 `scripts/weather.ps1`，参数为城市名。

示例：
[EXEC:skill.exec {"skill":"weather","script":"weather.ps1","args":["Beijing"],"timeoutSeconds":30}]

