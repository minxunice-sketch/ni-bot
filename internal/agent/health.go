package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type HealthMonitor struct {
	mu             sync.RWMutex
	server         *http.Server
	startTime      time.Time
	uptime         time.Duration
	totalSessions  int
	activeSessions int
	messageCount   int
	toolCallCount  int
	approvalCount  int
	denialCount    int
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
}

func NewHealthMonitor(port int) *HealthMonitor {
	monitor := &HealthMonitor{
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}
	
	monitor.wg.Add(1)
	go monitor.updateUptime()
	
	if port > 0 {
		monitor.startHTTPServer(port)
	}
	
	return monitor
}

func (m *HealthMonitor) updateUptime() {
	defer m.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			m.uptime = time.Since(m.startTime)
			m.mu.Unlock()
		case <-m.stopCh:
			return
		}
	}
}

func (m *HealthMonitor) startHTTPServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", m.healthHandler)
	mux.HandleFunc("/metrics", m.metricsHandler)
	mux.HandleFunc("/stats", m.statsHandler)
	
	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	go func() {
		log.Printf("Health monitor server starting on port %d", port)
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health monitor server error: %v", err)
		}
	}()
}

func (m *HealthMonitor) healthHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := map[string]interface{}{
		"status":    "healthy",
		"uptime":    m.uptime.String(),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (m *HealthMonitor) metricsHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptimeMinutes := m.uptime.Minutes()
	messagesPerMinute := 0.0
	toolCallsPerMinute := 0.0
	if uptimeMinutes > 0 {
		messagesPerMinute = float64(m.messageCount) / uptimeMinutes
		toolCallsPerMinute = float64(m.toolCallCount) / uptimeMinutes
	}
	approvalDenialTotal := m.approvalCount + m.denialCount
	approvalRate := 0.0
	if approvalDenialTotal > 0 {
		approvalRate = float64(m.approvalCount) / float64(approvalDenialTotal)
	}
	
	metrics := map[string]interface{}{
		"uptime_seconds":          m.uptime.Seconds(),
		"total_sessions":         m.totalSessions,
		"active_sessions":        m.activeSessions,
		"total_messages":         m.messageCount,
		"total_tool_calls":       m.toolCallCount,
		"total_approvals":        m.approvalCount,
		"total_denials":          m.denialCount,
		"messages_per_minute":    messagesPerMinute,
		"tool_calls_per_minute":  toolCallsPerMinute,
		"approval_rate":          approvalRate,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (m *HealthMonitor) statsHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptimeMinutes := m.uptime.Minutes()
	messagesPerMinute := 0.0
	toolCallsPerMinute := 0.0
	if uptimeMinutes > 0 {
		messagesPerMinute = float64(m.messageCount) / uptimeMinutes
		toolCallsPerMinute = float64(m.toolCallCount) / uptimeMinutes
	}
	approvalDenialTotal := m.approvalCount + m.denialCount
	approvalRatePercent := 0.0
	if approvalDenialTotal > 0 {
		approvalRatePercent = 100 * float64(m.approvalCount) / float64(approvalDenialTotal)
	}
	
	stats := map[string]interface{}{
		"start_time":     m.startTime.Format(time.RFC3339),
		"uptime":         m.uptime.String(),
		"sessions": map[string]interface{}{
			"total":  m.totalSessions,
			"active": m.activeSessions,
		},
		"messages":       m.messageCount,
		"tool_calls":     m.toolCallCount,
		"approvals":      m.approvalCount,
		"denials":        m.denialCount,
		"rates": map[string]interface{}{
			"messages_per_minute":   fmt.Sprintf("%.2f", messagesPerMinute),
			"tool_calls_per_minute": fmt.Sprintf("%.2f", toolCallsPerMinute),
			"approval_rate":         fmt.Sprintf("%.1f%%", approvalRatePercent),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (m *HealthMonitor) Shutdown() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m.server.Shutdown(ctx)
	}
	m.wg.Wait()
}

// Session tracking methods
func (m *HealthMonitor) SessionStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalSessions++
	m.activeSessions++
}

func (m *HealthMonitor) SessionEnded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeSessions = max(0, m.activeSessions-1)
}

func (m *HealthMonitor) MessageProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messageCount++
}

func (m *HealthMonitor) ToolCallExecuted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolCallCount++
}

func (m *HealthMonitor) ApprovalRecorded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.approvalCount++
}

func (m *HealthMonitor) DenialRecorded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.denialCount++
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
