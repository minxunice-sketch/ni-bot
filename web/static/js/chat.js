class ChatApp {
    constructor() {
        this.socket = null;
        this.sessionId = this.generateSessionId();
        this.messageHistory = [];
        this.isConnected = false;
        this.currentLanguage = 'zh'; // 默认中文
        
        this.initializeElements();
        this.bindEvents();
        this.connectWebSocket();
        this.loadConversationHistory();
        this.updateLanguageUI();
    }

    initializeElements() {
        this.messageInput = document.getElementById('messageInput');
        this.sendButton = document.getElementById('sendButton');
        this.chatMessages = document.getElementById('chatMessages');
        this.statusDot = document.querySelector('.status-dot');
        this.statusText = document.querySelector('.status-text');
        this.languageButtons = document.querySelectorAll('.lang-btn');
    }

    bindEvents() {
        // 发送消息事件
        this.sendButton.addEventListener('click', () => this.sendMessage());
        
        // 输入框事件
        this.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
            
            // 历史消息导航
            if (e.key === 'ArrowUp') {
                e.preventDefault();
                this.navigateHistory(-1);
            } else if (e.key === 'ArrowDown') {
                e.preventDefault();
                this.navigateHistory(1);
            } else if (e.key === 'Escape') {
                e.preventDefault();
                this.messageInput.value = '';
            }
        });

        // 自动调整输入框高度
        this.messageInput.addEventListener('input', () => {
            this.adjustTextareaHeight();
        });

        // 页面可见性变化时重连
        document.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'visible' && !this.isConnected) {
                this.connectWebSocket();
            }
        });

        // 语言切换事件
        this.languageButtons.forEach(button => {
            button.addEventListener('click', () => {
                const lang = button.getAttribute('data-lang');
                this.switchLanguage(lang);
            });
        });
    }

    generateSessionId() {
        return 'session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    }

    connectWebSocket() {
        try {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;
            
            this.socket = new WebSocket(wsUrl);
            
            this.socket.onopen = () => {
                this.updateConnectionStatus(true);
                console.log('WebSocket connected');
            };

            this.socket.onmessage = (event) => {
                this.handleMessage(JSON.parse(event.data));
            };

            this.socket.onclose = () => {
                this.updateConnectionStatus(false);
                console.log('WebSocket disconnected');
                
                // 5秒后尝试重连
                setTimeout(() => this.connectWebSocket(), 5000);
            };

            this.socket.onerror = (error) => {
                console.error('WebSocket error:', error);
                this.updateConnectionStatus(false);
            };

        } catch (error) {
            console.error('Failed to connect WebSocket:', error);
            this.fallbackToHTTP();
        }
    }

    fallbackToHTTP() {
        console.log('Falling back to HTTP API');
        this.updateConnectionStatus(false, 'HTTP模式');
    }

    updateConnectionStatus(connected, text = null) {
        this.isConnected = connected;
        
        if (this.statusDot) {
            this.statusDot.style.background = connected ? '#4ade80' : '#ef4444';
        }
        
        if (this.statusText) {
            this.statusText.textContent = text || (connected ? '已连接' : '连接中...');
        }
    }

    async sendMessage() {
        const message = this.messageInput.value.trim();
        if (!message) return;

        // 添加到历史记录
        this.messageHistory.push(message);
        this.historyIndex = this.messageHistory.length;

        // 添加用户消息到界面
        this.addMessage('user', message);
        
        // 清空输入框
        this.messageInput.value = '';
        this.adjustTextareaHeight();
        
        // 显示正在输入指示器
        this.showTypingIndicator();

        try {
            if (this.isConnected && this.socket) {
                // WebSocket发送
                this.socket.send(JSON.stringify({
                    message: message,
                    session_id: this.sessionId
                }));
            } else {
                // HTTP回退
                const response = await fetch('/api/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        message: message,
                        session_id: this.sessionId
                    })
                });

                if (response.ok) {
                    const data = await response.json();
                    this.handleMessage(data);
                } else {
                    throw new Error('HTTP request failed');
                }
            }
        } catch (error) {
            console.error('Failed to send message:', error);
            this.addMessage('error', '发送消息失败，请检查网络连接');
            this.hideTypingIndicator();
        }
    }

    handleMessage(data) {
        this.hideTypingIndicator();
        
        if (data.type === 'assistant') {
            this.addMessage('assistant', data.content);
        } else if (data.type === 'error') {
            this.addMessage('error', data.content);
        }
    }

    addMessage(type, content) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${type}`;
        
        const avatar = document.createElement('div');
        avatar.className = 'message-avatar';
        avatar.innerHTML = type === 'user' ? '👤' : '🤖';
        
        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';
        
        // 处理代码块和格式化
        const formattedContent = this.formatMessage(content);
        contentDiv.innerHTML = formattedContent;
        
        messageDiv.appendChild(avatar);
        messageDiv.appendChild(contentDiv);
        
        this.chatMessages.appendChild(messageDiv);
        this.scrollToBottom();
        
        // 高亮代码块
        this.highlightCodeBlocks(contentDiv);
    }

    formatMessage(content) {
        // 简单的Markdown处理
        return content
            .replace(/\`\`\`([\s\S]*?)\`\`\`/g, '<pre><code>$1</code></pre>')
            .replace(/\`([^`]+)\`/g, '<code>$1</code>')
            .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
            .replace(/\*(.+?)\*/g, '<em>$1</em>')
            .replace(/\n/g, '<br>');
    }

    highlightCodeBlocks(container) {
        const codeBlocks = container.querySelectorAll('pre code');
        codeBlocks.forEach(block => {
            // 简单的语法高亮（可以集成Prism.js等库）
            const text = block.textContent;
            // 这里可以添加更复杂的高亮逻辑
            block.innerHTML = text;
        });
    }

    showTypingIndicator() {
        const indicator = document.createElement('div');
        indicator.className = 'message assistant';
        indicator.id = 'typing-indicator';
        
        const avatar = document.createElement('div');
        avatar.className = 'message-avatar';
        avatar.innerHTML = '🤖';
        
        const content = document.createElement('div');
        content.className = 'typing-indicator';
        content.innerHTML = `
            <div class="typing-dot"></div>
            <div class="typing-dot"></div>
            <div class="typing-dot"></div>
        `;
        
        indicator.appendChild(avatar);
        indicator.appendChild(content);
        this.chatMessages.appendChild(indicator);
        this.scrollToBottom();
    }

    hideTypingIndicator() {
        const indicator = document.getElementById('typing-indicator');
        if (indicator) {
            indicator.remove();
        }
    }

    scrollToBottom() {
        this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
    }

    adjustTextareaHeight() {
        this.messageInput.style.height = 'auto';
        this.messageInput.style.height = Math.min(this.messageInput.scrollHeight, 120) + 'px';
    }

    navigateHistory(direction) {
        if (this.messageHistory.length === 0) return;
        
        if (this.historyIndex === undefined) {
            this.historyIndex = this.messageHistory.length;
        }
        
        this.historyIndex += direction;
        
        if (this.historyIndex < 0) this.historyIndex = 0;
        if (this.historyIndex > this.messageHistory.length) this.historyIndex = this.messageHistory.length;
        
        if (this.historyIndex === this.messageHistory.length) {
            this.messageInput.value = '';
        } else {
            this.messageInput.value = this.messageHistory[this.historyIndex];
        }
        
        this.adjustTextareaHeight();
    }

    // 语言切换功能
    switchLanguage(lang) {
        if (this.currentLanguage === lang) return;
        
        this.currentLanguage = lang;
        
        // 更新语言按钮状态
        this.languageButtons.forEach(button => {
            if (button.getAttribute('data-lang') === lang) {
                button.classList.add('active');
            } else {
                button.classList.remove('active');
            }
        });
        
        // 更新界面文本
        this.updateLanguageUI();
        
        // 保存语言偏好
        localStorage.setItem('chat_language', lang);
    }

    // 更新界面文本内容
    updateLanguageUI() {
        // 更新所有带有 data-en 和 data-zh 属性的元素
        const elements = document.querySelectorAll('[data-en], [data-zh]');
        elements.forEach(element => {
            const text = element.getAttribute(`data-${this.currentLanguage}`);
            if (text) {
                element.textContent = text;
            }
        });

        // 更新连接状态文本
        if (this.statusText) {
            const statusTexts = {
                zh: { connected: '已连接', disconnected: '连接中...', http: 'HTTP模式' },
                en: { connected: 'Connected', disconnected: 'Connecting...', http: 'HTTP Mode' }
            };
            
            if (this.isConnected) {
                this.statusText.textContent = statusTexts[this.currentLanguage].connected;
            } else if (this.statusText.textContent.includes('HTTP')) {
                this.statusText.textContent = statusTexts[this.currentLanguage].http;
            } else {
                this.statusText.textContent = statusTexts[this.currentLanguage].disconnected;
            }
        }

        // 更新输入框占位符
        const placeholders = {
            zh: '输入消息...',
            en: 'Type a message...'
        };
        this.messageInput.placeholder = placeholders[this.currentLanguage];
    }

    // 加载对话历史
    loadConversationHistory() {
        try {
            const savedHistory = sessionStorage.getItem('chat_history');
            const savedLanguage = localStorage.getItem('chat_language');
            
            if (savedHistory) {
                const history = JSON.parse(savedHistory);
                history.forEach(msg => {
                    this.addMessage(msg.type, msg.content);
                });
            }
            
            if (savedLanguage) {
                this.switchLanguage(savedLanguage);
            }
        } catch (error) {
            console.error('Failed to load conversation history:', error);
        }
    }

    // 保存对话历史
    saveConversationHistory() {
        try {
            const messages = [];
            const messageElements = this.chatMessages.querySelectorAll('.message');
            
            messageElements.forEach(element => {
                const type = element.classList.contains('user') ? 'user' : 
                            element.classList.contains('assistant') ? 'assistant' : 'error';
                const content = element.querySelector('.message-content').textContent;
                messages.push({ type, content });
            });
            
            sessionStorage.setItem('chat_history', JSON.stringify(messages));
        } catch (error) {
            console.error('Failed to save conversation history:', error);
        }
    }

    // 重写 addMessage 方法以自动保存历史
    addMessage(type, content) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${type}`;
        
        const avatar = document.createElement('div');
        avatar.className = 'message-avatar';
        avatar.innerHTML = type === 'user' ? '👤' : '🤖';
        
        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';
        
        // 处理代码块和格式化
        const formattedContent = this.formatMessage(content);
        contentDiv.innerHTML = formattedContent;
        
        messageDiv.appendChild(avatar);
        messageDiv.appendChild(contentDiv);
        
        this.chatMessages.appendChild(messageDiv);
        this.scrollToBottom();
        
        // 高亮代码块
        this.highlightCodeBlocks(contentDiv);
        
        // 保存对话历史
        this.saveConversationHistory();
    }
}

// 页面加载完成后初始化
window.addEventListener('DOMContentLoaded', () => {
    new ChatApp();
});

// 服务Worker注册（可选）
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').catch(console.error);
}