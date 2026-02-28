package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ExecCall struct {
	Tool    string
	ArgsRaw string
}

type ToolResult struct {
	Tool   string
	OK     bool
	Output string
	Error  string
}

type ExecContext struct {
	Workspace string
	Policy    ToolPolicy
}

type Approver interface {
	Approve(call ExecCall) bool
}

func ExtractExecCalls(text string) []ExecCall {
	const prefix = "[EXEC:"
	var calls []ExecCall

	for i := 0; i < len(text); {
		idx := strings.Index(text[i:], prefix)
		if idx == -1 {
			break
		}
		start := i + idx
		j := start + len(prefix)

		toolStart := j
		for j < len(text) {
			ch := text[j]
			if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == ']' {
				break
			}
			j++
		}
		tool := strings.TrimSpace(text[toolStart:j])
		for j < len(text) && (text[j] == ' ' || text[j] == '\t') {
			j++
		}
		argsStart := j

		inString := false
		escape := false
		braceDepth := 0
		bracketDepth := 0

		for j < len(text) {
			ch := text[j]

			if inString {
				if escape {
					escape = false
					j++
					continue
				}
				if ch == '\\' {
					escape = true
					j++
					continue
				}
				if ch == '"' {
					inString = false
				}
				j++
				continue
			}

			switch ch {
			case '"':
				inString = true
			case '{':
				braceDepth++
			case '}':
				if braceDepth > 0 {
					braceDepth--
				}
			case '[':
				bracketDepth++
			case ']':
				if braceDepth == 0 && bracketDepth == 0 {
					args := strings.TrimSpace(text[argsStart:j])
					if tool != "" {
						calls = append(calls, ExecCall{Tool: tool, ArgsRaw: args})
					}
					j++
					i = j
					goto next
				}
				if bracketDepth > 0 {
					bracketDepth--
				}
			}
			j++
		}

		break
	next:
	}

	if len(calls) == 0 {
		return nil
	}
	return calls
}

func ExecuteCalls(ctx ExecContext, calls []ExecCall, approver Approver) []ToolResult {
	if !ctx.Policy.Loaded {
		ctx.Policy = DefaultToolPolicy()
	}

	results := make([]ToolResult, 0, len(calls))
	for _, call := range calls {
		if !ctx.Policy.AllowsTool(call.Tool) {
			results = append(results, ToolResult{
				Tool:   call.Tool,
				OK:     false,
				Error:  "disabled by policy",
				Output: "",
			})
			continue
		}

		// 检查是否需要审批 - 支持静默授权模式
		if ctx.Policy.RequiresApproval(call.Tool) && approver != nil && os.Getenv("NIBOT_AUTO_APPROVE") != "true" {
			if !approver.Approve(call) {
				results = append(results, ToolResult{
					Tool:   call.Tool,
					OK:     false,
					Error:  "denied by user",
					Output: "",
				})
				continue
			}
		}

		res := executeOne(ctx, call)
		results = append(results, res)
	}
	return results
}

