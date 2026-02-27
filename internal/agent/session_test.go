package agent

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewSessionManager(tempDir, nil)

	// Test starting a new session
	session := manager.StartNewSession()
	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	if session.SessionID == "" {
		t.Error("Expected session ID to be set")
	}

	if session.MessageCount != 0 {
		t.Errorf("Expected initial message count 0, got %d", session.MessageCount)
	}

	// Test updating session
	manager.IncrementMessageCount()
	manager.IncrementToolCalls()
	manager.SetCurrentTask("test task")
	manager.AddToMemory("test memory item")

	// Verify updates
	current := manager.GetCurrentSession()
	if current.MessageCount != 1 {
		t.Errorf("Expected message count 1, got %d", current.MessageCount)
	}

	if current.ToolCalls != 1 {
		t.Errorf("Expected tool calls 1, got %d", current.ToolCalls)
	}

	if current.CurrentTask != "test task" {
		t.Errorf("Expected current task 'test task', got '%s'", current.CurrentTask)
	}

	if len(current.Memory) != 1 || current.Memory[0] != "test memory item" {
		t.Errorf("Expected memory to contain 'test memory item', got %v", current.Memory)
	}

	// Test persistence
	if err := manager.PersistSession(current); err != nil {
		t.Errorf("Failed to persist session: %v", err)
	}

	// Test loading session
	loaded, err := manager.LoadSession(current.SessionID)
	if err != nil {
		t.Errorf("Failed to load session: %v", err)
	}

	if loaded.SessionID != current.SessionID {
		t.Errorf("Loaded session ID mismatch: expected %s, got %s", current.SessionID, loaded.SessionID)
	}

	if loaded.MessageCount != 1 {
		t.Errorf("Loaded message count mismatch: expected 1, got %d", loaded.MessageCount)
	}

	// Test listing sessions
	sessions, err := manager.ListSessions()
	if err != nil {
		t.Errorf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].SessionID != current.SessionID {
		t.Errorf("Listed session ID mismatch: expected %s, got %s", current.SessionID, sessions[0].SessionID)
	}
}

func TestSessionManager_NoCurrentSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session_test_none")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewSessionManager(tempDir, nil)

	// Test getting current session when none exists
	if session := manager.GetCurrentSession(); session != nil {
		t.Error("Expected nil current session when none started")
	}

	// Test persisting nil session
	if err := manager.PersistSession(nil); err == nil {
		t.Error("Expected error when persisting nil session")
	}
}

func TestSessionManager_SQLiteStoreWrites(t *testing.T) {
	t.Setenv("NIBOT_STORAGE", "sqlite")

	tempDir, err := os.MkdirTemp("", "session_test_sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewSessionManager(tempDir, nil)
	s := manager.StartNewSession()
	manager.RecordMessage("user", "hello")
	if err := manager.PersistSession(s); err != nil {
		t.Fatalf("persist: %v", err)
	}
	manager.SessionEnded()

	dbPath := filepath.Join(tempDir, "data", "nibot.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow("select count(*) from sessions").Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 session, got %d", n)
	}
	if err := db.QueryRow("select count(*) from messages").Scan(&n); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 message, got %d", n)
	}
}

func TestSessionState_Concurrency(t *testing.T) {
	state := &SessionState{
		SessionID:    "test_concurrent",
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}

	// Test concurrent access
	done := make(chan bool)
	
	// Start multiple goroutines that update the state
	for i := 0; i < 10; i++ {
		go func(index int) {
			state.mu.Lock()
			state.MessageCount++
			state.mu.Unlock()
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	if state.MessageCount != 10 {
		t.Errorf("Expected message count 10 after concurrent updates, got %d", state.MessageCount)
	}
}

func TestHealthMonitor_MetricsNoInfNaN(t *testing.T) {
	m := NewHealthMonitor(0)
	defer m.Shutdown()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	m.metricsHandler(w, r)

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	checkFloat := func(key string) {
		raw, ok := v[key]
		if !ok {
			t.Fatalf("missing key %s", key)
		}
		f, ok := raw.(float64)
		if !ok {
			t.Fatalf("key %s not float64", key)
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			t.Fatalf("key %s is invalid: %v", key, f)
		}
	}

	checkFloat("messages_per_minute")
	checkFloat("tool_calls_per_minute")
	checkFloat("approval_rate")
}
