package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type SessionState struct {
	SessionID    string    `json:"session_id"`
	StartTime    time.Time `json:"start_time"`
	LastActivity time.Time `json:"last_activity"`
	MessageCount int       `json:"message_count"`
	ToolCalls    int       `json:"tool_calls"`
	Approvals    int       `json:"approvals"`
	Denials      int       `json:"denials"`
	CurrentTask  string    `json:"current_task"`
	Memory       []string  `json:"memory"`
	mu           sync.Mutex
}

type SessionManager struct {
	workspace     string
	current       *SessionState
	persistDir    string
	healthMonitor *HealthMonitor
	store         *SQLiteStore
	mu            sync.RWMutex
}

func NewSessionManager(workspace string, healthMonitor *HealthMonitor) *SessionManager {
	persistDir := filepath.Join(workspace, "data", "sessions")
	os.MkdirAll(persistDir, 0755)

	store, err := OpenSQLiteStore(workspace)
	if err != nil {
		log.Printf("Failed to open SQLite store: %v", err)
		store = nil
	}

	return &SessionManager{
		workspace:     workspace,
		persistDir:    persistDir,
		healthMonitor: healthMonitor,
		store:         store,
	}
}

func (sm *SessionManager) StartNewSession() *SessionState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sessionID := fmt.Sprintf("session_%s", time.Now().Format("20060102_150405"))
	
	state := &SessionState{
		SessionID:    sessionID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		MessageCount: 0,
		ToolCalls:    0,
		Approvals:    0,
		Denials:      0,
		Memory:       []string{},
	}
	
	sm.current = state
	
	// Notify health monitor about new session
	if sm.healthMonitor != nil {
		sm.healthMonitor.SessionStarted()
	}
	
	// Auto-save the session state
	go sm.PersistSession(state)
	
	return state
}

func (sm *SessionManager) GetCurrentSession() *SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

func (sm *SessionManager) UpdateSession(updateFunc func(*SessionState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.current != nil {
		sm.current.mu.Lock()
		updateFunc(sm.current)
		sm.current.LastActivity = time.Now()
		sm.current.mu.Unlock()
		
		// Auto-save on significant updates
		go sm.PersistSession(sm.current)
	}
}

func (sm *SessionManager) PersistSession(state *SessionState) error {
	if state == nil {
		return fmt.Errorf("no session to persist")
	}
	
	state.mu.Lock()
	defer state.mu.Unlock()
	
	filePath := filepath.Join(sm.persistDir, state.SessionID+".json")
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %v", err)
	}
	
	// Write to temporary file first for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %v", err)
	}
	
	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("failed to rename session file: %v", err)
	}

	if sm.store != nil {
		_ = sm.store.UpsertSession(state)
	}
	
	return nil
}

func (sm *SessionManager) LoadSession(sessionID string) (*SessionState, error) {
	filePath := filepath.Join(sm.persistDir, sessionID+".json")
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %v", err)
	}
	
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session state: %v", err)
	}
	
	return &state, nil
}

func (sm *SessionManager) ListSessions() ([]SessionInfo, error) {
	files, err := os.ReadDir(sm.persistDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %v", err)
	}
	
	var sessions []SessionInfo
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		sessionID := strings.TrimSuffix(file.Name(), ".json")
		
		// Get file info for timestamps
		info, err := file.Info()
		if err != nil {
			continue
		}
		
		sessions = append(sessions, SessionInfo{
			SessionID:   sessionID,
			Modified:    info.ModTime(),
			Size:        info.Size(),
		})
	}
	
	// Sort by modification time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})
	
	return sessions, nil
}

type SessionInfo struct {
	SessionID string    `json:"session_id"`
	Modified  time.Time `json:"modified"`
	Size      int64     `json:"size"`
}

// Helper methods for common session updates
func (sm *SessionManager) IncrementMessageCount() {
	sm.UpdateSession(func(s *SessionState) {
		s.MessageCount++
	})
	if sm.healthMonitor != nil {
		sm.healthMonitor.MessageProcessed()
	}
}

func (sm *SessionManager) IncrementToolCalls() {
	sm.UpdateSession(func(s *SessionState) {
		s.ToolCalls++
	})
	if sm.healthMonitor != nil {
		sm.healthMonitor.ToolCallExecuted()
	}
}

func (sm *SessionManager) IncrementApprovals() {
	sm.UpdateSession(func(s *SessionState) {
		s.Approvals++
	})
	if sm.healthMonitor != nil {
		sm.healthMonitor.ApprovalRecorded()
	}
}

func (sm *SessionManager) IncrementDenials() {
	sm.UpdateSession(func(s *SessionState) {
		s.Denials++
	})
	if sm.healthMonitor != nil {
		sm.healthMonitor.DenialRecorded()
	}
}

func (sm *SessionManager) SessionEnded() {
	if sm.healthMonitor != nil {
		sm.healthMonitor.SessionEnded()
	}
	if sm.store != nil {
		sm.store.Close()
		sm.store = nil
	}
}

func (sm *SessionManager) SetCurrentTask(task string) {
	sm.UpdateSession(func(s *SessionState) {
		s.CurrentTask = task
	})
}

func (sm *SessionManager) AddToMemory(memoryItem string) {
	sm.UpdateSession(func(s *SessionState) {
		s.Memory = append(s.Memory, memoryItem)
		// Keep only last 100 memory items
		if len(s.Memory) > 100 {
			s.Memory = s.Memory[1:]
		}
	})
}

func (sm *SessionManager) RecordMessage(role, content string) {
	if sm == nil || sm.store == nil {
		return
	}
	s := sm.GetCurrentSession()
	if s == nil {
		return
	}
	_ = sm.store.InsertMessage(s.SessionID, role, content)
}

func (sm *SessionManager) RecordToolResults(calls []ExecCall, results []ToolResult) {
	if sm == nil || sm.store == nil {
		return
	}
	s := sm.GetCurrentSession()
	if s == nil {
		return
	}
	_ = sm.store.InsertToolAudit(s.SessionID, calls, results)
}
