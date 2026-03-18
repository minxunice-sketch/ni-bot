package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type LLMClient struct {
	mu               sync.RWMutex
	Config           Config
	History          []Message
	SystemMsg        string
	Workspace        string
	SessionManager   *SessionManager
	MaxToolIters     int
	LastSummary      string
	LastSummaryTitle string
	SpecMode         bool
	pendingSpecSlug  string
	pendingSpecInput string
}

type Config struct {
	Provider  string
	BaseURL   string
	APIKey    string
	ModelName string
	LogLevel  string
	Policy    ToolPolicy
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model      string       `json:"model"`
	Messages   []Message    `json:"messages"`
	Tools      []openAITool `json:"tools,omitempty"`
	ToolChoice any          `json:"tool_choice,omitempty"`
}

type openAITool struct {
	Type     string            `json:"type"`
	Function openAIFunctionDef `json:"function"`
}

type openAIFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewLLMClient(cfg Config, workspace string, systemPrompt string, sessionManager *SessionManager) *LLMClient {
	return &LLMClient{
		Config:         cfg,
		SystemMsg:      systemPrompt,
		Workspace:      workspace,
		SessionManager: sessionManager,
		History:        []Message{},
		MaxToolIters:   5,
		SpecMode:       loadSpecModeSetting(workspace),
	}
}

func (c *LLMClient) Chat(userInput string) (string, error) {
	c.History = append(c.History, Message{Role: "user", Content: redactSecrets(userInput)})

	c.mu.RLock()
	systemMsg := c.SystemMsg
	c.mu.RUnlock()

	var finalResponse string
	var err error

	for i := 0; i < c.MaxToolIters; i++ {
		// Prepare messages for this turn
		messages := []Message{{Role: "system", Content: systemMsg}}
		if auto := buildAutoRecallBlock(c.Workspace, userInput); strings.TrimSpace(auto) != "" {
			messages = append(messages, Message{Role: "system", Content: auto})
		}
		messages = append(messages, c.History...)

		var responseContent string

		// Determine provider and make call
		provider := strings.ToLower(strings.TrimSpace(c.Config.Provider))
		if provider == "" {
			provider = "openai"
		}

		// Check if we should use Mock mode
		useMock := false
		if provider != "ollama" && strings.TrimSpace(c.Config.APIKey) == "" {
			useMock = true
		}

		if useMock {
			// In mock mode, we use the last message content as input
			lastMsg := ""
			if len(c.History) > 0 {
				lastMsg = c.History[len(c.History)-1].Content
			}
			responseContent = c.mockRespond(lastMsg)
		} else {
			switch provider {
			case "openai", "deepseek", "nvidia", "nvidia_nim":
				responseContent, err = c.callOpenAICompatible(messages)
			case "ollama":
				responseContent, err = c.callOllama(messages)
			default:
				// Fallback to mock if provider unknown
				lastMsg := ""
				if len(c.History) > 0 {
					lastMsg = c.History[len(c.History)-1].Content
				}
				responseContent = fmt.Sprintf("[MOCK] Received: %s", lastMsg)
			}
		}

		if err != nil {
			return "", err
		}

		// Append assistant response
		c.History = append(c.History, Message{Role: "assistant", Content: redactSecrets(responseContent)})
		finalResponse = responseContent

		// Extract and execute tools
		calls := ExtractExecCalls(responseContent)
		if len(calls) == 0 {
			break
		}

		// Execute tools
		ctx := ExecContext{
			Workspace: c.Workspace,
			Policy:    c.Config.Policy,
		}
		// Pass nil approver for now (assumes auto-approve or trusted environment)
		results := ExecuteCalls(ctx, calls, nil)

		// Format results
		var sb strings.Builder
		sb.WriteString("TOOL_RESULTS:\n")
		for _, res := range results {
			sb.WriteString(fmt.Sprintf("- tool: %s\n", res.Tool))
			if res.OK {
				// Indent output for YAML-like readability
				indented := strings.ReplaceAll(res.Output, "\n", "\n    ")
				sb.WriteString(fmt.Sprintf("  status: ok\n  output: |\n    %s\n", indented))
			} else {
				sb.WriteString(fmt.Sprintf("  status: error\n  error: %s\n", res.Error))
			}
		}
		toolOutput := sb.String()

		// Append tool output as user message for next iteration
		c.History = append(c.History, Message{Role: "user", Content: toolOutput})
	}

	return finalResponse, nil
}

func (c *LLMClient) ChatOnce(userInput string) (string, error) {
	c.History = append(c.History, Message{Role: "user", Content: redactSecrets(userInput)})

	c.mu.RLock()
	systemMsg := c.SystemMsg
	c.mu.RUnlock()

	messages := []Message{{Role: "system", Content: systemMsg}}
	if auto := buildAutoRecallBlock(c.Workspace, userInput); strings.TrimSpace(auto) != "" {
		messages = append(messages, Message{Role: "system", Content: auto})
	}
	messages = append(messages, c.History...)

	provider := strings.ToLower(strings.TrimSpace(c.Config.Provider))
	if provider == "" {
		provider = "openai"
	}

	useMock := false
	if provider != "ollama" && strings.TrimSpace(c.Config.APIKey) == "" {
		useMock = true
	}

	var responseContent string
	var err error
	if useMock {
		lastMsg := ""
		if len(c.History) > 0 {
			lastMsg = c.History[len(c.History)-1].Content
		}
		responseContent = c.mockRespond(lastMsg)
	} else {
		switch provider {
		case "openai", "deepseek", "nvidia", "nvidia_nim":
			responseContent, err = c.callOpenAICompatible(messages)
		case "ollama":
			responseContent, err = c.callOllama(messages)
		default:
			lastMsg := ""
			if len(c.History) > 0 {
				lastMsg = c.History[len(c.History)-1].Content
			}
			responseContent = fmt.Sprintf("[MOCK] Received: %s", lastMsg)
		}
	}
	if err != nil {
		return "", err
	}

	c.History = append(c.History, Message{Role: "assistant", Content: redactSecrets(responseContent)})
	return responseContent, nil
}

func (c *LLMClient) Call(messages []Message) (string, error) {
	var responseContent string
	var err error

	provider := strings.ToLower(strings.TrimSpace(c.Config.Provider))
	if provider == "" {
		provider = "openai"
	}
	if provider != "ollama" && strings.TrimSpace(c.Config.APIKey) == "" {
		last := ""
		if len(messages) > 0 {
			last = messages[len(messages)-1].Content
		}
		return c.mockRespond(last), nil
	}

	switch provider {
	case "openai", "deepseek", "nvidia", "nvidia_nim":
		responseContent, err = c.callOpenAICompatible(messages)
	case "ollama":
		responseContent, err = c.callOllama(messages)
	default:
		last := ""
		if len(messages) > 0 {
			last = messages[len(messages)-1].Content
		}
		responseContent = fmt.Sprintf("[MOCK] Received: %s", last)
	}
	if err != nil {
		return "", err
	}
	return responseContent, nil
}

