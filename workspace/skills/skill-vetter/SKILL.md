# skill-vetter

对指定 GitHub 仓库进行基础安全评估，输出结构化 JSON 风险报告，用于自动学习/自动安装前的风险过滤。

## 用法

- Windows:
  - scripts/vet.ps1 <owner>/<repo>
- macOS / Linux:
  - scripts/vet.sh <owner>/<repo>

## 输出

标准输出为 JSON：

```json
{
  "ok": true,
  "riskLevel": "low",
  "score": 12,
  "signals": ["..."]
}
```

