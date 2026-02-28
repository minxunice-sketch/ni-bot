package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FeishuConfig struct {
	Enabled           bool
	AppID             string
	AppSecret         string
	VerificationToken string
	EncryptKey        string
	WebhookURL        string
	Timeout           time.Duration
	MaxConcurrent     int
	Debug             bool
}

type feishuUserSession struct {
	sessionManager *SessionManager
	client         *LLMClient
}

type FeishuBot struct {
	config        *FeishuConfig
	cfg           Config
	workspace     string
	systemPrompt  string
	healthMonitor *HealthMonitor
	
	sessions      map[string]*feishuUserSession
	mu            sync.RWMutex
	cancel        context.CancelFunc
	sem           chan struct{}
	httpServer    *http.Server
}

// FeishuMessage 飞书消息结构
// 参考: https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive
// 参考: https://open.feishu.cn/document/ukTMukTMukTM/uYDNxYjL2QTM24iN0EjN/event-subscription

type FeishuMessageEvent struct {
	Schema string          `json:"schema"`
	Header FeishuEventHeader `json:"header"`
	Event  FeishuEventData  `json:"event"`
}

type FeishuEventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

type FeishuEventData struct {
	Sender  FeishuSender  `json:"sender"`
	Message FeishuMessage `json:"message"`
}

type FeishuSender struct {
	SenderID   FeishuSenderID `json:"sender_id"`
	SenderType string         `json:"sender_type"`
	TenantKey  string         `json:"tenant_key"`
}

type FeishuSenderID struct {
	UserID string `json:"user_id"`
	OpenID string `json:"open_id"`
	UnionID string `json:"union_id"`
}

type FeishuMessage struct {
	MessageID   string          `json:"message_id"`
	RootID      string          `json:"root_id"`
	ParentID    string          `json:"parent_id"`
	CreateTime  string          `json:"create_time"`
	ChatID      string          `json:"chat_id"`
	ChatType    string          `json:"chat_type"`
	MessageType string          `json:"message_type"`
	Content     string          `json:"content"`
}

// FeishuTextContent 文本消息内容
type FeishuTextContent struct {
	Text string `json:"text"`
}

// FeishuAPIResponse 飞书API响应
type FeishuAPIResponse struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data"`
}

func NewFeishuConfig() *FeishuConfig {
	config := &FeishuConfig{
		Enabled:           os.Getenv("NIBOT_ENABLE_FEISHU") == "true" || os.Getenv("FEISHU_APP_ID") != "",
		AppID:            os.Getenv("FEISHU_APP_ID"),
		AppSecret:        os.Getenv("FEISHU_APP_SECRET"),
		VerificationToken: os.Getenv("FEISHU_VERIFICATION_TOKEN"),
		EncryptKey:       os.Getenv("FEISHU_ENCRYPT_KEY"),
		WebhookURL:       os.Getenv("FEISHU_WEBHOOK_URL"),
		Timeout:          30 * time.Second,
		MaxConcurrent:    10,
		Debug:           os.Getenv("FEISHU_DEBUG") == "true",
	}

	// 解析超时设置
	if timeoutStr := os.Getenv("FEISHU_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.Timeout = time.Duration(timeout) * time.Second
		}
	}

	// 解析并发数设置
	if concurrentStr := os.Getenv("FEISHU_MAX_CONCURRENT"); concurrentStr != "" {
		if concurrent, err := strconv.Atoi(concurrentStr); err == nil {
			config.MaxConcurrent = concurrent
		}
	}

	return config
}

func NewFeishuBot(feishuCfg *FeishuConfig, cfg Config, workspace string, systemPrompt string, healthMonitor *HealthMonitor) (*FeishuBot, error) {
	if feishuCfg == nil {
		return nil, fmt.Errorf("feishu config is required")
	}
	if strings.TrimSpace(feishuCfg.AppID) == "" {
		return nil, fmt.Errorf("feishu app id is required")
	}
	if strings.TrimSpace(feishuCfg.AppSecret) == "" {
		return nil, fmt.Errorf("feishu app secret is required")
	}

	maxConcurrent := feishuCfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	return &FeishuBot{
		config:        feishuCfg,
		cfg:           cfg,
		workspace:     workspace,
		systemPrompt:  systemPrompt,
		healthMonitor: healthMonitor,
		sessions:      make(map[string]*feishuUserSession),
		sem:           make(chan struct{}, maxConcurrent),
	}, nil
}