func buildAutoRecallBlock(workspace string, userInput string) string {
	if !autoRecallEnabled() {
		return ""
	}
	in := strings.TrimSpace(userInput)
	if in == "" {
		return ""
	}
	if strings.HasPrefix(in, "TOOL_RESULTS:") {
		return ""
	}

	scope := strings.TrimSpace(os.Getenv("NIBOT_AUTO_RECALL_SCOPE"))
	if scope == "" {
		scope = "global"
	}

	limit := autoRecallLimit()
	maxBytes := autoRecallMaxBytes()
	terms := extractRecallTerms(redactSecrets(in), 20)
	if len(terms) == 0 {
		return ""
	}

	var sb strings.Builder
	wroteAny := false

	memTerms := terms
	if len(memTerms) > 6 {
		memTerms = memTerms[:6]
	}
	if memBlock := buildAutoRecallMemoriesBlock(workspace, scope, memTerms, limit, maxBytes); strings.TrimSpace(memBlock) != "" {
		sb.WriteString(memBlock)
		wroteAny = true
	}

	remaining := maxBytes
	if remaining > 0 {
		remaining = maxBytes - len([]byte(sb.String()))
		if remaining < 0 {
			remaining = 0
		}
	}

	if autoRecallLearningsEnabled(workspace) && (remaining == 0 || remaining >= 200) {
		lb := autoRecallLearningsMaxBytes(maxBytes, remaining)
		if learnBlock := buildAutoRecallLearningsBlock(workspace, terms, lb); strings.TrimSpace(learnBlock) != "" {
			if wroteAny {
				sb.WriteString("\n")
			}
			sb.WriteString(learnBlock)
			wroteAny = true
		}
	}

	if !wroteAny {
		return ""
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildAutoRecallMemoriesBlock(workspace string, scope string, terms []string, limit int, maxBytes int) string {
	s, err := OpenSQLiteStore(workspace)
	if err != nil || s == nil {
		return ""
	}
	defer s.Close()

	byID := map[int64]MemoryItem{}
	perTerm := 5
	if limit > 0 && limit < perTerm {
		perTerm = limit
	}
	for _, term := range terms {
		items, err := s.SearchMemories(scope, term, perTerm)
		if err != nil {
			continue
		}
		for _, it := range items {
			byID[it.ID] = it
		}
	}
	if len(byID) == 0 {
		return ""
	}

	var merged []MemoryItem
	for _, it := range byID {
		merged = append(merged, it)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ID > merged[j].ID })
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}

	var sb strings.Builder
	sb.WriteString("=== RELEVANT MEMORIES ===\n")
	for _, it := range merged {
		line := fmt.Sprintf("- id=%d scope=%s tags=%s: %s\n", it.ID, it.Scope, it.Tags, previewText(redactSecrets(it.Content), 240))
		if maxBytes > 0 && sb.Len()+len([]byte(line)) > maxBytes {
			break
		}
		sb.WriteString(line)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func autoRecallLearningsEnabled(workspace string) bool {
	if v, ok := os.LookupEnv("NIBOT_AUTO_RECALL_LEARNINGS"); ok && strings.TrimSpace(v) != "" {
		return parseBool(v, false)
	}
	abs := filepath.Join(workspace, ".learnings")
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		return true
	}
	return false
}

func autoRecallLearningsMaxBytes(totalMax int, remaining int) int {
	if v := strings.TrimSpace(os.Getenv("NIBOT_AUTO_RECALL_LEARNINGS_MAX_BYTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if remaining > 0 && n > remaining {
				n = remaining
			}
			if totalMax > 0 && n > totalMax {
				n = totalMax
			}
			if n < 200 {
				n = 200
			}
			if n > 4000 {
				n = 4000
			}
			return n
		}
	}
	n := totalMax / 2
	if n < 200 {
		n = 200
	}
	if n > 1200 {
		n = 1200
	}
	if remaining > 0 && n > remaining {
		n = remaining
	}
	if totalMax > 0 && n > totalMax {
		n = totalMax
	}
	return n
}

func buildAutoRecallLearningsBlock(workspace string, terms []string, maxBytes int) string {
	files := []string{
		filepath.Join(workspace, ".learnings", "LEARNINGS.md"),
		filepath.Join(workspace, ".learnings", "ERRORS.md"),
		filepath.Join(workspace, ".learnings", "FEATURE_REQUESTS.md"),
	}

	type cand struct {
		score int
		file  string
		text  string
	}
	var cands []cand
	for _, abs := range files {
		b, err := readFileTail(abs, 256*1024)
		if err != nil || len(b) == 0 {
			continue
		}
		txt := strings.ReplaceAll(string(b), "\r\n", "\n")
		sections := extractMarkdownSections(txt)
		for _, sec := range sections {
			s := sectionScore(sec, terms)
			if s <= 0 {
				continue
			}
			cands = append(cands, cand{score: s, file: filepath.Base(abs), text: sec})
		}
		if len(cands) == 0 {
			s := sectionScore(txt, terms)
			if s > 0 {
				cands = append(cands, cand{score: s, file: filepath.Base(abs), text: txt})
			}
		}
	}

	if len(cands) == 0 {
		return ""
	}

	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return len([]rune(cands[i].text)) > len([]rune(cands[j].text))
	})

	var sb strings.Builder
	sb.WriteString("=== RELEVANT LEARNINGS ===\n")
	seen := map[string]struct{}{}
	for _, it := range cands {
		key := it.file + ":" + previewText(it.text, 80)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		line := fmt.Sprintf("- file=%s score=%d: %s\n", it.file, it.score, previewText(it.text, 360))
		if maxBytes > 0 && sb.Len()+len([]byte(line)) > maxBytes {
			break
		}
		sb.WriteString(line)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func extractMarkdownSections(s string) []string {
	lines := strings.Split(s, "\n")
	var sections []string
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		sec := strings.TrimSpace(strings.Join(cur, "\n"))
		cur = nil
		if sec != "" {
			sections = append(sections, sec)
		}
	}
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## ") {
			flush()
		}
		cur = append(cur, ln)
	}
	flush()
	return sections
}

func sectionScore(s string, terms []string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	score := 0
	for _, t := range terms {
		tt := strings.TrimSpace(t)
		if tt == "" {
			continue
		}
		n := strings.Count(low, strings.ToLower(tt))
		score += n
	}
	return score
}