func executeOne(ctx ExecContext, call ExecCall) ToolResult {
	switch call.Tool {
	case "fs.read", "file_read":
		out, err := toolFSRead(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "fs.write", "file_write":
		out, err := toolFSWrite(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "memory.store":
		out, err := toolMemoryStore(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "memory.recall":
		out, err := toolMemoryRecall(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "memory.forget":
		out, err := toolMemoryForget(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "memory.list":
		out, err := toolMemoryList(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "memory.stats":
		out, err := toolMemoryStats(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "skills.install", "install_skill", "skill_store_install":
		out, err := toolInstallSkill(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "runtime.exec", "shell_exec":
		if !ctx.Policy.AllowsTool(call.Tool) {
			return ToolResult{Tool: call.Tool, OK: false, Error: "disabled by policy"}
		}
		out, err := toolRuntimeExec(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "skill.exec":
		if !ctx.Policy.AllowsTool(call.Tool) {
			return ToolResult{Tool: call.Tool, OK: false, Error: "disabled by policy"}
		}
		out, err := toolSkillExec(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	case "skill_exec":
		if !ctx.Policy.AllowsTool(call.Tool) {
			return ToolResult{Tool: call.Tool, OK: false, Error: "disabled by policy"}
		}
		out, err := toolSkillExec(ctx, call.ArgsRaw)
		if err != nil {
			return ToolResult{Tool: call.Tool, OK: false, Error: err.Error(), Output: out}
		}
		return ToolResult{Tool: call.Tool, OK: true, Output: out}
	default:
		return ToolResult{Tool: call.Tool, OK: false, Error: "unknown tool"}
	}
}

type fsReadArgs struct {
	Path string `json:"path"`
}

func toolFSRead(ctx ExecContext, argsRaw string) (string, error) {
	var path string
	if strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		var a fsReadArgs
		if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
			return "", fmt.Errorf("invalid JSON args for fs.read: %w", err)
		}
		path = a.Path
	} else {
		path = strings.TrimSpace(argsRaw)
	}

	path = normalizeWorkspaceRelPath(path)
	if path == "" {
		return "", fmt.Errorf("fs.read requires path")
	}

	abs, err := resolveWorkspacePath(ctx.Workspace, path)
	if err != nil {
		return "", err
	}

	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()

	const maxBytes = 256 * 1024
	limited := io.LimitReader(f, maxBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(b) > maxBytes {
		b = b[:maxBytes]
		return string(b) + "\n\n[TRUNCATED]", nil
	}
	return string(b), nil
}

type fsWriteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode"`
}

func toolFSWrite(ctx ExecContext, argsRaw string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("fs.write requires JSON args: {\"path\":\"...\",\"content\":\"...\",\"mode\":\"append|overwrite\"}")
	}
	var a fsWriteArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for fs.write: %w", err)
	}
	a.Path = normalizeWorkspaceRelPath(a.Path)
	if a.Path == "" {
		return "", fmt.Errorf("fs.write requires path")
	}
	if len(a.Content) > 512*1024 {
		return "", fmt.Errorf("fs.write content too large")
	}
	mode := strings.ToLower(strings.TrimSpace(a.Mode))
	if mode == "" {
		mode = "append"
	}
	if mode != "append" && mode != "overwrite" {
		return "", fmt.Errorf("fs.write invalid mode: %s", mode)
	}
	if mode == "overwrite" {
		base := strings.ToLower(filepath.Base(filepath.ToSlash(a.Path)))
		if base == "facts.md" || base == "reflections.md" || base == "agent.md" {
			return "", fmt.Errorf("fs.write overwrite denied for protected file: %s", a.Path)
		}
	}

	if !isAllowedWritePath(a.Path) {
		return "", fmt.Errorf("fs.write denied for path (allowed: memory/, skills/, logs/)")
	}
	if !ctx.Policy.AllowsWritePath(a.Path) {
		return "", fmt.Errorf("fs.write denied by policy")
	}

	// 构建完整路径并确保绝对路径
	absWorkspace, _ := filepath.Abs(ctx.Workspace)
	relPath := normalizeWorkspacePath(a.Path, absWorkspace)
	abs := filepath.Join(absWorkspace, relPath)
	
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if mode == "overwrite" {
		if err := os.WriteFile(abs, []byte(a.Content), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("overwrote %d bytes to %s", len(a.Content), a.Path), nil
	}

	var prefix string
	if st, err := os.Stat(abs); err == nil && st.Size() > 0 {
		if !strings.HasPrefix(a.Content, "\n") {
			prefix = "\n"
		}
	}

	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	n1, err := f.WriteString(prefix)
	if err != nil {
		return "", err
	}
	n2, err := f.WriteString(a.Content)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("appended %d bytes to %s", n1+n2, a.Path), nil
}

func isAllowedWritePath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" {
		return false
	}
	
	// 可信目录白名单
	trustedDirs := []string{
		"memory/",
		"skills/", 
		"logs/",
		"workspace/",
		"data/",
	}
	
	for _, dir := range trustedDirs {
		if strings.HasPrefix(p, dir) {
			return true
		}
	}
	
	return false
}

// 路径容错逻辑：去除重复的workspace前缀
func normalizeWorkspacePath(path, absWorkspace string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	absWorkspaceSlash := filepath.ToSlash(absWorkspace) + "/"
	
	// 如果路径包含绝对workspace路径，去除重复部分
	if strings.HasPrefix(path, absWorkspaceSlash) {
		path = strings.TrimPrefix(path, absWorkspaceSlash)
	}
	
	// 去除重复的workspace前缀
	path = strings.TrimPrefix(path, "workspace/")
	path = strings.TrimPrefix(path, "workspace\\")
	
	return path
}

type runtimeExecArgs struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func toolRuntimeExec(ctx ExecContext, argsRaw string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("runtime.exec requires JSON args: {\"command\":\"...\",\"timeoutSeconds\":10}")
	}
	var a runtimeExecArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for runtime.exec: %w", err)
	}
	if strings.TrimSpace(a.Command) == "" {
		return "", fmt.Errorf("runtime.exec requires command")
	}
	if os.Getenv("NIBOT_ENABLE_EXEC") != "1" && !isSafeRuntimeCommandWhenExecDisabled(a.Command) {
		return "", fmt.Errorf("runtime.exec disabled (set NIBOT_ENABLE_EXEC=1 to enable)")
	}
	if !ctx.Policy.AllowsRuntimeCommand(a.Command) {
		return "", fmt.Errorf("runtime.exec command denied by policy")
	}
	timeout := time.Duration(a.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}

	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"powershell", "-NoProfile", "-Command", a.Command}
	} else {
		argv = []string{"sh", "-lc", a.Command}
	}
	argv, err := wrapWithSandbox(argv)
	if err != nil {
		return "", err
	}
	release := acquireExecSlot()
	defer release()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = ctx.Workspace

	maxOut := execMaxOutputBytes()
	stdout := newCappedBuffer(maxOut)
	stderr := newCappedBuffer(maxOut)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := runWithTimeout(cmd, timeout); err != nil {
		out := strings.TrimSpace(stdout.String())
		er := strings.TrimSpace(stderr.String())
		if er == "" {
			er = err.Error()
		} else {
			er = er + "\n" + err.Error()
		}
		return formatExecOutput(out, er), fmt.Errorf("runtime.exec failed")
	}

	return formatExecOutput(strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())), nil
}

