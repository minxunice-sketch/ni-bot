package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoop_SkillsListsSkillAndScript(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(ws, "skills", "x")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "manifest.json"), []byte(`{"name":"x","display_name":"X","description":"desc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.cmd"), []byte("@echo ok\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := NewLLMClient(Config{}, ws, "dummy", nil)
	in := bytes.NewBufferString("skills\nexit\n")
	var out bytes.Buffer
	client.Loop(in, &out, nil)

	s := out.String()
	if !strings.Contains(s, "Skills:") {
		t.Fatalf("expected Skills header, got: %s", s)
	}
	if !strings.Contains(s, "- x — desc") {
		t.Fatalf("expected skill line, got: %s", s)
	}
	if !strings.Contains(s, "run.cmd") || !strings.Contains(s, "[EXEC:skill.exec") {
		t.Fatalf("expected script call example, got: %s", s)
	}
}

func TestLoop_ReloadUpdatesSystemPrompt(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := NewLLMClient(Config{}, ws, "dummy", nil)
	in := bytes.NewBufferString("reload\nexit\n")
	var out bytes.Buffer
	client.Loop(in, &out, nil)

	if client.SystemMsg == "dummy" || !strings.Contains(client.SystemMsg, "=== IDENTITY ===") {
		t.Fatalf("expected SystemMsg to be reloaded, got: %q", client.SystemMsg)
	}
	if !strings.Contains(out.String(), "已重新加载 System Prompt") {
		t.Fatalf("expected reload message, got: %s", out.String())
	}
}

func TestLoop_SkillsShow(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(ws, "skills", "x")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "manifest.json"), []byte(`{"name":"x","display_name":"X","description":"desc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.cmd"), []byte("@echo ok\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := NewLLMClient(Config{}, ws, "dummy", nil)
	in := bytes.NewBufferString("skills show x\nexit\n")
	var out bytes.Buffer
	client.Loop(in, &out, nil)

	s := out.String()
	if !strings.Contains(s, "Skill: x") {
		t.Fatalf("expected show header, got: %s", s)
	}
	if !strings.Contains(s, "Docs:") || !strings.Contains(s, "Description: desc") {
		t.Fatalf("expected docs/description, got: %s", s)
	}
	if !strings.Contains(s, "Scripts:") || !strings.Contains(s, "run.cmd") {
		t.Fatalf("expected scripts, got: %s", s)
	}
}

func TestLoop_SkillsSearch(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(ws, "skills", "x")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "manifest.json"), []byte(`{"name":"x","display_name":"X","description":"desc"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	client := NewLLMClient(Config{}, ws, "dummy", nil)
	in := bytes.NewBufferString("skills search desc\nexit\n")
	var out bytes.Buffer
	client.Loop(in, &out, nil)

	s := out.String()
	if !strings.Contains(s, "Skills:") || !strings.Contains(s, "- x — desc") {
		t.Fatalf("expected search hit, got: %s", s)
	}
}

func TestLoop_SkillsTest(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(ws, "skills", "x")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "manifest.json"), []byte(`{"name":"x","display_name":"X","description":"desc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.cmd"), []byte("@echo ok\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := NewLLMClient(Config{}, ws, "dummy", nil)
	in := bytes.NewBufferString("skills test x\nexit\n")
	var out bytes.Buffer
	client.Loop(in, &out, nil)

	s := out.String()
	if !strings.Contains(s, "Skills Test:") {
		t.Fatalf("expected test output, got: %s", s)
	}
	if !strings.Contains(s, "skill.exec disabled") {
		t.Fatalf("expected warn about skill.exec env, got: %s", s)
	}
}
