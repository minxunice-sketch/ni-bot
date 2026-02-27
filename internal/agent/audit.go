package agent

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func writeAuditApproval(logger *os.File, logLevel string, call ExecCall, approved bool) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	argsPreview := redactSecrets(previewArgs(call.ArgsRaw))
	decision := "deny"
	if approved {
		decision = "allow"
	}
	line := fmt.Sprintf("- %s approval %s tool=%s args=%q\n", ts, decision, call.Tool, argsPreview)
	writeLog(logger, ensureAuditHeader(logLevel))
	writeLog(logger, line)
}

func writeAuditToolResults(logger *os.File, logLevel string, calls []ExecCall, results []ToolResult) {
	if len(calls) == 0 || len(results) == 0 {
		return
	}
	writeLog(logger, ensureAuditHeader(logLevel))
	ts := time.Now().Format("2006-01-02 15:04:05")
	for i := 0; i < len(calls) && i < len(results); i++ {
		call := calls[i]
		r := results[i]

		errPreview := ""
		if strings.TrimSpace(r.Error) != "" {
			errPreview = firstLine(redactSecrets(r.Error))
		}
		outBytes := len([]byte(strings.TrimSpace(r.Output)))
		argsPreview := redactSecrets(previewArgs(call.ArgsRaw))

		if normalizeLogLevel(logLevel) == "meta" {
			writeLog(logger, fmt.Sprintf("- %s tool=%s ok=%v args_bytes=%d output_bytes=%d error=%q\n",
				ts, call.Tool, r.OK, len([]byte(strings.TrimSpace(call.ArgsRaw))), outBytes, errPreview))
		} else {
			writeLog(logger, fmt.Sprintf("- %s tool=%s ok=%v args=%q output_bytes=%d error=%q\n",
				ts, call.Tool, r.OK, argsPreview, outBytes, errPreview))
		}
	}
}

func ensureAuditHeader(logLevel string) string {
	if normalizeLogLevel(logLevel) == "meta" {
		return "\n### Audit (meta)\n"
	}
	return "\n### Audit\n"
}

