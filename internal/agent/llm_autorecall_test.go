package agent

import (
	"os"
	"path/filepath"
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

func TestBuildAutoRecallBlock_IncludesLearningsMatchesWithoutSQLite(t *testing.T) {
	t.Setenv("NIBOT_AUTO_RECALL", "1")
	t.Setenv("NIBOT_AUTO_RECALL_LEARNINGS", "1")
	ws := t.TempDir()

	if err := os.MkdirAll(filepath.Join(ws, ".learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "## 2026-03-18\n\n学习主题：自动整理文件技能\n\n来源：example/repo\n\n触发方式：监听目录\n"
	if err := os.WriteFile(filepath.Join(ws, ".learnings", "LEARNINGS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	block := buildAutoRecallBlock(ws, "我想学习如何创建一个自动整理文件的技能")
	if !strings.Contains(block, "=== RELEVANT LEARNINGS ===") {
		t.Fatalf("expected learnings header, got: %q", block)
	}
	if !strings.Contains(block, "自动整理文件") {
		t.Fatalf("expected learnings content, got: %q", block)
	}
}