func formatExecOutput(out string, err string) string {
	if out == "" && err == "" {
		return "(no output)"
	}
	if err == "" {
		return out
	}
	if out == "" {
		return "STDERR:\n" + err
	}
	return "STDOUT:\n" + out + "\n\nSTDERR:\n" + err
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return fmt.Errorf("timeout after %s", timeout)
	}
}

func normalizeWorkspaceRelPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return p
	}
	s := filepath.ToSlash(p)
	s = strings.TrimLeft(s, "/")
	for {
		l := strings.ToLower(s)
		if strings.HasPrefix(l, "workspace/") {
			s = strings.TrimLeft(s[len("workspace/"):], "/")
			continue
		}
		break
	}
	return s
}

func containsShellMeta(s string) bool {
	return strings.ContainsAny(s, ";&|`$><\n\r")
}

func isSafeRuntimeCommandWhenExecDisabled(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	if containsShellMeta(command) {
		return false
	}
	tokens := splitCommandLine(command)
	if len(tokens) == 0 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(tokens[0]))

	switch first {
	case "ls":
		for _, a := range tokens[1:] {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			if strings.HasPrefix(a, "-") {
				continue
			}
			if filepath.IsAbs(a) || strings.Contains(a, "..") || strings.HasPrefix(a, "~") {
				return false
			}
		}
		return true
	case "dir":
		for _, a := range tokens[1:] {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			if filepath.IsAbs(a) || strings.Contains(a, "..") || strings.HasPrefix(a, "~") {
				return false
			}
		}
		return true
	case "git":
		if len(tokens) != 4 {
			return false
		}
		if strings.ToLower(strings.TrimSpace(tokens[1])) != "clone" {
			return false
		}
		url := strings.TrimSpace(tokens[2])
		dest := strings.TrimSpace(tokens[3])
		if !isSafeGitURL(url) {
			return false
		}
		if dest == "" || filepath.IsAbs(dest) || strings.Contains(dest, "..") || strings.HasPrefix(dest, "~") {
			return false
		}
		dest = filepath.ToSlash(dest)
		if !strings.HasPrefix(strings.ToLower(dest), "skills/") {
			return false
		}
		return true
	default:
		return false
	}
}