func (fb *FeishuBot) Start(ctx context.Context) error {
	log.Printf("Starting Feishu bot with AppID: %s", fb.config.AppID)

	// 验证配置
	if err := fb.validateConfig(); err != nil {
		return fmt.Errorf("feishu config validation failed: %v", err)
	}

	// 启动HTTP服务器接收飞书消息
	if fb.config.WebhookURL == "" {
		return fb.startHTTPServer(ctx)
	}

	// 如果配置了Webhook URL，则使用主动推送模式
	return fb.startWebhookMode(ctx)
}

func (fb *FeishuBot) validateConfig() error {
	if fb.config.AppID == "" {
		return fmt.Errorf("FEISHU_APP_ID is required")
	}
	if fb.config.AppSecret == "" {
		return fmt.Errorf("FEISHU_APP_SECRET is required")
	}
	return nil
}

func (fb *FeishuBot) startHTTPServer(ctx context.Context) error {
	// 设置HTTP服务器配置
	port := os.Getenv("FEISHU_HTTP_PORT")
	if port == "" {
		port = "8081"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feishu/webhook", fb.handleWebhook)
	mux.HandleFunc("/feishu/health", fb.handleHealthCheck)

	fb.httpServer = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Feishu HTTP server starting on port %s", port)
	
	go func() {
		if err := fb.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Feishu HTTP server error: %v", err)
		}
	}()

	return nil
}

func (fb *FeishuBot) startWebhookMode(ctx context.Context) error {
	log.Printf("Starting Feishu bot in webhook mode with URL: %s", fb.config.WebhookURL)
	// 实现webhook模式的定期检查或长连接
	return nil
}

func (fb *FeishuBot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// 验证请求（飞书验证token）
	if !fb.verifyRequest(r) {
		http.Error(w, "Invalid verification token", http.StatusUnauthorized)
		return
	}

	// 解析飞书消息
	var event FeishuMessageEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// 处理不同类型的事件
	switch event.Header.EventType {
	case "im.message.receive_v1":
		fb.handleMessageReceive(event, w)
	case "url_verification":
		fb.handleURLVerification(event, w)
	default:
		fb.handleOtherEvent(event, w)
	}
}

func (fb *FeishuBot) verifyRequest(r *http.Request) bool {
	// 简单的token验证（实际应该使用更安全的验证方式）
	token := r.Header.Get("X-Feishu-Token")
	return token == fb.config.VerificationToken || fb.config.VerificationToken == ""
}

func (fb *FeishuBot) handleMessageReceive(event FeishuMessageEvent, w http.ResponseWriter) {
	// 获取消息内容
	message := event.Event.Message
	userID := event.Event.Sender.SenderID.UserID

	// 解析文本内容
	var textContent FeishuTextContent
	if err := json.Unmarshal([]byte(message.Content), &textContent); err != nil {
		log.Printf("Failed to parse message content: %v", err)
		http.Error(w, "Invalid message content", http.StatusBadRequest)
		return
	}

	// 处理消息（使用限流器）
	select {
	case fb.sem <- struct{}{}:
		defer func() { <-fb.sem }()
		
		response, err := fb.processMessage(userID, textContent.Text, message.MessageID)
		if err != nil {
			log.Printf("Failed to process message: %v", err)
			http.Error(w, "Message processing failed", http.StatusInternalServerError)
			return
		}

		// 发送回复
		if err := fb.sendReply(message.ChatID, message.MessageID, response); err != nil {
			log.Printf("Failed to send reply: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FeishuAPIResponse{Code: 0, Msg: "success"})
	
	default:
		http.Error(w, "Too many concurrent requests", http.StatusTooManyRequests)
	}
}

func (fb *FeishuBot) handleURLVerification(event FeishuMessageEvent, w http.ResponseWriter) {
	// 飞书URL验证处理
	w.Header().Set("Content-Type", "application/json")
	
	// 飞书URL验证需要返回特定的JSON格式
	// 直接从请求体中解析challenge字段
	var requestData struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// 解析JSON
	if err := json.Unmarshal(body, &requestData); err != nil {
		log.Printf("Failed to parse JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// 返回飞书要求的验证格式
	if requestData.Type == "url_verification" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"challenge": requestData.Challenge,
		})
		return
	}
	
	// 对于其他类型的事件，返回通用响应
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 0,
		"msg": "Event received",
		"data": nil,
	})
}

