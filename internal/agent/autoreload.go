package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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

	check := func(mod time.Time) {
		if mod.After(latest) {
			latest = mod
		}
	}

	if st, err := os.Stat(filepath.Join(workspace, "AGENT.md")); err == nil {
		check(st.ModTime())
	}
	if st, err := os.Stat(filepath.Join(workspace, "data", "policy.toml")); err == nil {
		check(st.ModTime())
	}

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
			info, err := d.Info()
			if err != nil {
				return nil
			}
			check(info.ModTime())
			return nil
		})
	}

	return latest, nil
}

func (c *LLMClient) StartAutoReload(logger *os.File) func() {
	if !autoReloadEnabled() {
		return func() {}
	}

	stop := make(chan struct{})
	stopOnce := sync.Once{}
	stopFn := func() { stopOnce.Do(func() { close(stop) }) }

	if fsStop, ok := c.startAutoReloadFSNotify(logger, stop); ok {
		return func() {
			stopFn()
			fsStop()
		}
	}
	c.startAutoReloadPoll(logger, stop)
	return stopFn
}

func (c *LLMClient) startAutoReloadPoll(logger *os.File, stop <-chan struct{}) {
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
				if !c.reloadPromptAndPolicy(logger) {
					continue
				}
				last = cur
			case <-stop:
				return
			}
		}
	}()
}

func (c *LLMClient) startAutoReloadFSNotify(logger *os.File, stop <-chan struct{}) (func(), bool) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return func() {}, false
	}
	if err := addWorkspaceWatches(w, c.Workspace); err != nil {
		_ = w.Close()
		return func() {}, false
	}

	var closeOnce sync.Once
	closeWatcher := func() { closeOnce.Do(func() { _ = w.Close() }) }

	go func() {
		defer closeWatcher()

		debounce := 200 * time.Millisecond
		var timer *time.Timer
		var timerC <-chan time.Time
		resetTimer := func() {
			if timer == nil {
				timer = time.NewTimer(debounce)
				timerC = timer.C
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
			timerC = timer.C
		}

		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if shouldIgnoreWatchPath(c.Workspace, ev.Name) {
					continue
				}
				if ev.Op&fsnotify.Create != 0 {
					if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
						_ = addWatchDirRecursive(w, ev.Name)
					}
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) != 0 {
					resetTimer()
				}
			case <-timerC:
				_ = c.reloadPromptAndPolicy(logger)
				timerC = nil
			case <-stop:
				return
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return closeWatcher, true
}

func (c *LLMClient) reloadPromptAndPolicy(logger *os.File) bool {
	systemPrompt, err := ConstructSystemPrompt(c.Workspace)
	if err != nil {
		return false
	}
	policy := LoadToolPolicy(c.Workspace)

	c.mu.Lock()
	c.SystemMsg = systemPrompt
	c.Config.Policy = policy
	logLevel := c.Config.LogLevel
	c.mu.Unlock()

	if normalizeLogLevel(logLevel) == "meta" {
		writeLog(logger, fmt.Sprintf("\n## Auto Reload\n\n(prompt_bytes=%d)\n\n---\n", len([]byte(systemPrompt))))
	} else {
		writeLog(logger, "\n## Auto Reload\n\n```\n"+RedactForLog(systemPrompt)+"\n```\n\n---\n")
	}
	return true
}

func addWorkspaceWatches(w *fsnotify.Watcher, workspace string) error {
	var roots []string
	roots = append(roots, workspace)
	roots = append(roots, filepath.Join(workspace, "data"))
	roots = append(roots, filepath.Join(workspace, "memory"))
	roots = append(roots, filepath.Join(workspace, "skills"))

	for _, r := range roots {
		_ = addWatchDirRecursive(w, r)
	}
	return nil
}

func addWatchDirRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := strings.ToLower(strings.TrimSpace(d.Name()))
		if name == "logs" || shouldIgnoreSkillDir(name) {
			return filepath.SkipDir
		}
		_ = w.Add(path)
		return nil
	})
}

func shouldIgnoreWatchPath(workspace, absPath string) bool {
	rel, err := filepath.Rel(workspace, absPath)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	parts := strings.Split(rel, string(filepath.Separator))
	for _, p := range parts {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == "" || n == "." {
			continue
		}
		if n == "logs" || shouldIgnoreSkillDir(n) {
			return true
		}
	}
	return false
}
