package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type learningState struct {
	LastRunUnix int64 `json:"lastRunUnix"`
}

var weeklyLearningMu sync.Mutex
var githubHTTP = &http.Client{Timeout: 30 * time.Second}

func StartWeeklyLearning(workspace string, p ToolPolicy) {
	enabled := os.Getenv("NIBOT_WEEKLY_LEARNING")
	run := false
	if strings.TrimSpace(enabled) != "" {
		run = parseBool(enabled, false)
	} else {
		run = os.Getenv("NIBOT_ENABLE_SKILLS") == "1"
	}
	if !run {
		return
	}
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for {
			_ = maybeRunWeeklyLearning(workspace, p)
			<-t.C
		}
	}()
}

func maybeRunWeeklyLearning(workspace string, p ToolPolicy) error {
	weeklyLearningMu.Lock()
	defer weeklyLearningMu.Unlock()

	if err := ensureLearningsFiles(workspace, p); err != nil {
		return err
	}

	st, _ := readLearningState(workspace)
	now := time.Now().Unix()
	if st.LastRunUnix > 0 {
		if time.Duration(now-st.LastRunUnix)*time.Second < 7*24*time.Hour {
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	if err := runFileOrganizerLearning(ctx, workspace, p); err != nil {
		_ = appendError(workspace, p, fmt.Sprintf("weekly learning failed: %v", err))
		return err
	}

	st.LastRunUnix = now
	_ = writeLearningState(workspace, p, st)
	return nil
}

func ensureLearningsFiles(workspace string, p ToolPolicy) error {
	if !p.AllowsWritePath(".learnings/") {
		return fmt.Errorf("policy denies learnings write")
	}
	dir, err := resolveWorkspacePath(workspace, ".learnings")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := []string{
		".learnings/ERRORS.md",
		".learnings/LEARNINGS.md",
		".learnings/FEATURE_REQUESTS.md",
	}
	for _, rel := range files {
		if !p.AllowsWritePath(rel) {
			return fmt.Errorf("policy denies write to %s", rel)
		}
		abs, err := resolveWorkspacePath(workspace, rel)
		if err != nil {
			return err
		}
		if _, err := os.Stat(abs); err == nil {
			continue
		}
		if err := os.WriteFile(abs, []byte(""), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readLearningState(workspace string) (learningState, error) {
	abs, err := resolveWorkspacePath(workspace, ".learnings/state.json")
	if err != nil {
		return learningState{}, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return learningState{}, err
	}
	var st learningState
	if err := json.Unmarshal(b, &st); err != nil {
		return learningState{}, err
	}
	return st, nil
}

func writeLearningState(workspace string, p ToolPolicy, st learningState) error {
	if !p.AllowsWritePath(".learnings/state.json") {
		return fmt.Errorf("policy denies write to .learnings/state.json")
	}
	abs, err := resolveWorkspacePath(workspace, ".learnings/state.json")
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(abs, b, 0o644)
}

func appendError(workspace string, p ToolPolicy, msg string) error {
	return appendLearningsFile(workspace, p, ".learnings/ERRORS.md", msg)
}

func appendLearning(workspace string, p ToolPolicy, msg string) error {
	return appendLearningsFile(workspace, p, ".learnings/LEARNINGS.md", msg)
}

func appendLearningsFile(workspace string, p ToolPolicy, rel string, msg string) error {
	if !p.AllowsWritePath(rel) {
		return fmt.Errorf("policy denies write to %s", rel)
	}
	abs, err := resolveWorkspacePath(workspace, rel)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	ts := time.Now().Format(time.RFC3339)
	if _, err := f.WriteString(fmt.Sprintf("\n\n## %s\n\n%s\n", ts, strings.TrimSpace(msg))); err != nil {
		return err
	}
	return nil
}

type ghSearchResult struct {
	Items []ghRepo `json:"items"`
}

type ghRepo struct {
	FullName        string `json:"full_name"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	Language        string `json:"language"`
	StargazersCount int    `json:"stargazers_count"`
	UpdatedAt       string `json:"updated_at"`
	DefaultBranch   string `json:"default_branch"`
}

type ghContent struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type ghFile struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type vetResult struct {
	OK        bool     `json:"ok"`
	RiskLevel string   `json:"riskLevel"`
	Score     int      `json:"score"`
	Signals   []string `json:"signals"`
}

func runFileOrganizerLearning(ctx context.Context, workspace string, p ToolPolicy) error {
	repos, err := githubSearchTopRepos(ctx, "file organizer in:name,description", 5)
	if err != nil {
		return err
	}

	var kept []ghRepo
	for _, r := range repos {
		vr, err := runSkillVetter(ctx, workspace, p, r.FullName)
		if err != nil {
			_ = appendError(workspace, p, fmt.Sprintf("skill-vetter failed for %s: %v", r.FullName, err))
			continue
		}
		if strings.EqualFold(vr.RiskLevel, "high") || vr.Score >= 80 {
			_ = appendLearning(workspace, p, fmt.Sprintf("排除高风险项目：%s (%s, score=%d, signals=%s)", r.HTMLURL, vr.RiskLevel, vr.Score, strings.Join(vr.Signals, "; ")))
			continue
		}
		kept = append(kept, r)
	}
	if len(kept) == 0 {
		return fmt.Errorf("no repos passed skill-vetter")
	}

	sort.Slice(kept, func(i, j int) bool { return kept[i].StargazersCount > kept[j].StargazersCount })
	if len(kept) > 5 {
		kept = kept[:5]
	}

	var b strings.Builder
	b.WriteString("学习主题：自动整理文件技能（GitHub Top 项目分析）\n\n")
	for i, r := range kept {
		entry, err := analyzeRepo(ctx, r)
		if err != nil {
			_ = appendError(workspace, p, fmt.Sprintf("analyze failed for %s: %v", r.FullName, err))
			continue
		}
		b.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, r.FullName))
		b.WriteString(entry)
		b.WriteString("\n\n")
	}
	return appendLearning(workspace, p, b.String())
}

func analyzeRepo(ctx context.Context, r ghRepo) (string, error) {
	root, err := githubListRoot(ctx, r.FullName)
	if err != nil {
		return "", err
	}

	has := func(name string) bool {
		name = strings.ToLower(name)
		for _, c := range root {
			if strings.ToLower(c.Name) == name {
				return true
			}
		}
		return false
	}

	var deps []string
	var triggers []string

	if has("go.mod") {
		deps = append(deps, "Go modules")
		txt, ok, _ := githubFetchTextFile(ctx, r.FullName, "go.mod", 120_000)
		if ok {
			if strings.Contains(txt, "fsnotify") {
				triggers = append(triggers, "文件系统监听（fsnotify）")
			}
			if strings.Contains(txt, "cobra") {
				triggers = append(triggers, "CLI（cobra）")
			}
		}
	}
	if has("package.json") {
		deps = append(deps, "Node.js")
		txt, ok, _ := githubFetchTextFile(ctx, r.FullName, "package.json", 120_000)
		if ok {
			if strings.Contains(txt, "\"chokidar\"") {
				triggers = append(triggers, "文件系统监听（chokidar）")
			}
			if strings.Contains(txt, "\"commander\"") {
				triggers = append(triggers, "CLI（commander）")
			}
		}
	}
	if has("requirements.txt") || has("pyproject.toml") {
		deps = append(deps, "Python")
		path := "requirements.txt"
		if has("pyproject.toml") {
			path = "pyproject.toml"
		}
		txt, ok, _ := githubFetchTextFile(ctx, r.FullName, path, 120_000)
		if ok {
			if strings.Contains(strings.ToLower(txt), "watchdog") {
				triggers = append(triggers, "文件系统监听（watchdog）")
			}
			if strings.Contains(strings.ToLower(txt), "click") || strings.Contains(strings.ToLower(txt), "typer") || strings.Contains(strings.ToLower(txt), "argparse") {
				triggers = append(triggers, "CLI（Python）")
			}
		}
	}
	if has("dockerfile") {
		deps = append(deps, "Docker")
	}

	skillMd, ok, err := githubFetchTextFile(ctx, r.FullName, "SKILL.md", 120_000)
	if err != nil {
		return "", err
	}

	readme, _, _ := githubFetchTextFile(ctx, r.FullName, "README.md", 200_000)
	readme = strings.TrimSpace(readme)
	if readme != "" {
		if strings.Contains(strings.ToLower(readme), "watch") || strings.Contains(strings.ToLower(readme), "monitor") {
			triggers = append(triggers, "可能支持监听/监控触发（README 提及 watch/monitor）")
		}
		if strings.Contains(strings.ToLower(readme), "dry-run") {
			triggers = append(triggers, "支持 dry-run/预览（README 提及）")
		}
	}

	triggers = uniqueStrings(triggers)
	deps = uniqueStrings(deps)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("- 仓库：%s\n", r.HTMLURL))
	if strings.TrimSpace(r.Description) != "" {
		b.WriteString(fmt.Sprintf("- 简介：%s\n", strings.TrimSpace(r.Description)))
	}
	b.WriteString(fmt.Sprintf("- Stars：%d\n", r.StargazersCount))
	if strings.TrimSpace(r.Language) != "" {
		b.WriteString(fmt.Sprintf("- 主要语言：%s\n", strings.TrimSpace(r.Language)))
	}
	if len(deps) > 0 {
		b.WriteString(fmt.Sprintf("- 依赖/生态：%s\n", strings.Join(deps, "；")))
	}
	if len(triggers) > 0 {
		b.WriteString(fmt.Sprintf("- 触发方式（推断）：%s\n", strings.Join(triggers, "；")))
	}
	if ok {
		b.WriteString("\n- SKILL.md 摘要：\n\n")
		b.WriteString(indentBlock(strings.TrimSpace(skillMd), 2))
		b.WriteString("\n")
	} else {
		names := topLevelNames(root, 25)
		if len(names) > 0 {
			b.WriteString(fmt.Sprintf("- 目录结构（根目录前 25 项）：%s\n", strings.Join(names, ", ")))
		}
	}

	return b.String(), nil
}

func uniqueStrings(in []string) []string {
	m := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := m[s]; ok {
			continue
		}
		m[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func indentBlock(s string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

func topLevelNames(root []ghContent, n int) []string {
	var names []string
	for _, c := range root {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	if n > 0 && len(names) > n {
		names = names[:n]
	}
	return names
}

func githubSearchTopRepos(ctx context.Context, q string, n int) ([]ghRepo, error) {
	if n <= 0 {
		n = 5
	}
	u := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=%d", urlQueryEscape(q), n)
	var res ghSearchResult
	if err := githubGetJSON(ctx, u, &res); err != nil {
		return nil, err
	}
	return res.Items, nil
}

func githubListRoot(ctx context.Context, fullName string) ([]ghContent, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/contents", fullName)
	var res []ghContent
	if err := githubGetJSON(ctx, u, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func githubFetchTextFile(ctx context.Context, fullName, path string, maxBytes int64) (string, bool, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", fullName, githubPathEscape(path))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", false, err
	}
	addGitHubHeaders(req)
	resp, err := githubHTTP.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", false, fmt.Errorf("github contents %s: %s: %s", path, resp.Status, strings.TrimSpace(string(b)))
	}
	var f ghFile
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return "", false, err
	}
	if strings.ToLower(strings.TrimSpace(f.Encoding)) != "base64" {
		return "", false, fmt.Errorf("unsupported encoding: %s", f.Encoding)
	}
	decoded, err := decodeBase64Std(f.Content)
	if err != nil {
		return "", false, err
	}
	if maxBytes > 0 && int64(len(decoded)) > maxBytes {
		decoded = decoded[:maxBytes]
	}
	return string(decoded), true, nil
}

func githubGetJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	addGitHubHeaders(req)
	resp, err := githubHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github api: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func addGitHubHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "NiBot-Agent")
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

func runSkillVetter(ctx context.Context, workspace string, p ToolPolicy, fullName string) (vetResult, error) {
	script := "vet.sh"
	if runtime.GOOS == "windows" {
		script = "vet.ps1"
	}
	args := skillExecArgs{
		Skill:          "skill-vetter",
		Script:         script,
		Args:           []string{fullName},
		TimeoutSeconds: 45,
	}
	raw, _ := json.Marshal(args)
	out, err := toolSkillExec(ExecContext{Workspace: workspace, Policy: p}, string(raw))
	if err != nil {
		return vetResult{}, err
	}
	out = strings.TrimSpace(out)
	if strings.HasPrefix(out, "STDOUT:") {
		out = strings.TrimSpace(strings.TrimPrefix(out, "STDOUT:"))
	}
	var vr vetResult
	if err := json.Unmarshal([]byte(out), &vr); err != nil {
		return vetResult{}, fmt.Errorf("invalid vet result JSON: %w (out=%s)", err, previewText(out, 400))
	}
	return vr, nil
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(strings.TrimSpace(s))
}

func githubPathEscape(p string) string {
	p = strings.TrimLeft(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	for i := range segs {
		segs[i] = url.PathEscape(segs[i])
	}
	return strings.Join(segs, "/")
}

func decodeBase64Std(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return []byte{}, nil
	}
	s = strings.ReplaceAll(s, " ", "")
	return base64.StdEncoding.DecodeString(s)
}
