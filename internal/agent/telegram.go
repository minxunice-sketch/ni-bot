package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramConfig struct {
	Enabled           bool
	BotToken         string
	AllowedUserIDs   []int64
	ProxyURL         string
	Timeout          time.Duration
	MaxConcurrent    int
	LongPollingTimeout int
	Debug            bool
}

type telegramUserSession struct {
	sessionManager *SessionManager
	client         *LLMClient
}

type TelegramBot struct {
	bot           *tgbotapi.BotAPI
	config        *TelegramConfig
	cfg           Config
	workspace     string
	systemPrompt  string
	healthMonitor *HealthMonitor

	sessions map[int64]*telegramUserSession
	mu       sync.RWMutex
	cancel   context.CancelFunc
	sem      chan struct{}
}

func NewTelegramConfig() *TelegramConfig {
	config := &TelegramConfig{
		Enabled:           os.Getenv("NIBOT_ENABLE_TELEGRAM") == "true" || os.Getenv("TELEGRAM_BOT_TOKEN") != "",
		BotToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		ProxyURL:         os.Getenv("TELEGRAM_PROXY_URL"),
		Timeout:          30 * time.Second,
		MaxConcurrent:    10,
		LongPollingTimeout: 60,
		Debug:           os.Getenv("TELEGRAM_DEBUG") == "true",
	}

	// 解析超时设置
	if timeoutStr := os.Getenv("TELEGRAM_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.Timeout = time.Duration(timeout) * time.Second
		}
	}

	// 解析并发数设置
	if concurrentStr := os.Getenv("TELEGRAM_MAX_CONCURRENT"); concurrentStr != "" {
		if concurrent, err := strconv.Atoi(concurrentStr); err == nil {
			config.MaxConcurrent = concurrent
		}
	}

	// 解析长轮询超时
	if pollingStr := os.Getenv("TELEGRAM_LONG_POLLING_TIMEOUT"); pollingStr != "" {
		if polling, err := strconv.Atoi(pollingStr); err == nil {
			config.LongPollingTimeout = polling
		}
	}

	// 解析允许的用户ID
	if userIDsStr := os.Getenv("TELEGRAM_ALLOWED_USER_IDS"); userIDsStr != "" {
		userIDs := strings.Split(userIDsStr, ",")
		for _, idStr := range userIDs {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				config.AllowedUserIDs = append(config.AllowedUserIDs, id)
			}
		}
	}

	return config
}

func NewTelegramBot(telegramCfg *TelegramConfig, cfg Config, workspace string, systemPrompt string, healthMonitor *HealthMonitor) (*TelegramBot, error) {
	if telegramCfg == nil {
		return nil, fmt.Errorf("telegram config is required")
	}
	if strings.TrimSpace(telegramCfg.BotToken) == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	bot, err := tgbotapi.NewBotAPI(strings.TrimSpace(telegramCfg.BotToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %v", err)
	}

	bot.Debug = telegramCfg.Debug

	maxConcurrent := telegramCfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	return &TelegramBot{
		bot:           bot,
		config:        telegramCfg,
		cfg:           cfg,
		workspace:     workspace,
		systemPrompt:  systemPrompt,
		healthMonitor: healthMonitor,
		sessions:      make(map[int64]*telegramUserSession),
		sem:           make(chan struct{}, maxConcurrent),
	}, nil
}

func (tb *TelegramBot) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	tb.mu.Lock()
	tb.cancel = cancel
	tb.mu.Unlock()

	log.Printf("Starting Telegram bot @%s", tb.bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = tb.config.LongPollingTimeout

	updates := tb.bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram bot stopped")
			return nil
		case update := <-updates:
			tb.sem <- struct{}{}
			go func(u tgbotapi.Update) {
				defer func() { <-tb.sem }()
				tb.handleUpdate(u)
			}(update)
		}
	}
}