func readFileTail(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size <= 0 {
		return nil, nil
	}
	if size <= maxBytes {
		return io.ReadAll(f)
	}
	_, err = f.Seek(-maxBytes, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func autoRecallEnabled() bool {
	v := strings.TrimSpace(os.Getenv("NIBOT_AUTO_RECALL"))
	if v == "" {
		return true
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func autoRecallLimit() int {
	v := strings.TrimSpace(os.Getenv("NIBOT_AUTO_RECALL_LIMIT"))
	if v == "" {
		return 6
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 6
	}
	if n > 20 {
		n = 20
	}
	return n
}

func autoRecallMaxBytes() int {
	v := strings.TrimSpace(os.Getenv("NIBOT_AUTO_RECALL_MAX_BYTES"))
	if v == "" {
		return 1200
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 1200
	}
	if n < 200 {
		n = 200
	}
	if n > 8000 {
		n = 8000
	}
	return n
}

func extractRecallTerms(s string, maxTerms int) []string {
	if maxTerms <= 0 {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		t := strings.TrimSpace(b.String())
		b.Reset()
		if len([]rune(t)) < 2 {
			return
		}
		tokens = append(tokens, t)
	}

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	if len(tokens) == 0 {
		return nil
	}

	var expanded []string
	expanded = append(expanded, tokens...)
	for _, t := range tokens {
		if !containsHanRunes(t) {
			continue
		}
		rs := []rune(t)
		if len(rs) <= 8 {
			continue
		}
		win := 6
		if len(rs) < win {
			win = len(rs)
		}
		added := 0
		stride := 2
		if len(rs) <= 12 {
			stride = 1
		}
		for i := 0; i+win <= len(rs) && added < 12; i += stride {
			sub := strings.TrimSpace(string(rs[i : i+win]))
			if len([]rune(sub)) >= 2 {
				expanded = append(expanded, sub)
				added++
			}
		}
	}

	seen := map[string]struct{}{}
	var uniq []string
	for _, t := range expanded {
		low := strings.ToLower(t)
		if _, ok := seen[low]; ok {
			continue
		}
		seen[low] = struct{}{}
		uniq = append(uniq, t)
	}
	sort.Slice(uniq, func(i, j int) bool { return len([]rune(uniq[i])) > len([]rune(uniq[j])) })
	if len(uniq) > maxTerms {
		uniq = uniq[:maxTerms]
	}
	return uniq
}

func containsHanRunes(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han) {
			return true
		}
	}
	return false
}

func (c *LLMClient) mockRespond(userInput string) string {
	in := strings.TrimSpace(userInput)
	low := strings.ToLower(in)

	if strings.HasPrefix(in, "TOOL_RESULTS:") {
		tool := extractFirstToolName(in)
		out := extractFirstToolOutput(in)
		if tool == "" {
			return "已收到工具结果，但无法识别工具名。请重试或把 Tool Results 原样粘贴出来。"
		}
		if out == "" {
			return fmt.Sprintf("已收到工具结果（%s），但没有 output 内容。你希望我基于 error 信息做什么？", tool)
		}

		switch tool {
		case "fs.read":
			if strings.Contains(out, "Ni bot 事实记忆库") {
				summary := "要点总结：\n- facts.md 用于长期、稳定、可验证的事实与约定\n- 记忆中禁止写入密钥/Token/隐私信息\n- 每条事实应包含来源或验证方式，避免记忆污染\n- 反思应写成可执行的“触发条件→错误模式→修复策略→验证方法”\n- 下次任务开始前先检索 facts/reflections 并引用条目编号"
				c.LastSummary = summary
				c.LastSummaryTitle = "Facts 总结"
				return "已读取工具输出。\n\n" + summary + "\n\n如需落盘，我可以写入 memory/notes.md。"
			}
			c.LastSummary = "读取结果（节选）：\n" + headLines(out, 20)
			c.LastSummaryTitle = "读取结果"
			return "已读取工具输出。\n\n" + c.LastSummary + "\n\n如需落盘，我可以写入 memory/notes.md。"
		case "skill.exec":
			compact := strings.TrimSpace(out)
			compact = headLines(compact, 12)
			summary := "技能输出（节选）：\n" + compact
			c.LastSummary = summary
			c.LastSummaryTitle = "技能输出"
			return "已收到技能执行输出。\n\n" + summary + "\n\n如需落盘，我可以写入 memory/notes.md。"
		default:
			c.LastSummary = "工具输出（节选）：\n" + headLines(out, 20)
			c.LastSummaryTitle = "工具输出"
			return "已收到工具输出。\n\n" + c.LastSummary + "\n\n如需落盘，我可以写入 memory/notes.md。"
		}
	}

	if strings.Contains(low, "天气") || strings.Contains(low, "weather") {
		skill := "weather"
		script := "weather.ps1"
		if fileExists(filepath.Join(c.Workspace, "skills", skill, "scripts", script)) {
			city := extractCityFromWeatherQuery(in)
			if city == "" {
				city = "Beijing"
			}
			args, _ := json.Marshal(skillExecArgs{Skill: skill, Script: script, Args: []string{city}, TimeoutSeconds: 30})
			return fmt.Sprintf("[EXEC:skill.exec %s]", string(args))
		}
		return "未找到天气技能脚本。请先创建 workspace/skills/weather/scripts/weather.ps1，然后再试一次。"
	}

	if strings.Contains(low, "写入") || strings.Contains(low, "落盘") || strings.Contains(low, "保存") {
		target := "memory/notes.md"
		if strings.Contains(low, "reflections") || strings.Contains(low, "反思") {
			target = "memory/reflections.md"
		}
		if strings.Contains(low, "notes.md") || strings.Contains(low, "notes") || strings.Contains(low, "笔记") || strings.Contains(low, "总结") {
			target = "memory/notes.md"
		}

		if c.LastSummary == "" {
			return "我还没有可落盘的总结内容。请先让我读取并总结一次 facts.md，然后再让我写入 memory/notes.md。"
		}

		title := c.LastSummaryTitle
		if title == "" {
			title = "Notes"
		}
		entry := formatNotesEntry(title, c.LastSummary)
		args, _ := json.Marshal(fsWriteArgs{Path: target, Content: entry, Mode: "append"})
		return fmt.Sprintf("[EXEC:fs.write %s]", string(args))
	}

	if strings.Contains(low, "读取") || strings.Contains(low, "read") {
		if strings.Contains(low, "reflections") || strings.Contains(low, "反思") {
			return `[EXEC:fs.read {"path":"memory/reflections.md"}]`
		}
		return `[EXEC:fs.read {"path":"memory/facts.md"}]`
	}

	return "当前未配置 API Key，因此处于 Mock 模式。\n\n你可以输入：\n- 读取 facts.md 并总结\n- 读取 reflections.md 并总结\n- 查询 Beijing 天气（需启用 NIBOT_ENABLE_SKILLS=1）"
}

func extractFirstToolOutput(toolSummary string) string {
	lines := strings.Split(toolSummary, "\n")
	inOutput := false
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(line, "  output: |") {
			inOutput = true
			continue
		}
		if !inOutput {
			continue
		}
		if strings.HasPrefix(line, "    ") {
			out = append(out, strings.TrimPrefix(line, "    "))
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func extractFirstToolName(toolSummary string) string {
	for _, line := range strings.Split(toolSummary, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- tool:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "- tool:"))
		}
	}
	return ""
}

func headLines(s string, max int) string {
	if max <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"), "\n")
	if len(lines) <= max {
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return strings.TrimSpace(strings.Join(lines[:max], "\n")) + "\n[TRUNCATED]"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func formatNotesEntry(title, body string) string {
	ts := time.Now().Format("2006-01-02 15:04:05")
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" {
		title = "Notes"
	}
	if body == "" {
		body = "(empty)"
	}
	return fmt.Sprintf("## %s - %s\n\n%s\n", ts, title, body)
}

func extractCityFromWeatherQuery(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}

	if idx := strings.Index(s, "天气"); idx >= 0 {
		city := strings.TrimSpace(s[:idx])
		city = strings.TrimSpace(strings.TrimPrefix(city, "查询"))
		city = strings.TrimSpace(strings.TrimPrefix(city, "查"))
		city = strings.TrimSpace(strings.TrimSuffix(city, "的"))
		city = strings.TrimSpace(strings.Trim(city, "：:，,？?。.! "))
		city = strings.Join(strings.Fields(city), " ")
		return city
	}

	low := strings.ToLower(s)
	if idx := strings.Index(low, "weather"); idx >= 0 {
		after := strings.TrimSpace(s[idx+len("weather"):])
		afterLow := strings.ToLower(after)
		afterLow = strings.TrimSpace(afterLow)

		if strings.HasPrefix(afterLow, "in ") {
			after = strings.TrimSpace(after[2:])
		} else if strings.HasPrefix(afterLow, "for ") {
			after = strings.TrimSpace(after[3:])
		}

		after = strings.TrimSpace(strings.Trim(after, " :,-"))
		after = strings.TrimRightFunc(after, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r)
		})
		after = strings.Join(strings.Fields(after), " ")
		if after != "" {
			return after
		}

		before := strings.TrimSpace(s[:idx])
		before = strings.Join(strings.Fields(before), " ")
		return before
	}

	return ""
}

