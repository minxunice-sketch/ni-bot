package agent

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func LoadConfig(workspace string) Config {
	var cfg Config
	cfg.Policy = LoadToolPolicy(workspace)

	providerSet := false
	baseURLSet := false
	apiKeySet := false
	modelSet := false
	logLevelSet := false

	if fileCfg, ok := readConfigYAML(filepath.Join(workspace, "data", "config.yaml")); ok {
		if strings.TrimSpace(fileCfg.Provider) != "" {
			cfg.Provider = strings.TrimSpace(fileCfg.Provider)
			providerSet = true
		}
		if strings.TrimSpace(fileCfg.BaseURL) != "" {
			cfg.BaseURL = strings.TrimSpace(fileCfg.BaseURL)
			baseURLSet = true
		}
		if strings.TrimSpace(fileCfg.APIKey) != "" {
			cfg.APIKey = strings.TrimSpace(fileCfg.APIKey)
			apiKeySet = true
		}
		if strings.TrimSpace(fileCfg.ModelName) != "" {
			cfg.ModelName = strings.TrimSpace(fileCfg.ModelName)
			modelSet = true
		}
		if strings.TrimSpace(fileCfg.LogLevel) != "" {
			cfg.LogLevel = strings.TrimSpace(fileCfg.LogLevel)
			logLevelSet = true
		}
	}

	if fileCfg, ok := readConfigToml(filepath.Join(workspace, "data", "config.toml")); ok {
		if !providerSet && strings.TrimSpace(fileCfg.Provider) != "" {
			cfg.Provider = strings.TrimSpace(fileCfg.Provider)
			providerSet = true
		}
		if !baseURLSet && strings.TrimSpace(fileCfg.BaseURL) != "" {
			cfg.BaseURL = strings.TrimSpace(fileCfg.BaseURL)
			baseURLSet = true
		}
		if !apiKeySet && strings.TrimSpace(fileCfg.APIKey) != "" {
			cfg.APIKey = strings.TrimSpace(fileCfg.APIKey)
			apiKeySet = true
		}
		if !modelSet && strings.TrimSpace(fileCfg.ModelName) != "" {
			cfg.ModelName = strings.TrimSpace(fileCfg.ModelName)
			modelSet = true
		}
		if !logLevelSet && strings.TrimSpace(fileCfg.LogLevel) != "" {
			cfg.LogLevel = strings.TrimSpace(fileCfg.LogLevel)
			logLevelSet = true
		}
	}

	if v, ok := os.LookupEnv("LLM_PROVIDER"); ok && strings.TrimSpace(v) != "" {
		cfg.Provider = strings.TrimSpace(v)
		providerSet = true
	}
	if v, ok := os.LookupEnv("LLM_BASE_URL"); ok && strings.TrimSpace(v) != "" {
		cfg.BaseURL = strings.TrimSpace(v)
		baseURLSet = true
	}
	if v, ok := os.LookupEnv("LLM_MODEL_NAME"); ok && strings.TrimSpace(v) != "" {
		cfg.ModelName = strings.TrimSpace(v)
		modelSet = true
	}
	if v, ok := os.LookupEnv("LLM_API_KEY"); ok && strings.TrimSpace(v) != "" {
		cfg.APIKey = strings.TrimSpace(v)
		apiKeySet = true
	}
	if v, ok := os.LookupEnv("NIBOT_LOG_LEVEL"); ok && strings.TrimSpace(v) != "" {
		cfg.LogLevel = strings.TrimSpace(v)
		logLevelSet = true
	}

	if !providerSet || strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = "deepseek"
		providerSet = true
	}
	if !baseURLSet || strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL(cfg.Provider)
	}
	if !modelSet || strings.TrimSpace(cfg.ModelName) == "" {
		cfg.ModelName = defaultModelName(cfg.Provider)
	}
	if !logLevelSet || strings.TrimSpace(cfg.LogLevel) == "" {
		cfg.LogLevel = "full"
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
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "openai":
		return ""
	default:
		return ""
	}
}

func readConfigYAML(path string) (Config, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, false
	}
	defer f.Close()

	var cfg Config
	inLLM := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimRight(raw, "\r\n")
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "#") {
			continue
		}

		if strings.HasSuffix(trim, ":") && !strings.Contains(trim, " ") {
			section := strings.TrimSuffix(trim, ":")
			inLLM = strings.ToLower(strings.TrimSpace(section)) == "llm"
			continue
		}
		if !inLLM {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			continue
		}
		kv := strings.TrimSpace(line)
		parts := strings.SplitN(kv, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		val = strings.Trim(val, "'")
		val = strings.Trim(val, "`")

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

	if cfg.Provider == "" && cfg.BaseURL == "" && cfg.APIKey == "" && cfg.ModelName == "" && cfg.LogLevel == "" {
		return Config{}, false
	}
	return cfg, true
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

	if cfg.Provider == "" && cfg.BaseURL == "" && cfg.APIKey == "" && cfg.ModelName == "" && cfg.LogLevel == "" {
		return Config{}, false
	}
	return cfg, true
}
