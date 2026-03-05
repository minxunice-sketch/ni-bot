package agent

import (
	"strings"
	"testing"
)

func TestBuildAutoRecallBlock_IncludesMatches(t *testing.T) {
	t.Setenv("NIBOT_MEMORY_DB", "sqlite")
	t.Setenv("NIBOT_AUTO_RECALL", "1")
	ws := t.TempDir()

	s, err := OpenSQLiteStore(ws)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if s == nil {
		t.Fatalf("expected sqlite store")
	}
	_, err = s.InsertMemory("global", "pref", "Prefer TypeScript over JavaScript")
	s.Close()
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	block := buildAutoRecallBlock(ws, "TypeScript coding style?")
	if !strings.Contains(block, "=== RELEVANT MEMORIES ===") {
		t.Fatalf("expected header, got: %q", block)
	}
	if !strings.Contains(strings.ToLower(block), "typescript") {
		t.Fatalf("expected match content, got: %q", block)
	}
}

func TestBuildAutoRecallBlock_SkipsToolResults(t *testing.T) {
	t.Setenv("NIBOT_MEMORY_DB", "sqlite")
	t.Setenv("NIBOT_AUTO_RECALL", "1")
	ws := t.TempDir()

	s, err := OpenSQLiteStore(ws)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if s == nil {
		t.Fatalf("expected sqlite store")
	}
	_, _ = s.InsertMemory("global", "x", "hello")
	s.Close()

	block := buildAutoRecallBlock(ws, "TOOL_RESULTS:\n- tool: x\n  ok: true\n")
	if strings.TrimSpace(block) != "" {
		t.Fatalf("expected empty, got: %q", block)
	}
}
