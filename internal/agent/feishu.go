package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	sessions   map[string]*feishuUserSession
	mu         sync.RWMutex
	cancel     context.CancelFunc
	sem        chan struct{}
	httpServer *http.Server
}

// FeishuMessage 飞书消息结构
// 参考: https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive
// 参考: https://open.feishu.cn/document/ukTMukTMukTM/uYDNxYjL2QTM24iN0EjN/event-subscription

type FeishuMessageEvent struct {
	Schema string            `json:"schema"`
	Header FeishuEventHeader `json:"header"`
	Event  FeishuEventData   `json:"event"`
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
	UserID  string `json:"user_id"`
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
}

type FeishuMessage struct {
	MessageID   string `json:"message_id"`
	RootID      string `json:"root_id"`
	ParentID    string `json:"parent_id"`
	CreateTime  string `json:"create_time"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

// FeishuTextContent 文本消息内容
type FeishuTextContent struct {
	Text string `json:"text"`
}

// FeishuAPIResponse 飞书API响应
type FeishuAPIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func NewFeishuConfig() *FeishuConfig {
	config := &FeishuConfig{
		Enabled:           parseBool(os.Getenv("NIBOT_ENABLE_FEISHU"), false) || strings.TrimSpace(os.Getenv("FEISHU_APP_ID")) != "",
		AppID:             os.Getenv("FEISHU_APP_ID"),
		AppSecret:         os.Getenv("FEISHU_APP_SECRET"),
		VerificationToken: os.Getenv("FEISHU_VERIFICATION_TOKEN"),
		EncryptKey:        os.Getenv("FEISHU_ENCRYPT_KEY"),
		WebhookURL:        os.Getenv("FEISHU_WEBHOOK_URL"),
		Timeout:           30 * time.Second,
		MaxConcurrent:     10,
		Debug:             parseBool(os.Getenv("FEISHU_DEBUG"), false),
	}

	// 解析超时设置
	if timeoutStr := os.Getenv("FEISHU_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			if timeout < 1 {
				timeout = 1
			}
			if timeout > 600 {
				timeout = 600
			}
			config.Timeout = time.Duration(timeout) * time.Second
		}
	}

	// 解析并发数设置
	if concurrentStr := os.Getenv("FEISHU_MAX_CONCURRENT"); concurrentStr != "" {
		if concurrent, err := strconv.Atoi(concurrentStr); err == nil {
			if concurrent < 1 {
				concurrent = 1
			}
			if concurrent > 200 {
				concurrent = 200
			}
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

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	fb.cancel = cancel

	go func() {
		<-ctx.Done()
		fb.Stop()
	}()

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
	return fb.startHTTPServer(ctx)
}

func (fb *FeishuBot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	const defaultMaxBody = int64(1024 * 1024)
	maxBody := defaultMaxBody
	if v := strings.TrimSpace(os.Getenv("FEISHU_MAX_BODY_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			if n < 4096 {
				n = 4096
			}
			if n > 10*1024*1024 {
				n = 10 * 1024 * 1024
			}
			maxBody = n
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var verifyReq struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Header    struct {
			Token string `json:"token"`
		} `json:"header"`
	}
	_ = json.Unmarshal(body, &verifyReq)

	if fb.config.VerificationToken != "" {
		h := r.Header.Get("X-Feishu-Token")
		if h != fb.config.VerificationToken && verifyReq.Token != fb.config.VerificationToken && verifyReq.Header.Token != fb.config.VerificationToken {
			http.Error(w, "Invalid verification token", http.StatusUnauthorized)
			return
		}
	}

	if verifyReq.Type == "url_verification" && strings.TrimSpace(verifyReq.Challenge) != "" {
		fb.handleURLVerification(verifyReq.Challenge, w)
		return
	}

	var event FeishuMessageEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	switch event.Header.EventType {
	case "im.message.receive_v1":
		fb.handleMessageReceive(event, w)
	default:
		fb.handleOtherEvent(event, w)
	}
}

func (fb *FeishuBot) handleMessageReceive(event FeishuMessageEvent, w http.ResponseWriter) {
	// 获取消息内容
	message := event.Event.Message
	userID := strings.TrimSpace(event.Event.Sender.SenderID.UserID)
	if userID == "" {
		userID = strings.TrimSpace(event.Event.Sender.SenderID.OpenID)
	}
	if userID == "" {
		userID = strings.TrimSpace(event.Event.Sender.SenderID.UnionID)
	}
	if userID == "" {
		http.Error(w, "Missing sender id", http.StatusBadRequest)
		return
	}

	if strings.ToLower(strings.TrimSpace(message.MessageType)) != "text" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(FeishuAPIResponse{Code: 0, Msg: "ignored"})
		return
	}

	// 解析文本内容
	var textContent FeishuTextContent
	if err := json.Unmarshal([]byte(message.Content), &textContent); err != nil {
		log.Printf("Failed to parse message content: %v", err)
		http.Error(w, "Invalid message content", http.StatusBadRequest)
		return
	}
	textContent.Text = strings.TrimSpace(textContent.Text)
	if textContent.Text == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(FeishuAPIResponse{Code: 0, Msg: "empty"})
		return
	}

	// 处理消息（使用限流器）
	select {
	case fb.sem <- struct{}{}:
		go func() {
			defer func() { <-fb.sem }()
			response, err := fb.processMessage(userID, textContent.Text, message.MessageID)
			if err != nil {
				log.Printf("Failed to process message: %v", err)
				return
			}
			if err := fb.sendReply(message.ChatID, message.MessageID, response); err != nil {
				log.Printf("Failed to send reply: %v", err)
				return
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(FeishuAPIResponse{Code: 0, Msg: "success"})

	default:
		http.Error(w, "Too many concurrent requests", http.StatusTooManyRequests)
	}
}

func (fb *FeishuBot) handleURLVerification(challenge string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"challenge": challenge,
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
	c := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	c = strings.TrimSpace(c)
	if len(c) > 500 {
		c = c[:500]
	}
	log.Printf("Sending reply to chat %s message %s: %s", chatID, messageID, c)
	if strings.TrimSpace(fb.config.WebhookURL) == "" {
		return nil
	}

	payload := map[string]any{
		"msg_type": "text",
		"content": map[string]any{
			"text": c,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(fb.config.WebhookURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: fb.config.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("feishu webhook reply failed: %s", msg)
	}
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
	if fb.cancel != nil {
		fb.cancel()
	}
	if fb.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fb.httpServer.Shutdown(ctx)
	}

	log.Println("Feishu bot stopped")
}
