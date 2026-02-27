package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func InstallSkillsFromGitURL(workspace, url string) ([]string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("empty git url")
	}
	if os.Getenv("NIBOT_ENABLE_GIT") != "1" {
		return nil, fmt.Errorf("git install disabled (set NIBOT_ENABLE_GIT=1 to enable)")
	}
	if !isSafeGitURL(url) {
		return nil, fmt.Errorf("git url denied (only https:// URLs are allowed)")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found in PATH")
	}

	tmp := filepath.Join(os.TempDir(), "nibot_skill_git_"+safeBaseName(url))
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	cmd := exec.Command(gitPath, "clone", "--depth", "1", url, tmp)
	cmd.Dir = workspace
	if err := runWithTimeout(cmd, 3*time.Minute); err != nil {
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	return InstallSkillsFromPath(workspace, tmp)
}

func isSafeGitURL(url string) bool {
	u := strings.TrimSpace(url)
	if u == "" {
		return false
	}
	u = strings.ToLower(u)
	if strings.Contains(u, " ") || strings.Contains(u, "\t") || strings.Contains(u, "\n") || strings.Contains(u, "\r") {
		return false
	}
	if strings.HasPrefix(u, "https://") {
		return true
	}
	return false
}