func resolveWorkspacePath(workspace string, p string) (string, error) {
	if strings.Contains(p, "\x00") {
		return "", fmt.Errorf("invalid path")
	}
	p = normalizeWorkspaceRelPath(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	cleanRel := filepath.Clean(p)
	if cleanRel == "." || cleanRel == string(filepath.Separator) {
		return "", fmt.Errorf("invalid path")
	}
	if strings.HasPrefix(cleanRel, "..") || strings.Contains(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	targetAbs := filepath.Join(workspaceAbs, cleanRel)
	targetAbs, err = filepath.Abs(targetAbs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(workspaceAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path escapes workspace")
	}
	return targetAbs, nil
}

type installSkillArgs struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Layer string `json:"layer"`
}

func toolInstallSkill(ctx ExecContext, argsRaw string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("install_skill requires JSON args: {\"name\":\"evomap\",\"url\":\"https://...\",\"layer\":\"upstream\"}")
	}
	var a installSkillArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for install_skill: %w", err)
	}
	a.Name = strings.TrimSpace(a.Name)
	a.URL = strings.TrimSpace(a.URL)
	a.Layer = strings.ToLower(strings.TrimSpace(a.Layer))
	if a.Layer == "" {
		a.Layer = "upstream"
	}
	if a.Name == "" {
		return "", fmt.Errorf("install_skill requires name")
	}
	if a.URL == "" {
		return "", fmt.Errorf("install_skill requires url (https://...)")
	}
	if !isSafeGitURL(a.URL) {
		return "", fmt.Errorf("install_skill denied: only https:// URLs are allowed")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found in PATH")
	}

	tmp := filepath.Join(os.TempDir(), "nibot_skill_git_"+safeBaseName(a.URL))
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	cmd := exec.Command(gitPath, "clone", "--depth", "1", a.URL, tmp)
	cmd.Dir = ctx.Workspace
	if err := runWithTimeout(cmd, 3*time.Minute); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	installed, err := InstallSkillsFromPathWithOrigin(ctx.Workspace, tmp, a.URL, a.Layer)
	if err != nil {
		return "", err
	}
	return "installed skills: " + strings.Join(installed, ", "), nil
}

type memoryStoreArgs struct {
	Scope   string `json:"scope"`
	Tags    string `json:"tags"`
	Content string `json:"content"`
}

func toolMemoryStore(ctx ExecContext, argsRaw string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("memory.store requires JSON args: {\"scope\":\"global\",\"tags\":\"...\",\"content\":\"...\"}")
	}
	var a memoryStoreArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for memory.store: %w", err)
	}
	a.Scope = stringsTrimSpace(a.Scope)
	if a.Scope == "" {
		a.Scope = "global"
	}
	a.Tags = stringsTrimSpace(a.Tags)
	a.Content = stringsTrimSpace(a.Content)
	if a.Content == "" {
		return "", fmt.Errorf("memory.store requires content")
	}
	a.Content = redactSecrets(a.Content)

	s, err := OpenSQLiteStore(ctx.Workspace)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("memory db disabled (set NIBOT_MEMORY_DB=sqlite or NIBOT_STORAGE=sqlite)")
	}
	defer s.Close()

	id, err := s.InsertMemory(a.Scope, a.Tags, a.Content)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("stored memory id=%d scope=%s", id, a.Scope), nil
}

