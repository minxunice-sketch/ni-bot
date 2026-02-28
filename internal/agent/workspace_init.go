package agent

import (
	"os"
	"path/filepath"
)

func EnsureWorkspaceScaffold(workspace string) error {
	dirs := []string{
		workspace,
		filepath.Join(workspace, "logs"),      // 必开：日志目录
		filepath.Join(workspace, "memory"),    // 必开：记忆库目录
		filepath.Join(workspace, "data"),
		filepath.Join(workspace, "data", "sessions"),
		filepath.Join(workspace, "skills"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	identityPath := filepath.Join(workspace, "AGENT.md")
	if _, err := os.Stat(identityPath); err == nil {
		return nil
	}
	if err := os.WriteFile(identityPath, []byte(defaultAgentIdentity()), 0644); err != nil {
		return err
	}
	return nil
}

func defaultAgentIdentity() string {
	return "# Ni bot\n\n你是一个极简、文件驱动的 AI Agent。你会优先使用 workspace 中的 identity/memory/skills 来完成任务，并遵守工具审批与安全策略。\n"
}