func (c *LLMClient) callOpenAICompatible(messages []Message) (string, error) {
	reqBody := openAIRequest{
		Model:    c.Config.ModelName,
		Messages: messages,
	}
	if nativeToolsEnabled() {
		reqBody.Tools = c.openAIToolsForPolicy()
		reqBody.ToolChoice = "auto"
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", c.Config.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error: %s", string(body))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", err
	}
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	if nativeToolsEnabled() && len(openAIResp.Choices[0].Message.ToolCalls) > 0 {
		return translateToolCallsToExec(openAIResp.Choices[0].Message.ToolCalls), nil
	}
	return openAIResp.Choices[0].Message.Content, nil
}

func nativeToolsEnabled() bool {
	v := strings.TrimSpace(os.Getenv("NIBOT_ENABLE_NATIVE_TOOLS"))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func translateToolCallsToExec(calls []openAIToolCall) string {
	var lines []string
	for _, tc := range calls {
		name := strings.TrimSpace(tc.Function.Name)
		if name == "" {
			continue
		}
		args := strings.TrimSpace(tc.Function.Arguments)
		if args == "" {
			lines = append(lines, fmt.Sprintf("[EXEC:%s]", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("[EXEC:%s %s]", name, args))
	}
	return strings.Join(lines, "\n")
}

func (c *LLMClient) openAIToolsForPolicy() []openAITool {
	p := c.Config.Policy
	if !p.Loaded {
		p = DefaultToolPolicy()
	}

	tools := []openAITool{
		{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "file_read",
				Description: "Read a file from Ni bot workspace",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
					"required": []string{"path"},
				},
			},
		},
	}

	if p.AllowsTool("file_write") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "file_write",
				Description: "Write a file under Ni bot workspace (append or overwrite subject to policy)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
						"mode":    map[string]any{"type": "string", "enum": []string{"append", "overwrite"}},
					},
					"required": []string{"path", "content"},
				},
			},
		})
	}

	if p.AllowsTool("install_skill") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "install_skill",
				Description: "Install skills from a https:// git repository into Ni bot workspace skills directory",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":  map[string]any{"type": "string"},
						"url":   map[string]any{"type": "string"},
						"layer": map[string]any{"type": "string", "enum": []string{"upstream", "local"}},
					},
					"required": []string{"name", "url"},
				},
			},
		})
	}

	if p.AllowsTool("memory.store") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "memory.store",
				Description: "Store a long-term memory item (SQLite memory DB must be enabled)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"scope":   map[string]any{"type": "string"},
						"tags":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
					},
					"required": []string{"content"},
				},
			},
		})
	}
	if p.AllowsTool("memory.recall") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "memory.recall",
				Description: "Search long-term memories by keyword match (SQLite memory DB must be enabled)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"scope": map[string]any{"type": "string"},
						"query": map[string]any{"type": "string"},
						"limit": map[string]any{"type": "integer"},
					},
					"required": []string{"query"},
				},
			},
		})
	}
	if p.AllowsTool("memory.forget") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "memory.forget",
				Description: "Delete a memory item by id (SQLite memory DB must be enabled)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "integer"},
					},
					"required": []string{"id"},
				},
			},
		})
	}
	if p.AllowsTool("memory.list") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "memory.list",
				Description: "List recent memory items (SQLite memory DB must be enabled)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"scope": map[string]any{"type": "string"},
						"limit": map[string]any{"type": "integer"},
					},
				},
			},
		})
	}
	if p.AllowsTool("memory.stats") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "memory.stats",
				Description: "Show memory database stats (SQLite memory DB must be enabled)",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		})
	}
	if p.AllowsTool("memory.import") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "memory.import",
				Description: "Import memory items from a pasted text block (SQLite memory DB must be enabled)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"source": map[string]any{"type": "string"},
						"scope":  map[string]any{"type": "string"},
						"tags":   map[string]any{"type": "string"},
						"text":   map[string]any{"type": "string"},
						"limit":  map[string]any{"type": "integer"},
					},
					"required": []string{"text"},
				},
			},
		})
	}

	if p.AllowsTool("shell_exec") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "shell_exec",
				Description: "Execute a shell command with approval and sandbox/policy restrictions",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command":        map[string]any{"type": "string"},
						"timeoutSeconds": map[string]any{"type": "integer"},
					},
					"required": []string{"command"},
				},
			},
		})
	}

	if p.AllowsTool("skill_exec") {
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        "skill_exec",
				Description: "Execute a skill script with approval and sandbox/policy restrictions",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill":          map[string]any{"type": "string"},
						"script":         map[string]any{"type": "string"},
						"args":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"timeoutSeconds": map[string]any{"type": "integer"},
					},
					"required": []string{"skill", "script"},
				},
			},
		})
	}

	return tools
}

func (c *LLMClient) callOllama(messages []Message) (string, error) {
	return c.callOpenAICompatible(messages)
}