type memoryRecallArgs struct {
	Scope string `json:"scope"`
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func toolMemoryRecall(ctx ExecContext, argsRaw string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("memory.recall requires JSON args: {\"query\":\"...\",\"scope\":\"global\",\"limit\":10}")
	}
	var a memoryRecallArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for memory.recall: %w", err)
	}
	a.Scope = stringsTrimSpace(a.Scope)
	a.Query = stringsTrimSpace(a.Query)
	if a.Query == "" {
		return "", fmt.Errorf("memory.recall requires query")
	}

	s, err := OpenSQLiteStore(ctx.Workspace)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("memory db disabled (set NIBOT_MEMORY_DB=sqlite or NIBOT_STORAGE=sqlite)")
	}
	defer s.Close()

	items, err := s.SearchMemories(a.Scope, a.Query, a.Limit)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "(no matches)", nil
	}
	var sb strings.Builder
	for _, it := range items {
		sb.WriteString(fmt.Sprintf("- id=%d scope=%s tags=%s: %s\n", it.ID, it.Scope, it.Tags, previewText(it.Content, 240)))
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

type memoryForgetArgs struct {
	ID int64 `json:"id"`
}

func toolMemoryForget(ctx ExecContext, argsRaw string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("memory.forget requires JSON args: {\"id\":123}")
	}
	var a memoryForgetArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for memory.forget: %w", err)
	}
	if a.ID <= 0 {
		return "", fmt.Errorf("memory.forget requires id")
	}

	s, err := OpenSQLiteStore(ctx.Workspace)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("memory db disabled (set NIBOT_MEMORY_DB=sqlite or NIBOT_STORAGE=sqlite)")
	}
	defer s.Close()

	if err := s.DeleteMemory(a.ID); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted memory id=%d", a.ID), nil
}

type memoryListArgs struct {
	Scope string `json:"scope"`
	Limit int    `json:"limit"`
}

func toolMemoryList(ctx ExecContext, argsRaw string) (string, error) {
	scope := ""
	limit := 50
	if strings.TrimSpace(argsRaw) != "" {
		if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
			return "", fmt.Errorf("memory.list requires JSON args: {\"scope\":\"global\",\"limit\":50}")
		}
		var a memoryListArgs
		if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
			return "", fmt.Errorf("invalid JSON args for memory.list: %w", err)
		}
		scope = stringsTrimSpace(a.Scope)
		if a.Limit > 0 {
			limit = a.Limit
		}
	}

	s, err := OpenSQLiteStore(ctx.Workspace)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("memory db disabled (set NIBOT_MEMORY_DB=sqlite or NIBOT_STORAGE=sqlite)")
	}
	defer s.Close()

	items, err := s.ListMemories(scope, limit)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "(empty)", nil
	}
	var sb strings.Builder
	for _, it := range items {
		sb.WriteString(fmt.Sprintf("- id=%d scope=%s tags=%s: %s\n", it.ID, it.Scope, it.Tags, previewText(it.Content, 200)))
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func toolMemoryStats(ctx ExecContext, argsRaw string) (string, error) {
	_ = argsRaw
	s, err := OpenSQLiteStore(ctx.Workspace)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("memory db disabled (set NIBOT_MEMORY_DB=sqlite or NIBOT_STORAGE=sqlite)")
	}
	defer s.Close()

	n, err := s.MemoryStats()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("memories=%d", n), nil
}

func previewText(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 {
		max = 200
	}
	if len([]byte(s)) <= max {
		return s
	}
	b := []byte(s)
	if max > len(b) {
		max = len(b)
	}
	return string(b[:max]) + "..."
}