func (tb *TelegramBot) Stop() {
	tb.mu.Lock()
	cancel := tb.cancel
	tb.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (tb *TelegramBot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	if !tb.isUserAllowed(userID) {
		tb.sendMessage(chatID, "抱歉，您没有权限使用此机器人")
		return
	}

	if strings.HasPrefix(text, "/") {
		tb.handleCommand(userID, chatID, text)
		return
	}

	session := tb.getUserSession(userID)
	response, err := tb.chatWithTools(session, text)
	if err != nil {
		log.Printf("Error processing message: %v", err)
		tb.sendMessage(chatID, "处理消息时发生错误")
		return
	}

	tb.sendMessage(chatID, response)
}

func (tb *TelegramBot) isUserAllowed(userID int64) bool {
	if len(tb.config.AllowedUserIDs) == 0 {
		return true
	}

	for _, allowedID := range tb.config.AllowedUserIDs {
		if userID == allowedID {
			return true
		}
	}
	return false
}

func (tb *TelegramBot) getUserSession(userID int64) *telegramUserSession {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if session, exists := tb.sessions[userID]; exists {
		return session
	}

	sessionManager := NewSessionManager(tb.workspace, tb.healthMonitor)
	sessionManager.StartNewSession()
	client := NewLLMClient(tb.cfg, tb.workspace, tb.systemPrompt, sessionManager)

	us := &telegramUserSession{
		sessionManager: sessionManager,
		client:         client,
	}
	tb.sessions[userID] = us
	return us
}

func (tb *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := tb.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (tb *TelegramBot) handleCommand(userID int64, chatID int64, text string) {
	cmd := strings.Fields(text)
	if len(cmd) == 0 {
		return
	}

	switch cmd[0] {
	case "/start":
		tb.sendMessage(chatID, "欢迎使用 Ni Bot！\n\n直接发送消息即可开始对话。\n\n可用命令：\n/help\n/skills\n/reset\n/clear\n/reload")
	case "/help":
		tb.sendMessage(chatID, "用法：\n- 直接发送消息与 Ni Bot 对话\n- /skills 查看技能\n- /reset 重置该用户会话\n- /reload 重新加载 System Prompt\n- /clear 清屏")
	case "/clear":
		tb.sendMessage(chatID, strings.Repeat("\n", 40))
	case "/reset":
		tb.resetUserSession(userID)
		tb.sendMessage(chatID, "已重置会话")
	case "/reload":
		if err := tb.reloadSystemPrompt(); err != nil {
			tb.sendMessage(chatID, "重新加载失败："+err.Error())
			return
		}
		tb.sendMessage(chatID, "已重新加载 System Prompt")
	case "/skills":
		tb.sendMessage(chatID, tb.formatSkills())
	default:
		tb.sendMessage(chatID, "未知命令，请输入 /help 查看帮助")
	}
}

func (tb *TelegramBot) resetUserSession(userID int64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if us, ok := tb.sessions[userID]; ok {
		if us.sessionManager != nil {
			us.sessionManager.SessionEnded()
		}
		delete(tb.sessions, userID)
	}
}

func (tb *TelegramBot) reloadSystemPrompt() error {
	p, err := ConstructSystemPrompt(tb.workspace)
	if err != nil {
		return err
	}

	tb.mu.Lock()
	tb.systemPrompt = p
	for _, us := range tb.sessions {
		if us == nil || us.client == nil {
			continue
		}
		us.client.mu.Lock()
		us.client.SystemMsg = p
		us.client.mu.Unlock()
	}
	tb.mu.Unlock()

	return nil
}

func (tb *TelegramBot) formatSkills() string {
	skills, err := DiscoverSkills(tb.workspace)
	if err != nil {
		return "读取技能失败：" + err.Error()
	}
	if len(skills) == 0 {
		return "当前未发现技能（workspace/skills/ 下为空或不可读）。"
	}
	var b strings.Builder
	b.WriteString("Skills:\n")
	for _, s := range skills {
		line := "- " + s.Name
		if strings.TrimSpace(s.Description) != "" {
			line += " — " + strings.TrimSpace(s.Description)
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimSpace(b.String())
}

func (tb *TelegramBot) chatWithTools(us *telegramUserSession, text string) (string, error) {
	if us == nil || us.client == nil {
		return "", fmt.Errorf("session not initialized")
	}

	if us.sessionManager != nil {
		us.sessionManager.IncrementMessageCount()
		us.sessionManager.SetCurrentTask(text)
		us.sessionManager.RecordMessage("user", text)
	}

	resp, err := us.client.Chat(text)
	if err != nil {
		return "", err
	}

	for i := 0; i < us.client.MaxToolIters; i++ {
		calls := ExtractExecCalls(resp)
		if len(calls) == 0 {
			break
		}
		if us.sessionManager != nil {
			for range calls {
				us.sessionManager.IncrementToolCalls()
			}
		}

		results := ExecuteCalls(ExecContext{Workspace: tb.workspace, Policy: tb.cfg.Policy}, calls, nil)
		if us.sessionManager != nil {
			us.sessionManager.RecordToolResults(calls, results)
		}

		toolMsg := formatToolResults(results)
		resp, err = us.client.Chat(toolMsg)
		if err != nil {
			return "", err
		}
	}

	if us.sessionManager != nil {
		us.sessionManager.RecordMessage("assistant", resp)
	}
	return resp, nil
}
