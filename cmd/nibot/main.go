package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nibot/internal/agent"
)

type stringListFlag []string

func (s *stringListFlag) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringListFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var Version = "dev"

func main() {
	var workspaceFlag string
	var cmds stringListFlag
	var showVersion bool
	var enableTelegram bool

	flag.StringVar(&workspaceFlag, "workspace", "", "Workspace directory (default: ./workspace)")
	flag.Var(&cmds, "cmd", "Non-interactive mode: run a command and exit (repeatable)")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&enableTelegram, "telegram", false, "Enable Telegram bot mode")
	flag.Parse()

	if showVersion {
		fmt.Println(Version)
		return
	}
	if strings.TrimSpace(os.Getenv("NIBOT_VERSION")) == "" {
		_ = os.Setenv("NIBOT_VERSION", Version)
	}

	// 设置必开环境变量（确保核心功能默认启用）
	if os.Getenv("NIBOT_ENABLE_EXEC") == "" {
		_ = os.Setenv("NIBOT_ENABLE_EXEC", "1") // 必开：启用执行能力
	}
	if os.Getenv("GOPROXY") == "" {
		_ = os.Setenv("GOPROXY", "https://goproxy.cn,direct") // 必开：国内镜像加速
	}
	// 清空LLM_API_BASE避免误入OpenAI默认地址
	if os.Getenv("LLM_API_BASE") == "" {
		_ = os.Setenv("LLM_API_BASE", "")
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	workspace := filepath.Join(cwd, "workspace")
	if strings.TrimSpace(workspaceFlag) != "" {
		if p, err := filepath.Abs(strings.TrimSpace(workspaceFlag)); err == nil {
			workspace = p
		}
	}
	// 显示启动状态信息
	enableExec := os.Getenv("NIBOT_ENABLE_EXEC")
	enableSkills := os.Getenv("NIBOT_ENABLE_SKILLS")
	autoApprove := os.Getenv("NIBOT_AUTO_APPROVE")
	
	getStatusDisplay := func(enabled bool) string {
		if enabled {
			return "✅ ON"
		}
		return "❌ OFF"
	}
	
	getEnvDisplay := func(key string) string {
		value := os.Getenv(key)
		if value == "" {
			return "(not set)"
		}
		if len(value) > 20 {
			return value[:20] + "..."
		}
		return value
	}
	
	log.Printf("🚀 Starting Ni bot (v%s)", Version)
	log.Printf("   Workspace: %s", workspace)
	log.Printf("   EXEC: %s", getStatusDisplay(enableExec == "1"))
	log.Printf("   SKILLS: %s", getStatusDisplay(enableSkills == "1"))
	log.Printf("   AUTO_APPROVE: %s", getStatusDisplay(autoApprove == "true"))
	log.Printf("   GOPROXY: %s", getEnvDisplay("GOPROXY"))
	log.Printf("   LLM_API_BASE: %s", getEnvDisplay("LLM_API_BASE"))

	// 自动创建工作目录（集成start.sh逻辑到Go代码）
	if err := os.MkdirAll(filepath.Join(workspace, "logs"), 0o755); err != nil {
		log.Printf("Warning: Failed to create logs directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		log.Printf("Warning: Failed to create memory directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "data"), 0o755); err != nil {
		log.Printf("Warning: Failed to create data directory: %v", err)
	}



	if err := agent.EnsureWorkspaceScaffold(workspace); err != nil {
		log.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Initialize health monitor
	healthPort := 0
	if portStr, ok := os.LookupEnv("NIBOT_HEALTH_PORT"); ok {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			healthPort = port
		}
	}
	healthMonitor := agent.NewHealthMonitor(healthPort)
	defer healthMonitor.Shutdown()

	interactive := stdinIsTerminal() && len(cmds) == 0
	if err := agent.EnsureConfig(workspace, interactive, os.Stdout); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	cfg := agent.LoadConfig(workspace)
	log.Printf("Loaded Config: Provider=%s, Model=%s, LogLevel=%s", cfg.Provider, cfg.ModelName, cfg.LogLevel)
	if cfg.APIKey == "" && cfg.Provider != "ollama" {
		log.Printf("Warning: No API Key provided for %s", cfg.Provider)
	}

	// Initialize session manager
	sessionManager := agent.NewSessionManager(workspace, healthMonitor)
	session := sessionManager.StartNewSession()
	sessionID := session.SessionID
	logFile := filepath.Join(workspace, "logs", sessionID+".md")
	logger, err := initSessionLogger(logFile)
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer logger.Close()
	log.Printf("Session trace will be written to: %s", logFile)
	
	writeLog(logger, fmt.Sprintf("# Session Trace: %s\n\n**Provider**: %s\n**Model**: %s\n\n---\n", sessionID, cfg.Provider, cfg.ModelName))

	systemPrompt, err := agent.ConstructSystemPrompt(workspace)
	if err != nil {
		log.Fatalf("Failed to construct system prompt: %v", err)
	}
	if strings.ToLower(strings.TrimSpace(cfg.LogLevel)) == "meta" {
		writeLog(logger, fmt.Sprintf("## System Prompt Constructed\n\n(prompt_bytes=%d)\n\n---\n", len([]byte(systemPrompt))))
	} else {
		writeLog(logger, fmt.Sprintf("## System Prompt Constructed\n\n```\n%s\n```\n\n---\n", agent.RedactForLog(systemPrompt)))
	}
	log.Println("System Prompt constructed successfully.")

	fmt.Println("Ni bot is ready.")
	
	client := agent.NewLLMClient(cfg, workspace, systemPrompt, sessionManager)

	// 启动 Telegram 机器人（如果启用）
	if enableTelegram || os.Getenv("TELEGRAM_BOT_TOKEN") != "" {
		telegramConfig := agent.NewTelegramConfig()
		telegramBot, err := agent.NewTelegramBot(telegramConfig, cfg, workspace, systemPrompt, healthMonitor)
		if err != nil {
			log.Fatalf("Failed to create Telegram bot: %v", err)
		}
		
		log.Printf("Starting Telegram bot in background...")
		go func() {
			ctx := context.Background()
			if err := telegramBot.Start(ctx); err != nil {
				log.Printf("Telegram bot stopped with error: %v", err)
			}
		}()
		
		// 等待 Telegram 机器人启动
		time.Sleep(2 * time.Second)
	}

	if len(cmds) > 0 {
		var b bytes.Buffer
		for _, c := range cmds {
			b.WriteString(c)
			if !strings.HasSuffix(c, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("exit\n")
		client.Loop(bytes.NewReader(b.Bytes()), os.Stdout, logger)
		return
	}

	client.Loop(os.Stdin, os.Stdout, logger)
}

func initSessionLogger(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func writeLog(f *os.File, content string) {
	if _, err := f.WriteString(content); err != nil {
		log.Printf("Failed to write to session log: %v", err)
	}
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
