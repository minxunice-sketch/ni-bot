package agent

import (
	"fmt"
	"strings"
)

func normalizeLogLevel(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "meta", "full":
		return v
	default:
		return "full"
	}
}

func formatToolResultsMeta(results []ToolResult) string {
	var sb strings.Builder
	sb.WriteString("TOOL_RESULTS_META:\n")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("- tool: %s\n", r.Tool))
		sb.WriteString(fmt.Sprintf("  ok: %v\n", r.OK))
		if strings.TrimSpace(r.Error) != "" {
			sb.WriteString("  error: |\n")
			for _, line := range strings.Split(normalizeNewlines(redactSecrets(r.Error)), "\n") {
				if line == "" {
					sb.WriteString("    \n")
					continue
				}
				sb.WriteString("    " + line + "\n")
			}
		}
		out := strings.TrimSpace(r.Output)
		sb.WriteString(fmt.Sprintf("  output_bytes: %d\n", len([]byte(out))))
		if out != "" {
			first := firstLine(out)
			first = redactSecrets(first)
			sb.WriteString("  output_preview: |\n")
			sb.WriteString("    " + first + "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func firstLine(s string) string {
	s = normalizeNewlines(s)
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

