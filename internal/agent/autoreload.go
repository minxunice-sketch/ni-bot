package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func autoReloadEnabled() bool {
	return strings.TrimSpace(os.Getenv("NIBOT_AUTO_RELOAD")) == "1"
}

func autoReloadInterval() time.Duration {
	if v := strings.TrimSpace(os.Getenv("NIBOT_AUTO_RELOAD_INTERVAL_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n < 200 {
				n = 200
			}
			if n > 10000 {
				n = 10000
			}
			return time.Duration(n) * time.Millisecond
		}
	}
	return 1000 * time.Millisecond
}

func computeWorkspaceState(workspace string) (time.Time, error) {
	var latest time.Time

	check := func(p string) error {
		st, err := os.Stat(p)
		if err != nil {
			return nil
		}
		if st.ModTime().After(latest) {
			latest = st.ModTime()
		}
		return nil
	}

	_ = check(filepath.Join(workspace, "AGENT.md"))
	_ = check(filepath.Join(workspace, "data", "policy.toml"))

	for _, dir := range []string{"memory", "skills"} {
		root := filepath.Join(workspace, dir)
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := strings.ToLower(strings.TrimSpace(d.Name()))
				if name == "logs" || shouldIgnoreSkillDir(name) {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			return check(path)
		})
	}

	return latest, nil
}

func (c *LLMClient) StartAutoReload(logger *os.File) func() {
	stop := make(chan struct{})
	if !autoReloadEnabled() {
		close(stop)
		return func() {}
	}

	interval := autoReloadInterval()
	last, _ := computeWorkspaceState(c.Workspace)

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				cur, err := computeWorkspaceState(c.Workspace)
				if err != nil {
					continue
				}
				if !cur.After(last) {
					continue
				}

				systemPrompt, err := ConstructSystemPrompt(c.Workspace)
				if err != nil {
					continue
				}
				policy := LoadToolPolicy(c.Workspace)

				c.mu.Lock()
				c.SystemMsg = systemPrompt
				c.Config.Policy = policy
				c.mu.Unlock()

				last = cur
				if normalizeLogLevel(c.Config.LogLevel) == "meta" {
					writeLog(logger, fmt.Sprintf("\n## Auto Reload\n\n(prompt_bytes=%d)\n\n---\n", len([]byte(systemPrompt))))
				} else {
					writeLog(logger, "\n## Auto Reload\n\n```\n"+RedactForLog(systemPrompt)+"\n```\n\n---\n")
				}
			case <-stop:
				return
			}
		}
	}()

	return func() { close(stop) }
}

