package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nibot/internal/agent"

	"github.com/gorilla/websocket"
)

var (
	globalConfig agent.Config
	configMutex  sync.RWMutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

type ChatResponse struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type SkillStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
}

func main() {
	// 设置默认端口
	port := os.Getenv("NIBOT_WEB_PORT")
	if port == "" {
		port = "8080"
	}

	// 创建工作目录
	cwd, _ := os.Getwd()
	workspace := filepath.Join(cwd, "workspace")
	os.MkdirAll(filepath.Join(workspace, "logs"), 0o755)
	os.MkdirAll(filepath.Join(workspace, "memory"), 0o755)
	os.MkdirAll(filepath.Join(workspace, "data"), 0o755)

	// 加载配置
	reloadConfig(workspace)

	// 显示启动状态信息 (与 CLI 保持一致)
	enableExec := os.Getenv("NIBOT_ENABLE_EXEC")
	enableSkills := os.Getenv("NIBOT_ENABLE_SKILLS")

	getStatusDisplay := func(enabled bool) string {
		if enabled {
			return "✅ ON"
		}
		return "❌ OFF"
	}

	configMutex.RLock()
	log.Printf("🚀 Ni Bot Web Interface started on http://localhost:%s", port)
	log.Printf("   Open http://localhost:%s in your browser to start chatting", port)
	log.Printf("   Workspace: %s", workspace)
	log.Printf("   EXEC: %s", getStatusDisplay(enableExec == "1"))
	log.Printf("   SKILLS: %s", getStatusDisplay(enableSkills == "1"))
	log.Printf("   Provider: %s, Model: %s", globalConfig.Provider, globalConfig.ModelName)
	configMutex.RUnlock()

	// 设置静态文件服务
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API路由
	http.HandleFunc("/api/chat", chatHandler)
	http.HandleFunc("/api/config", configHandler)
	http.HandleFunc("/api/skills", skillsHandler)
	http.HandleFunc("/api/skills/toggle", skillToggleHandler)
	http.HandleFunc("/ws", websocketHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/templates/index.html")
	})

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func reloadConfig(workspace string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = agent.LoadConfig(workspace)
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 处理用户消息
	response := processMessage(req.Message, req.SessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	cwd, _ := os.Getwd()
	workspace := filepath.Join(cwd, "workspace")

	if r.Method == "GET" {
		configMutex.RLock()
		defer configMutex.RUnlock()

		// Return config but mask API key for security
		cfg := globalConfig
		if len(cfg.APIKey) > 8 {
			cfg.APIKey = cfg.APIKey[:4] + "..." + cfg.APIKey[len(cfg.APIKey)-4:]
		} else if cfg.APIKey != "" {
			cfg.APIKey = "***"
		}

		json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == "POST" {
		var newCfg agent.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update Config
		configMutex.Lock()
		// Preserve fields not sent if needed, but for now assume full update or partial merge
		// Here we assume the UI sends the full config struct.
		// However, API Key might be masked. If it's masked or empty, keep old one?
		if newCfg.APIKey == "" || strings.Contains(newCfg.APIKey, "***") || strings.Contains(newCfg.APIKey, "...") {
			newCfg.APIKey = globalConfig.APIKey
		}

		// Save to file
		if err := agent.SaveConfig(workspace, newCfg); err != nil {
			configMutex.Unlock()
			http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Update Policy
		if err := agent.SaveToolPolicy(workspace, newCfg.Policy); err != nil {
			configMutex.Unlock()
			http.Error(w, "Failed to save policy: "+err.Error(), http.StatusInternalServerError)
			return
		}

		configMutex.Unlock()

		// Reload to ensure consistency
		reloadConfig(workspace)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func skillsHandler(w http.ResponseWriter, r *http.Request) {
	cwd, _ := os.Getwd()
	workspace := filepath.Join(cwd, "workspace")

	skills, err := agent.DiscoverSkills(workspace)
	if err != nil {
		http.Error(w, "Failed to discover skills: "+err.Error(), http.StatusInternalServerError)
		return
	}

	configMutex.RLock()
	policy := globalConfig.Policy
	configMutex.RUnlock()

	var result []SkillStatus
	for _, s := range skills {
		enabled := true
		// If whitelist is not empty, check if skill is in it
		if len(policy.AllowedSkillNames) > 0 {
			enabled = false
			for _, allowed := range policy.AllowedSkillNames {
				if allowed == "*" || strings.EqualFold(allowed, s.Name) {
					enabled = true
					break
				}
			}
		}

		result = append(result, SkillStatus{
			Name:        s.Name,
			DisplayName: s.DisplayName,
			Description: s.Description,
			Enabled:     enabled,
			Source:      s.Source,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func skillToggleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	cwd, _ := os.Getwd()
	workspace := filepath.Join(cwd, "workspace")

	configMutex.Lock()
	defer configMutex.Unlock()

	policy := globalConfig.Policy

	// If currently "Allow All" (empty list), populate with ALL skills first
	if len(policy.AllowedSkillNames) == 0 {
		allSkills, _ := agent.DiscoverSkills(workspace)
		for _, s := range allSkills {
			policy.AllowedSkillNames = append(policy.AllowedSkillNames, s.Name)
		}
	}

	if req.Enabled {
		// Add to allowed list if not present
		found := false
		for _, name := range policy.AllowedSkillNames {
			if strings.EqualFold(name, req.Name) {
				found = true
				break
			}
		}
		if !found {
			policy.AllowedSkillNames = append(policy.AllowedSkillNames, req.Name)
		}
	} else {
		// Remove from allowed list
		var newAllowed []string
		for _, name := range policy.AllowedSkillNames {
			if !strings.EqualFold(name, req.Name) {
				newAllowed = append(newAllowed, name)
			}
		}
		// If list becomes empty after removal (and we are in restrictive mode),
		// add a placeholder to prevent reverting to "Allow All"
		if len(newAllowed) == 0 {
			newAllowed = append(newAllowed, "__DISABLED__")
		}
		policy.AllowedSkillNames = newAllowed
	}

	// Optimization: If allowed list contains ALL discovered skills, clear it to revert to "Allow All" mode?
	// Actually, safer to keep explicit list once modified.

	// Save Policy
	if err := agent.SaveToolPolicy(workspace, policy); err != nil {
		http.Error(w, "Failed to save policy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update global config in memory
	globalConfig.Policy = policy

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	for {
		var msg ChatRequest
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		// 实时处理消息
		response := processMessage(msg.Message, msg.SessionID)

		if err := conn.WriteJSON(response); err != nil {
			break
		}
	}
}

func processMessage(message, sessionID string) ChatResponse {
	// 创建执行上下文
	cwd, _ := os.Getwd()
	workspace := filepath.Join(cwd, "workspace")

	// 使用加载的配置中的策略，而不是默认策略
	configMutex.RLock()
	policy := globalConfig.Policy
	configMutex.RUnlock()

	if !policy.Loaded {
		policy = agent.DefaultToolPolicy()
	}

	ctx := agent.ExecContext{
		Workspace: workspace,
		Policy:    policy,
	}

	// 执行AI处理
	results := agent.ExecuteCalls(ctx, []agent.ExecCall{
		{
			Tool:    "llm.chat",
			ArgsRaw: fmt.Sprintf(`{"message":"%s","session_id":"%s"}`, message, sessionID),
		},
	}, nil)

	// 构建响应
	var response ChatResponse
	if len(results) > 0 && results[0].OK {
		response = ChatResponse{
			Type:      "assistant",
			Content:   results[0].Output,
			Timestamp: time.Now().Format(time.RFC3339),
		}
	} else {
		response = ChatResponse{
			Type:      "error",
			Content:   "Failed to process message",
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}

	return response
}
