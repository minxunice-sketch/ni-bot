package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func EnsureConfig(workspace string, interactive bool, out io.Writer) error {
	cfgPath := filepath.Join(workspace, "data", "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}

	if !interactive {
		content := renderConfigYAML(Config{
			Provider:  "nvidia",
			BaseURL:   defaultBaseURL("nvidia"),
			ModelName: defaultModelName("nvidia"),
			APIKey:    "",
			LogLevel:  "",
		})
		if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
			return err
		}
		if out != nil {
			_, _ = fmt.Fprintf(out, "已生成默认配置文件：%s\n", cfgPath)
		}
		return nil
	}

	if out != nil {
		_, _ = fmt.Fprintln(out, "首次运行：未检测到配置文件，开始初始化 LLM 配置。可直接回车使用默认值。")
	}

	in := bufio.NewReader(os.Stdin)
	provider := readLine(out, in, "LLM Provider（默认 nvidia，可选：nvidia/ollama/openai）: ")
	if strings.TrimSpace(provider) == "" {
		provider = "nvidia"
	}
	provider = strings.TrimSpace(provider)

	defaultURL := defaultBaseURL(provider)
	baseURLPrompt := "Base URL"
	if strings.TrimSpace(defaultURL) != "" {
		baseURLPrompt = fmt.Sprintf("Base URL（默认 %s）", defaultURL)
	}
	baseURL := readLine(out, in, baseURLPrompt+": ")
	baseURL = strings.TrimSpace(strings.Trim(baseURL, "`"))
	if baseURL == "" {
		baseURL = defaultURL
	}

	defaultModel := defaultModelName(provider)
	modelPrompt := "Model"
	if strings.TrimSpace(defaultModel) != "" {
		modelPrompt = fmt.Sprintf("Model（默认 %s）", defaultModel)
	}
	model := strings.TrimSpace(readLine(out, in, modelPrompt+": "))
	if model == "" {
		model = defaultModel
	}

	apiKey := strings.TrimSpace(readLine(out, in, "API Key（留空进入 Mock 模式）: "))
	logLevel := strings.TrimSpace(readLine(out, in, "Log Level（可选：full/meta，默认 full）: "))

	content := renderConfigYAML(Config{
		Provider:  provider,
		BaseURL:   baseURL,
		ModelName: model,
		APIKey:    apiKey,
		LogLevel:  logLevel,
	})
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		return err
	}
	if out != nil {
		_, _ = fmt.Fprintf(out, "✅ 配置文件已生成：%s\n", cfgPath)
	}
	return nil
}

func readLine(out io.Writer, in *bufio.Reader, prompt string) string {
	if out != nil {
		_, _ = fmt.Fprint(out, prompt)
	}
	s, _ := in.ReadString('\n')
	return strings.TrimRight(s, "\r\n")
}

func renderConfigYAML(cfg Config) string {
	provider := strings.TrimSpace(cfg.Provider)
	baseURL := strings.TrimSpace(cfg.BaseURL)
	model := strings.TrimSpace(cfg.ModelName)
	apiKey := strings.TrimSpace(cfg.APIKey)
	logLevel := strings.TrimSpace(cfg.LogLevel)

	var sb strings.Builder
	sb.WriteString("llm:\n")
	if provider != "" {
		sb.WriteString(fmt.Sprintf("  provider: %q\n", provider))
	} else {
		sb.WriteString("  provider: \"\"\n")
	}
	if baseURL != "" {
		sb.WriteString(fmt.Sprintf("  base_url: %q\n", baseURL))
	} else {
		sb.WriteString("  base_url: \"\"\n")
	}
	if model != "" {
		sb.WriteString(fmt.Sprintf("  model: %q\n", model))
	} else {
		sb.WriteString("  model: \"\"\n")
	}
	if apiKey != "" {
		sb.WriteString(fmt.Sprintf("  api_key: %q\n", apiKey))
	} else {
		sb.WriteString("  api_key: \"\"\n")
	}
	if logLevel != "" {
		sb.WriteString(fmt.Sprintf("  log_level: %q\n", logLevel))
	} else {
		sb.WriteString("  log_level: \"\"\n")
	}
	return sb.String()
}

func defaultModelName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "nvidia", "nvidia_nim":
		return "moonshotai/kimi-k2.5"
	case "ollama":
		return "qwen2.5:7b"
	default:
		return "gpt-4-turbo"
	}
}
