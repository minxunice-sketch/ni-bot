package agent

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func LoadConfig(workspace string) Config {
	cfg := Config{
		Provider:  "openai",
		ModelName: "gpt-4-turbo",
	}
	cfg.BaseURL = defaultBaseURL(cfg.Provider)
	cfg.Policy = LoadToolPolicy(workspace)

	fileCfg, ok := readConfigToml(filepath.Join(workspace, "data", "config.toml"))
	if ok {
		if fileCfg.Provider != "" {
			cfg.Provider = fileCfg.Provider
		}
		if fileCfg.BaseURL != "" {
			cfg.BaseURL = fileCfg.BaseURL
		} else {
			cfg.BaseURL = defaultBaseURL(cfg.Provider)
		}
		if fileCfg.APIKey != "" {
			cfg.APIKey = fileCfg.APIKey
		}
		if fileCfg.ModelName != "" {
			cfg.ModelName = fileCfg.ModelName
		}
	}

	if v, ok := os.LookupEnv("LLM_PROVIDER"); ok && strings.TrimSpace(v) != "" {
		cfg.Provider = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("LLM_BASE_URL"); ok && strings.TrimSpace(v) != "" {
		cfg.BaseURL = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("LLM_MODEL_NAME"); ok && strings.TrimSpace(v) != "" {
		cfg.ModelName = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("LLM_API_KEY"); ok && strings.TrimSpace(v) != "" {
		cfg.APIKey = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("NIBOT_LOG_LEVEL"); ok && strings.TrimSpace(v) != "" {
		cfg.LogLevel = strings.TrimSpace(v)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL(cfg.Provider)
	}
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}

	if cfg.APIKey == "" && (cfg.Provider == "nvidia" || cfg.Provider == "nvidia_nim") {
		if v, ok := os.LookupEnv("NVIDIA_API_KEY"); ok && strings.TrimSpace(v) != "" {
			cfg.APIKey = strings.TrimSpace(v)
		}
	}

	return cfg
}

func defaultBaseURL(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return "http://localhost:11434/v1"
	case "nvidia", "nvidia_nim":
		return "https://integrate.api.nvidia.com/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

func readConfigToml(path string) (Config, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, false
	}
	defer f.Close()

	var cfg Config

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		val = strings.Trim(val, "'")

		switch key {
		case "provider":
			cfg.Provider = val
		case "base_url":
			cfg.BaseURL = val
		case "api_key":
			cfg.APIKey = val
		case "model":
			cfg.ModelName = val
	case "log_level":
		cfg.LogLevel = val
		}
	}

	if cfg.Provider == "" && cfg.BaseURL == "" && cfg.APIKey == "" && cfg.ModelName == "" {
		return Config{}, false
	}
	return cfg, true
}