func (c *LLMClient) Loop(inputReader io.Reader, outputWriter io.Writer, logger *os.File) {
	scanner := bufio.NewScanner(inputReader)
	stopAutoReload := c.StartAutoReload(logger)
	defer stopAutoReload()
	defer c.persistSessionOnExit()

	v := strings.TrimSpace(os.Getenv("NIBOT_VERSION"))
	if v == "" {
		v = "dev"
	}
	fmt.Fprintf(outputWriter, "\n> Ni bot initialized (%s). Type your request (or 'exit' to quit):\n", v)
	fmt.Fprint(outputWriter, "> ")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if c.SpecMode && strings.TrimSpace(c.pendingSpecSlug) != "" && isSpecConfirmInput(input) {
			slug := strings.TrimSpace(c.pendingSpecSlug)
			req := strings.TrimSpace(c.pendingSpecInput)
			c.pendingSpecSlug = ""
			c.pendingSpecInput = ""
			input = fmt.Sprintf("用户已确认Spec，开始实施。\n\n需求：%s\n\n请先读取并遵循以下文件：\n- workspace/specs/%s/spec.md\n- workspace/specs/%s/tasks.md\n- workspace/specs/%s/checklist.md\n\n要求：先按 tasks.md 逐项执行，必要时调用工具；所有写入/执行仍需遵循 policy + 审批。", req, slug, slug, slug)
		}
		tokens := splitCommandLine(input)
		if len(tokens) == 0 {
			fmt.Fprint(outputWriter, "> ")
			continue
		}
		cmd := strings.ToLower(tokens[0])
		switch cmd {
		case "exit", "quit":
			return
		case "version", "/version":
			fmt.Fprintf(outputWriter, "\n%s\n", v)
			fmt.Fprint(outputWriter, "\n> ")
			continue
		case "update", "/update":
			yes := false
			for _, t := range tokens[1:] {
				tt := strings.ToLower(strings.TrimSpace(t))
				if tt == "-y" || tt == "--yes" {
					yes = true
				}
			}
			if !yes && stdinIsTerminal() {
				fmt.Fprint(outputWriter, "\n将执行 git pull + go mod tidy + go build（不会覆盖 workspace 数据）。继续？(y/n): ")
				if !scanner.Scan() {
					fmt.Fprint(outputWriter, "\n> ")
					continue
				}
				ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if ans != "y" && ans != "yes" {
					fmt.Fprint(outputWriter, "\n已取消。\n\n> ")
					continue
				}
			} else if !yes {
				fmt.Fprint(outputWriter, "\n用法：update --yes\n\n> ")
				continue
			}
			if err := runSelfUpdate(outputWriter); err != nil {
				fmt.Fprintf(outputWriter, "\n更新失败：%v\n\n> ", err)
				continue
			}
			fmt.Fprint(outputWriter, "\n✅ 更新完成。\n\n> ")
			continue
		case "reload", "/reload":
			systemPrompt, err := ConstructSystemPrompt(c.Workspace)
			if err != nil {
				fmt.Fprintf(outputWriter, "\n重新加载失败：%v\n", err)
				fmt.Fprint(outputWriter, "\n> ")
				continue
			}
			c.mu.Lock()
			c.SystemMsg = systemPrompt
			c.Config.Policy = LoadToolPolicy(c.Workspace)
			c.mu.Unlock()
			fmt.Fprintln(outputWriter, "\n已重新加载 System Prompt（包含最新 skills/memory）。")
			if normalizeLogLevel(c.Config.LogLevel) == "meta" {
				writeLog(logger, fmt.Sprintf("\n## System Prompt Reloaded\n\n(prompt_bytes=%d)\n\n---\n", len([]byte(systemPrompt))))
			} else {
				writeLog(logger, "\n## System Prompt Reloaded\n\n```\n"+RedactForLog(systemPrompt)+"\n```\n\n---\n")
			}
			fmt.Fprint(outputWriter, "\n> ")
			continue
		case "spec", "/spec":
			sub := "status"
			if len(tokens) >= 2 {
				sub = strings.ToLower(strings.TrimSpace(tokens[1]))
			}
			switch sub {
			case "on", "enable":
				c.SpecMode = true
				if hasToken(tokens, "--persist") {
					_ = saveSpecModeSetting(c.Workspace, true)
				}
				fmt.Fprintln(outputWriter, "\nSpec 模式：ON")
			case "off", "disable":
				c.SpecMode = false
				c.pendingSpecSlug = ""
				c.pendingSpecInput = ""
				if hasToken(tokens, "--persist") {
					_ = saveSpecModeSetting(c.Workspace, false)
				}
				fmt.Fprintln(outputWriter, "\nSpec 模式：OFF")
			case "persist":
				val := ""
				if len(tokens) >= 3 {
					val = strings.ToLower(strings.TrimSpace(tokens[2]))
				}
				if val == "on" || val == "true" || val == "1" || val == "yes" {
					_ = saveSpecModeSetting(c.Workspace, true)
					fmt.Fprintln(outputWriter, "\n已持久化：Spec 模式默认 ON")
				} else if val == "off" || val == "false" || val == "0" || val == "no" {
					_ = saveSpecModeSetting(c.Workspace, false)
					fmt.Fprintln(outputWriter, "\n已持久化：Spec 模式默认 OFF")
				} else {
					fmt.Fprintln(outputWriter, "\n用法：spec persist on|off")
				}
			case "status":
				persisted := loadSpecModeSetting(c.Workspace)
				fmt.Fprintf(outputWriter, "\nSpec 模式：%v（持久化默认：%v）\n", c.SpecMode, persisted)
				if strings.TrimSpace(c.pendingSpecSlug) != "" {
					fmt.Fprintf(outputWriter, "待确认Spec：workspace/specs/%s/\n", strings.TrimSpace(c.pendingSpecSlug))
				}
			default:
				fmt.Fprintln(outputWriter, "\n用法：")
				fmt.Fprintln(outputWriter, "- spec status")
				fmt.Fprintln(outputWriter, "- spec on [--persist]")
				fmt.Fprintln(outputWriter, "- spec off [--persist]")
				fmt.Fprintln(outputWriter, "- spec persist on|off")
			}
			fmt.Fprint(outputWriter, "\n> ")
			continue
		case "skills", "/skills":
			if len(tokens) >= 2 {
				sub := strings.ToLower(tokens[1])
				if sub == "show" {
					if len(tokens) < 3 {
						fmt.Fprintln(outputWriter, "\n用法：skills show <name>")
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					name := tokens[2]
					skills, err := DiscoverSkills(c.Workspace)
					if err != nil {
						fmt.Fprintf(outputWriter, "\n读取技能失败：%v\n", err)
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					var found *Skill
					for i := range skills {
						if strings.EqualFold(skills[i].Name, name) || strings.EqualFold(skills[i].DisplayName, name) {
							found = &skills[i]
							break
						}
					}
					if found == nil {
						fmt.Fprintln(outputWriter, "\n未找到技能："+name)
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					fmt.Fprintf(outputWriter, "\nSkill: %s\n", found.Name)
					if strings.TrimSpace(found.Description) != "" {
						fmt.Fprintf(outputWriter, "Description: %s\n", strings.TrimSpace(found.Description))
					}
					if strings.TrimSpace(found.Source) != "" {
						fmt.Fprintf(outputWriter, "Source: %s\n", strings.TrimSpace(found.Source))
					}
					if strings.TrimSpace(found.Docs) != "" {
						fmt.Fprintln(outputWriter, "\nDocs:")
						fmt.Fprintln(outputWriter, headLines(found.Docs, 40))
					}
					if len(found.Scripts) > 0 {
						fmt.Fprintln(outputWriter, "\nScripts:")
						for _, sc := range found.Scripts {
							fmt.Fprintf(outputWriter, "- %s\n", sc)
							fmt.Fprintf(outputWriter, "  call: [EXEC:skill.exec {\"skill\":\"%s\",\"script\":\"%s\",\"args\":[],\"timeoutSeconds\":30}]\n", found.Name, sc)
						}
					} else {
						fmt.Fprintln(outputWriter, "\nScripts: (none)")
					}
					fmt.Fprint(outputWriter, "\n> ")
					continue
				}
				if sub == "search" || sub == "find" {
					if len(tokens) < 3 {
						fmt.Fprintln(outputWriter, "\n用法：skills search <keyword>")
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					kw := strings.ToLower(strings.TrimSpace(tokens[2]))
					skills, err := DiscoverSkills(c.Workspace)
					if err != nil {
						fmt.Fprintf(outputWriter, "\n读取技能失败：%v\n", err)
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					var hits []Skill
					for _, s := range skills {
						hay := strings.ToLower(s.Name + "\n" + s.DisplayName + "\n" + s.Description + "\n" + headLines(s.Docs, 20))
						if strings.Contains(hay, kw) {
							hits = append(hits, s)
						}
					}
					if len(hits) == 0 {
						fmt.Fprintln(outputWriter, "\n未找到匹配技能。")
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					fmt.Fprintln(outputWriter, "\nSkills:")
					for _, s := range hits {
						line := "- " + s.Name
						if strings.TrimSpace(s.Description) != "" {
							line += " — " + strings.TrimSpace(s.Description)
						}
						fmt.Fprintln(outputWriter, line)
					}
					fmt.Fprint(outputWriter, "\n> ")
					continue
				}
				if sub == "install" || sub == "add" {
					if len(tokens) < 3 {
						fmt.Fprintln(outputWriter, "\n用法：")
						fmt.Fprintln(outputWriter, "- skills install <path|path.zip>")
						fmt.Fprintln(outputWriter, "- skills install git <https-url>")
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					var installed []string
					var err error
					if strings.EqualFold(tokens[2], "git") {
						if len(tokens) < 4 {
							fmt.Fprintln(outputWriter, "\n用法：skills install git <https-url>")
							fmt.Fprint(outputWriter, "\n> ")
							continue
						}
						installed, err = InstallSkillsFromGitURL(c.Workspace, tokens[3])
					} else {
						installed, err = InstallSkillsFromPath(c.Workspace, tokens[2])
					}
					if err != nil {
						fmt.Fprintf(outputWriter, "\n安装失败：%v\n", err)
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					fmt.Fprintln(outputWriter, "\n已安装技能：")
					for _, n := range installed {
						fmt.Fprintf(outputWriter, "- %s\n", n)
					}
					fmt.Fprintln(outputWriter, "\n提示：导入后可执行 reload 让模型立即加载新 skills/memory（无需重启）。")
					fmt.Fprint(outputWriter, "\n> ")
					continue
				}
				if sub == "doctor" || sub == "check" {
					issues, err := DiagnoseSkills(c.Workspace)
					if err != nil {
						fmt.Fprintf(outputWriter, "\n检查失败：%v\n", err)
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					if len(issues) == 0 {
						fmt.Fprintln(outputWriter, "\nSkills OK")
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					fmt.Fprintln(outputWriter, "\nSkills Doctor:")
					for _, it := range issues {
						fmt.Fprintf(outputWriter, "- [%s] %s: %s\n", it.Level, it.Skill, it.Message)
					}
					fmt.Fprint(outputWriter, "\n> ")
					continue
				}
				if sub == "test" {
					if len(tokens) < 3 {
						fmt.Fprintln(outputWriter, "\n用法：skills test <name>")
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					issues, err := CheckSkill(c.Workspace, tokens[2])
					if err != nil {
						fmt.Fprintf(outputWriter, "\n检查失败：%v\n", err)
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					if len(issues) == 0 {
						fmt.Fprintln(outputWriter, "\nSkill OK: "+tokens[2])
						fmt.Fprint(outputWriter, "\n> ")
						continue
					}
					fmt.Fprintln(outputWriter, "\nSkills Test:")
					for _, it := range issues {
						fmt.Fprintf(outputWriter, "- [%s] %s: %s\n", it.Level, it.Skill, it.Message)
					}
					fmt.Fprint(outputWriter, "\n> ")
					continue
				}
			}

			skills, err := DiscoverSkills(c.Workspace)
			if err != nil {
				fmt.Fprintf(outputWriter, "\n读取技能列表失败：%v\n", err)
				fmt.Fprint(outputWriter, "\n> ")
				continue
			}
			if len(skills) == 0 {
				fmt.Fprintln(outputWriter, "\n当前未发现技能（workspace/skills/ 下为空或不可读）。")
				fmt.Fprint(outputWriter, "\n> ")
				continue
			}
			fmt.Fprintln(outputWriter, "\nSkills:")
			for _, s := range skills {
				line := "- " + s.Name
				if strings.TrimSpace(s.Description) != "" {
					line += " — " + strings.TrimSpace(s.Description)
				}
				fmt.Fprintln(outputWriter, line)
				for _, sc := range s.Scripts {
					args := `[]`
					if strings.EqualFold(s.Name, "weather") {
						args = `["Beijing"]`
					}
					fmt.Fprintf(outputWriter, "  - %s (call: [EXEC:skill.exec {\"skill\":\"%s\",\"script\":\"%s\",\"args\":%s,\"timeoutSeconds\":30}])\n", sc, s.Name, sc, args)
				}
			}
			fmt.Fprint(outputWriter, "\n> ")
			continue
		case "help", "/help", "?":
			fmt.Fprintln(outputWriter, "\nCommands:")
			fmt.Fprintln(outputWriter, "- help / /help / ?: show this help")
			fmt.Fprintln(outputWriter, "- version / /version: print version")
			fmt.Fprintln(outputWriter, "- skills / /skills: list skills")
			fmt.Fprintln(outputWriter, "- skills show <name>: show skill docs and scripts")
			fmt.Fprintln(outputWriter, "- skills search <kw>: search skills by keyword")
			fmt.Fprintln(outputWriter, "- skills install <path>: install skills from a local folder")
			fmt.Fprintln(outputWriter, "- skills doctor: validate installed skills")
			fmt.Fprintln(outputWriter, "- skills test <name>: test a skill without executing")
			fmt.Fprintln(outputWriter, "- reload / /reload: reload system prompt (skills/memory)")
			fmt.Fprintln(outputWriter, "- spec / /spec: spec mode (generate spec/tasks/checklist)")
			fmt.Fprintln(outputWriter, "- update / /update: git pull + go mod tidy + go build (use: update --yes)")
			fmt.Fprintln(outputWriter, "- clear / /clear: clear the screen")
			fmt.Fprintln(outputWriter, "- reset / /reset: clear conversation memory (history)")
			fmt.Fprintln(outputWriter, "- exit / quit: exit Ni bot")
			fmt.Fprint(outputWriter, "\n> ")
			continue
		case "clear", "/clear":
			for i := 0; i < 40; i++ {
				fmt.Fprintln(outputWriter)
			}
			fmt.Fprintln(outputWriter, "> Ni bot initialized. Type your request (or 'exit' to quit):")
			fmt.Fprint(outputWriter, "> ")
			continue
		case "reset", "/reset":
			c.History = nil
			c.LastSummary = ""
			c.LastSummaryTitle = ""
			fmt.Fprintln(outputWriter, "\n已重置会话上下文（history 已清空）。")
			fmt.Fprint(outputWriter, "\n> ")
			continue
		}

		if c.SpecMode && strings.TrimSpace(c.pendingSpecSlug) == "" {
			slug, err := c.generateSpecDocs(input)
			if err != nil {
				fmt.Fprintf(outputWriter, "\n生成 Spec 失败：%v\n", err)
				fmt.Fprint(outputWriter, "\n> ")
				continue
			}
			c.pendingSpecSlug = slug
			c.pendingSpecInput = input
			fmt.Fprintf(outputWriter, "\n已生成 Spec 文件：workspace/specs/%s/\n", slug)
			fmt.Fprintln(outputWriter, "请回复：确认Spec，开始实施")
			fmt.Fprint(outputWriter, "\n> ")
			continue
		}

		writeLog(logger, fmt.Sprintf("\n### User:\n%s\n", redactSecrets(input)))
		if c.SessionManager != nil {
			c.SessionManager.RecordMessage("user", redactSecrets(input))
		}

		// Update session with user input
		if c.SessionManager != nil {
			c.SessionManager.IncrementMessageCount()
			c.SessionManager.SetCurrentTask(input)
		}

		nextUserInput := input
		lastAssistant := ""
		for iter := 0; iter < c.MaxToolIters; iter++ {
			resp, err := c.ChatOnce(nextUserInput)
			if err != nil {
				fmt.Fprintf(outputWriter, "\nError: %v\n", err)
				writeLog(logger, fmt.Sprintf("\n**Error**: %v\n", err))
				break
			}
			lastAssistant = resp

			fmt.Fprintf(outputWriter, "\n%s\n", redactSecrets(resp))
			writeLog(logger, fmt.Sprintf("\n### Ni bot:\n%s\n", redactSecrets(resp)))
			if c.SessionManager != nil {
				c.SessionManager.RecordMessage("assistant", redactSecrets(resp))
			}

			calls := ExtractExecCalls(resp)
			if len(calls) == 0 {
				break
			}

			approver := &cliApprover{scanner: scanner, out: outputWriter, logger: logger, logLevel: c.Config.LogLevel}
			results := ExecuteCalls(ExecContext{Workspace: c.Workspace, Policy: c.Config.Policy}, calls, approver)
			fullToolSummary := formatToolResults(results)
			toolSummaryForModel := redactSecrets(fullToolSummary)
			toolSummaryForDisplay := toolSummaryForModel
			toolSummaryForLog := toolSummaryForModel
			if normalizeLogLevel(c.Config.LogLevel) == "meta" {
				toolSummaryForLog = redactSecrets(formatToolResultsMeta(results))
			}

			writeLog(logger, "\n### Tool Results\n")
			writeLog(logger, toolSummaryForLog+"\n")
			writeAuditToolResults(logger, c.Config.LogLevel, calls, results)
			fmt.Fprintln(outputWriter, "\n[Tool Results]")
			fmt.Fprintln(outputWriter, toolSummaryForDisplay)
			if c.SessionManager != nil {
				c.SessionManager.RecordMessage("tool_results", toolSummaryForModel)
				c.SessionManager.RecordToolResults(calls, results)
			}

			if c.SessionManager != nil && len(calls) > 0 {
				for i := range calls {
					c.SessionManager.IncrementToolCalls()
					if i < len(results) && results[i].Error == "denied by user" {
						c.SessionManager.IncrementDenials()
						continue
					}
					if c.Config.Policy.RequiresApproval(calls[i].Tool) {
						c.SessionManager.IncrementApprovals()
					}
				}
			}

			nextUserInput = toolSummaryForModel
		}

		_ = c.maybeAutoExtractMemory(input, lastAssistant, scanner, outputWriter, logger)

		fmt.Fprint(outputWriter, "\n> ")
	}
}

func (c *LLMClient) maybeAutoExtractMemory(userText, assistantText string, scanner *bufio.Scanner, outputWriter io.Writer, logger *os.File) error {
	if !autoMemoryEnabled() {
		return nil
	}
	userText = strings.TrimSpace(userText)
	assistantText = strings.TrimSpace(assistantText)
	if userText == "" || assistantText == "" {
		return nil
	}
	s, err := OpenSQLiteStore(c.Workspace)
	if err != nil || s == nil {
		if s != nil {
			s.Close()
		}
		return nil
	}
	s.Close()

	scope := strings.TrimSpace(os.Getenv("NIBOT_AUTO_MEMORY_SCOPE"))
	if scope == "" {
		scope = "global"
	}
	baseTags := strings.TrimSpace(os.Getenv("NIBOT_AUTO_MEMORY_TAGS"))
	if baseTags == "" {
		baseTags = "auto"
	}
	maxItems := parseIntEnv("NIBOT_AUTO_MEMORY_MAX_ITEMS", 6, 1, 20)

	type proposal struct {
		Action  string `json:"action"`
		Scope   string `json:"scope"`
		Tags    string `json:"tags"`
		Content string `json:"content"`
	}
	type proposalResp struct {
		Items []proposal `json:"items"`
	}

	system := "你是一个严格的“长期记忆提取器”。只输出 JSON，不要输出其他任何文本。\n\n" +
		"目标：从对话中提取“稳定且可复用”的信息（长期偏好、项目约束、工作流约定、长期事实）。\n" +
		"禁止：密钥/token/账号、隐私（身份证/住址/手机号/银行卡）、一次性任务细节、可从上下文直接推断的内容。\n" +
		"输出格式：{\"items\":[{\"action\":\"store\",\"scope\":\"global\",\"tags\":\"...\",\"content\":\"...\"}, ...]}。\n" +
		fmt.Sprintf("限制：items 不超过 %d 条；content 必须是简短的一句话。", maxItems)

	u := "USER:\n" + redactSecrets(userText) + "\n\nASSISTANT:\n" + redactSecrets(assistantText)
	resp, err := c.Call([]Message{
		{Role: "system", Content: system},
		{Role: "user", Content: u},
	})
	if err != nil {
		return nil
	}
	js := extractJSONBlock(resp)
	var pr proposalResp
	if err := json.Unmarshal([]byte(js), &pr); err != nil {
		return nil
	}
	if len(pr.Items) == 0 {
		return nil
	}
	if len(pr.Items) > maxItems {
		pr.Items = pr.Items[:maxItems]
	}

	var calls []ExecCall
	for _, it := range pr.Items {
		action := strings.ToLower(strings.TrimSpace(it.Action))
		if action == "" {
			action = "store"
		}
		if action != "store" && action != "add" && action != "update" {
			continue
		}
		content := strings.TrimSpace(it.Content)
		if content == "" {
			continue
		}
		tags := strings.TrimSpace(it.Tags)
		if tags == "" {
			tags = baseTags
		} else {
			tags = mergeTags(baseTags, tags)
		}
		sc := strings.TrimSpace(it.Scope)
		if sc == "" {
			sc = scope
		}
		args, _ := json.Marshal(map[string]any{
			"scope":   sc,
			"tags":    tags,
			"content": content,
		})
		calls = append(calls, ExecCall{Tool: "memory.store", ArgsRaw: string(args)})
	}
	if len(calls) == 0 {
		return nil
	}

	fmt.Fprintln(outputWriter, "\n[Auto Memory]")
	fmt.Fprintf(outputWriter, "提取到 %d 条候选记忆，将逐条请求审批。\n", len(calls))
	writeLog(logger, fmt.Sprintf("\n### Auto Memory Proposals (%d)\n", len(calls)))

	approver := &cliApprover{scanner: scanner, out: outputWriter, logger: logger, logLevel: c.Config.LogLevel}
	results := ExecuteCalls(ExecContext{Workspace: c.Workspace, Policy: c.Config.Policy}, calls, approver)

	fullToolSummary := formatToolResults(results)
	toolSummaryForModel := redactSecrets(fullToolSummary)
	toolSummaryForLog := toolSummaryForModel
	if normalizeLogLevel(c.Config.LogLevel) == "meta" {
		toolSummaryForLog = redactSecrets(formatToolResultsMeta(results))
	}

	writeLog(logger, "\n### Auto Memory Results\n")
	writeLog(logger, toolSummaryForLog+"\n")
	writeAuditToolResults(logger, c.Config.LogLevel, calls, results)
	fmt.Fprintln(outputWriter, "\n[Auto Memory Results]")
	fmt.Fprintln(outputWriter, toolSummaryForModel)

	if c.SessionManager != nil && len(calls) > 0 {
		c.SessionManager.RecordMessage("tool_results", toolSummaryForModel)
		c.SessionManager.RecordToolResults(calls, results)
		for i := range calls {
			c.SessionManager.IncrementToolCalls()
			if i < len(results) && results[i].Error == "denied by user" {
				c.SessionManager.IncrementDenials()
				continue
			}
			if c.Config.Policy.RequiresApproval(calls[i].Tool) {
				c.SessionManager.IncrementApprovals()
			}
		}
	}

	return nil
}

func autoMemoryEnabled() bool {
	v := strings.TrimSpace(os.Getenv("NIBOT_AUTO_MEMORY"))
	if v == "" {
		return false
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func parseIntEnv(key string, def, min, max int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func extractJSONBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimPrefix(strings.TrimSpace(s), "json")
		s = strings.TrimSpace(s)
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}

func isSpecConfirmInput(input string) bool {
	in := strings.ToLower(strings.TrimSpace(input))
	in = strings.ReplaceAll(in, " ", "")
	in = strings.ReplaceAll(in, "：", ":")
	return strings.Contains(in, "确认spec") || strings.Contains(in, "开始实施") || strings.Contains(in, "startimplement")
}

func hasToken(tokens []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, t := range tokens {
		if strings.ToLower(strings.TrimSpace(t)) == want {
			return true
		}
	}
	return false
}

func loadSpecModeSetting(workspace string) bool {
	if v, ok := os.LookupEnv("NIBOT_SPEC_MODE"); ok && strings.TrimSpace(v) != "" {
		vv := strings.ToLower(strings.TrimSpace(v))
		return vv == "1" || vv == "true" || vv == "yes" || vv == "on"
	}
	p := filepath.Join(workspace, "data", "spec_mode.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	var v struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return false
	}
	return v.Enabled
}

func saveSpecModeSetting(workspace string, enabled bool) error {
	p := filepath.Join(workspace, "data", "spec_mode.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(map[string]any{"enabled": enabled}, "", "  ")
	return os.WriteFile(p, b, 0o644)
}

func (c *LLMClient) generateSpecDocs(requirement string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(c.Config.Provider))
	if provider != "ollama" && strings.TrimSpace(c.Config.APIKey) == "" {
		return "", fmt.Errorf("missing API key for spec generation")
	}
	system := "你是一个“Spec 模式”助手：必须先输出规格文档，再等待用户确认后才开始实施。\n\n" +
		"请根据用户需求生成 3 份 Markdown 文档，并以 JSON 输出：\n" +
		"{\"slug\":\"...\",\"spec_md\":\"...\",\"tasks_md\":\"...\",\"checklist_md\":\"...\"}\n\n" +
		"要求：\n" +
		"- 语言：中文\n" +
		"- spec_md 包含：Why/What/Impact/Requirements/Non-Goals/安全与隐私边界/回滚策略\n" +
		"- tasks_md：以任务列表形式拆分可实施步骤\n" +
		"- checklist_md：验收清单（可勾选）\n" +
		"- 仅输出 JSON，不要输出其他文字"
	resp, err := c.Call([]Message{
		{Role: "system", Content: system},
		{Role: "user", Content: strings.TrimSpace(requirement)},
	})
	if err != nil {
		return "", err
	}
	js := extractJSONBlock(resp)
	var v struct {
		Slug        string `json:"slug"`
		SpecMD      string `json:"spec_md"`
		TasksMD     string `json:"tasks_md"`
		ChecklistMD string `json:"checklist_md"`
	}
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		return "", fmt.Errorf("invalid spec JSON: %w", err)
	}
	slug := slugify(v.Slug)
	if slug == "" {
		slug = "spec-" + time.Now().Format("20060102-150405")
	}
	dir := filepath.Join(c.Workspace, "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(stringsTrimCRLF(v.SpecMD)), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(stringsTrimCRLF(v.TasksMD)), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "checklist.md"), []byte(stringsTrimCRLF(v.ChecklistMD)), 0o644); err != nil {
		return "", err
	}
	return slug, nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' || r == '_' || r == ' ' {
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 60 {
		out = out[:60]
		out = strings.Trim(out, "-")
	}
	return out
}

func stringsTrimCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n") + "\n"
}

func (c *LLMClient) persistSessionOnExit() {
	if c.SessionManager != nil {
		if session := c.SessionManager.GetCurrentSession(); session != nil {
			// Add final memory item about session completion
			c.SessionManager.AddToMemory(fmt.Sprintf("Session completed with %d messages, %d tool calls",
				session.MessageCount, session.ToolCalls))

			// Persist the final session state
			if err := c.SessionManager.PersistSession(session); err != nil {
				log.Printf("Failed to persist session state on exit: %v", err)
			} else {
				log.Printf("Session state persisted successfully: %s", session.SessionID)
			}

			// Notify health monitor about session ending
			c.SessionManager.SessionEnded()
		}
	}
}

func writeLog(f *os.File, content string) {
	if f != nil {
		f.WriteString(content)
	}
}

type cliApprover struct {
	scanner  *bufio.Scanner
	out      io.Writer
	logger   *os.File
	logLevel string
}

func (a *cliApprover) Approve(call ExecCall) bool {
	fmt.Fprintf(a.out, "\nApprove %s %s ? (y/n): ", call.Tool, redactSecrets(previewArgs(call.ArgsRaw)))
	for a.scanner.Scan() {
		v := strings.ToLower(strings.TrimSpace(a.scanner.Text()))
		if v == "y" || v == "yes" {
			writeAuditApproval(a.logger, a.logLevel, call, true)
			return true
		}
		if v == "n" || v == "no" {
			writeAuditApproval(a.logger, a.logLevel, call, false)
			return false
		}
		fmt.Fprint(a.out, "Please enter y or n: ")
	}
	writeAuditApproval(a.logger, a.logLevel, call, false)
	return false
}

func previewArgs(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return ""
	}
	if len(args) <= 120 {
		return args
	}
	return args[:120] + "..."
}

func formatToolResults(results []ToolResult) string {
	var sb strings.Builder
	sb.WriteString("TOOL_RESULTS:\n")
	for _, r := range results {
		sb.WriteString("- tool: " + r.Tool + "\n")
		sb.WriteString("  ok: " + fmt.Sprintf("%v", r.OK) + "\n")
		if r.Error != "" {
			sb.WriteString("  error: " + strings.ReplaceAll(r.Error, "\n", "\\n") + "\n")
		}
		if r.Output != "" {
			out := r.Output
			out = strings.ReplaceAll(out, "\r\n", "\n")
			out = strings.ReplaceAll(out, "\r", "\n")
			out = strings.TrimSpace(out)
			if len(out) > 2000 {
				out = out[:2000] + "\n[TRUNCATED]"
			}
			sb.WriteString("  output: |\n")
			for _, line := range strings.Split(out, "\n") {
				sb.WriteString("    " + line + "\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("If you need to call tools again, output [EXEC:tool {json_args}] only.\n")
	return sb.String()
}
