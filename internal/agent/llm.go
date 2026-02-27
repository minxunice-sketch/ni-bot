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
	"strings"
	"sync"
	"time"
	"unicode"
)

type LLMClient struct {
	mu              sync.RWMutex
	Config          Config
	History         []Message
	SystemMsg       string
	Workspace       string
	SessionManager  *SessionManager
	MaxToolIters    int
	LastSummary     string
	LastSummaryTitle string
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
	Model      string      `json:"model"`
	Messages   []Message    `json:"messages"`
	Tools      []openAITool `json:"tools,omitempty"`
	ToolChoice any         `json:"tool_choice,omitempty"`
}

type openAITool struct {
	Type     string           `json:"type"`
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
	Type     string           `json:"type"`
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
	}
}

func (c *LLMClient) Chat(userInput string) (string, error) {
	c.History = append(c.History, Message{Role: "user", Content: redactSecrets(userInput)})

	c.mu.RLock()
	systemMsg := c.SystemMsg
	c.mu.RUnlock()

	messages := []Message{{Role: "system", Content: systemMsg}}
	messages = append(messages, c.History...)

	var responseContent string
	var err error

	provider := strings.ToLower(strings.TrimSpace(c.Config.Provider))
	if provider == "" {
		provider = "openai"
	}
	if provider != "ollama" && strings.TrimSpace(c.Config.APIKey) == "" {
		responseContent = c.mockRespond(userInput)
		c.History = append(c.History, Message{Role: "assistant", Content: redactSecrets(responseContent)})
		return responseContent, nil
	}

	switch provider {
	case "openai", "deepseek", "nvidia", "nvidia_nim":
		responseContent, err = c.callOpenAICompatible(messages)
	case "ollama":
		responseContent, err = c.callOllama(messages)
	default:
		responseContent = fmt.Sprintf("[MOCK] Received: %s\n\nIf you want, I can read memory: [EXEC:fs.read {\"path\":\"memory/facts.md\"}]", userInput)
	}

	if err != nil {
		return "", err
	}

	c.History = append(c.History, Message{Role: "assistant", Content: redactSecrets(responseContent)})
	return responseContent, nil
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
	
	fmt.Fprintln(outputWriter, "\n> Ni bot initialized. Type your request (or 'exit' to quit):")
	fmt.Fprint(outputWriter, "> ")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		tokens := splitCommandLine(input)
		if len(tokens) == 0 {
			fmt.Fprint(outputWriter, "> ")
			continue
		}
		cmd := strings.ToLower(tokens[0])
		switch cmd {
		case "exit", "quit":
			return
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
			fmt.Fprintln(outputWriter, "- skills / /skills: list skills")
			fmt.Fprintln(outputWriter, "- skills show <name>: show skill docs and scripts")
			fmt.Fprintln(outputWriter, "- skills search <kw>: search skills by keyword")
			fmt.Fprintln(outputWriter, "- skills install <path>: install skills from a local folder")
			fmt.Fprintln(outputWriter, "- skills doctor: validate installed skills")
			fmt.Fprintln(outputWriter, "- skills test <name>: test a skill without executing")
			fmt.Fprintln(outputWriter, "- reload / /reload: reload system prompt (skills/memory)")
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
		for iter := 0; iter < c.MaxToolIters; iter++ {
			resp, err := c.Chat(nextUserInput)
			if err != nil {
				fmt.Fprintf(outputWriter, "\nError: %v\n", err)
				writeLog(logger, fmt.Sprintf("\n**Error**: %v\n", err))
				break
			}

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

		fmt.Fprint(outputWriter, "\n> ")
	}
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
	scanner *bufio.Scanner
	out     io.Writer
	logger  *os.File
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