type skillExecArgs struct {
	Skill          string   `json:"skill"`
	Script         string   `json:"script"`
	Args           []string `json:"args"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

func toolSkillExec(ctx ExecContext, argsRaw string) (string, error) {
	if os.Getenv("NIBOT_ENABLE_SKILLS") != "1" {
		return "", fmt.Errorf("skill.exec disabled (set NIBOT_ENABLE_SKILLS=1 to enable)")
	}
	if !strings.HasPrefix(strings.TrimSpace(argsRaw), "{") {
		return "", fmt.Errorf("skill.exec requires JSON args: {\"skill\":\"...\",\"script\":\"...\",\"args\":[...],\"timeoutSeconds\":30}")
	}
	var a skillExecArgs
	if err := json.Unmarshal([]byte(argsRaw), &a); err != nil {
		return "", fmt.Errorf("invalid JSON args for skill.exec: %w", err)
	}
	a.Skill = strings.TrimSpace(a.Skill)
	a.Script = strings.TrimSpace(a.Script)
	if a.Skill == "" || a.Script == "" {
		return "", fmt.Errorf("skill.exec requires skill and script")
	}
	if strings.Contains(a.Skill, "..") || strings.Contains(a.Script, "..") {
		return "", fmt.Errorf("invalid skill/script")
	}
	if !ctx.Policy.AllowsSkillExec(a.Skill, a.Script) {
		return "", fmt.Errorf("skill.exec denied by policy")
	}

	abs, err := resolveSkillScriptPath(ctx.Workspace, a.Skill, a.Script)
	if err != nil {
		return "", err
	}

	timeout := time.Duration(a.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}

	var argv []string
	if runtime.GOOS == "windows" {
		if strings.HasSuffix(strings.ToLower(a.Script), ".ps1") {
			argv = []string{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", abs}
			if len(a.Args) > 0 {
				argv = append(argv, a.Args...)
			}
		} else if strings.HasSuffix(strings.ToLower(a.Script), ".bat") || strings.HasSuffix(strings.ToLower(a.Script), ".cmd") {
			argv = []string{"cmd", "/c", abs}
			if len(a.Args) > 0 {
				argv = append(argv, a.Args...)
			}
		} else {
			argv = append([]string{abs}, a.Args...)
		}
	} else {
		if strings.HasSuffix(strings.ToLower(a.Script), ".sh") {
			argv = []string{"sh", abs}
			if len(a.Args) > 0 {
				argv = append(argv, a.Args...)
			}
		} else {
			argv = append([]string{abs}, a.Args...)
		}
	}
	argv, err = wrapWithSandbox(argv)
	if err != nil {
		return "", err
	}
	release := acquireExecSlot()
	defer release()
	cmd := exec.Command(argv[0], argv[1:]...)

	cmd.Dir = ctx.Workspace

	maxOut := execMaxOutputBytes()
	stdout := newCappedBuffer(maxOut)
	stderr := newCappedBuffer(maxOut)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := runWithTimeout(cmd, timeout); err != nil {
		out := strings.TrimSpace(stdout.String())
		er := strings.TrimSpace(stderr.String())
		if er == "" {
			er = err.Error()
		} else {
			er = er + "\n" + err.Error()
		}
		return formatExecOutput(out, er), fmt.Errorf("skill.exec failed: %w", err)
	}

	return formatExecOutput(strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())), nil
}

func resolveSkillScriptPath(workspace, skill, script string) (string, error) {
	candidates := []string{
		filepath.ToSlash(filepath.Join("skills", "_overrides", skill, "scripts", script)),
		filepath.ToSlash(filepath.Join("skills", skill, "scripts", script)),
		filepath.ToSlash(filepath.Join("skills", "_upstream", skill, "scripts", script)),
	}
	for _, rel := range candidates {
		abs, err := resolveWorkspacePath(workspace, rel)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}
	return "", fmt.Errorf("skill script not found: %s/%s", skill, script)
}
