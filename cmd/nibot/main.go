package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

func main() {
	var workspaceFlag string
	var cmds stringListFlag

	flag.StringVar(&workspaceFlag, "workspace", "", "Workspace directory (default: ./workspace)")
	flag.Var(&cmds, "cmd", "Non-interactive mode: run a command and exit (repeatable)")
	flag.Parse()

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
	log.Printf("Starting Ni bot in workspace: %s", workspace)

	// Initialize health monitor
	healthPort := 0
	if portStr, ok := os.LookupEnv("NIBOT_HEALTH_PORT"); ok {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			healthPort = port
		}
	}
	healthMonitor := agent.NewHealthMonitor(healthPort)
	defer healthMonitor.Shutdown()

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
