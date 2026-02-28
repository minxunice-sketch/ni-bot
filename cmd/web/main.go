package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"nibot/internal/agent"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatRequest struct {
	Message string `json:"message"`
	SessionID string `json:"session_id"`
}

type ChatResponse struct {
	Type string `json:"type"`
	Content string `json:"content"`
	Timestamp string `json:"timestamp"`
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

	// 设置静态文件服务
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API路由
	http.HandleFunc("/api/chat", chatHandler)
	http.HandleFunc("/ws", websocketHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/templates/index.html")
	})

	log.Printf("🚀 Ni Bot Web Interface started on http://localhost:%s", port)
	log.Printf("   Open http://localhost:%s in your browser to start chatting", port)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Failed to start server:", err)
	}
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
	
	ctx := agent.ExecContext{
		Workspace: workspace,
		Policy:    agent.DefaultToolPolicy(),
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