func (fb *FeishuBot) handleOtherEvent(event FeishuMessageEvent, w http.ResponseWriter) {
	// 处理其他类型的事件
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FeishuAPIResponse{Code: 0, Msg: "Event received"})
}

func (fb *FeishuBot) processMessage(userID, text, messageID string) (string, error) {
	// 获取用户会话
	session := fb.getUserSession(userID)
	
	// 处理特殊命令
	if strings.HasPrefix(text, "/") {
		return fb.handleCommand(userID, text, session)
	}

	// 使用LLM处理消息
	return fb.handleLLMMessage(userID, text, session)
}

func (fb *FeishuBot) getUserSession(userID string) *feishuUserSession {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if session, exists := fb.sessions[userID]; exists {
		return session
	}

	// 创建新会话
	sessionManager := NewSessionManager(fb.workspace, fb.healthMonitor)
	client := NewLLMClient(fb.cfg, fb.workspace, fb.systemPrompt, sessionManager)
	
	newSession := &feishuUserSession{
		sessionManager: sessionManager,
		client:         client,
	}
	
	fb.sessions[userID] = newSession
	return newSession
}

func (fb *FeishuBot) handleCommand(userID, command string, session *feishuUserSession) (string, error) {
	switch command {
	case "/help", "/start":
		return "🤖 Ni Bot 飞书版已启动！\n\n可用命令：\n/help - 显示帮助\n/skills - 查看可用技能\n/reset - 重置会话\n/reload - 重新加载配置\n/clear - 清除消息", nil
	case "/skills":
		return "🛠️ 可用技能：\n• 网页搜索 (/search)\n• 内容爬取 (/crawl)\n• 进化学习 (/evolve)\n• 文件操作 (/file)\n• 代码执行 (/code)", nil
	case "/reset":
		fb.mu.Lock()
		delete(fb.sessions, userID)
		fb.mu.Unlock()
		return "✅ 会话已重置", nil
	case "/reload":
		return "🔄 配置重载功能开发中", nil
	case "/clear":
		return "🧹 消息清除功能开发中", nil
	default:
		return "❌ 未知命令，请输入 /help 查看帮助", nil
	}
}

func (fb *FeishuBot) handleLLMMessage(userID, text string, session *feishuUserSession) (string, error) {
	// 使用LLM处理消息（这里需要实现具体的消息处理逻辑）
	// 暂时返回模拟响应
	return fmt.Sprintf("🤖 Ni Bot 收到消息：%s\n\n这是模拟响应，实际需要集成LLM处理", text), nil
}

func (fb *FeishuBot) sendReply(chatID, messageID, content string) error {
	// 实现飞书消息发送逻辑
	// 这里需要调用飞书API发送消息
	log.Printf("Sending reply to chat %s: %s", chatID, content)
	return nil
}

func (fb *FeishuBot) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"app_id":    fb.config.AppID,
	})
}

func (fb *FeishuBot) Stop() {
	if fb.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fb.httpServer.Shutdown(ctx)
	}
	
	if fb.cancel != nil {
		fb.cancel()
	}
	
	log.Println("Feishu bot stopped")
}