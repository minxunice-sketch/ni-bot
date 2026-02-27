package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestOpenAICompatible_NativeToolCallsTranslatedToExec(t *testing.T) {
	t.Setenv("NIBOT_ENABLE_NATIVE_TOOLS", "1")

	var gotTools []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var req map[string]any
		if err := json.Unmarshal(b, &req); err != nil {
			t.Fatal(err)
		}
		if tools, ok := req["tools"].([]any); ok {
			for _, it := range tools {
				m, _ := it.(map[string]any)
				fn, _ := m["function"].(map[string]any)
				if name, _ := fn["name"].(string); name != "" {
					gotTools = append(gotTools, name)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"type":"function","function":{"name":"file_read","arguments":"{\"path\":\"memory/facts.md\"}"}}]}}]}`))
	}))
	defer srv.Close()

	cfg := Config{
		Provider:  "openai",
		BaseURL:   srv.URL,
		APIKey:    "x",
		ModelName: "gpt-4-turbo",
		Policy: ToolPolicy{
			Loaded:           true,
			AllowFSWrite:      false,
			AllowRuntimeExec:  false,
			AllowSkillExec:    false,
			RequireFSWrite:    true,
			RequireRuntimeExec: true,
			RequireSkillExec:  true,
		},
	}
	c := NewLLMClient(cfg, t.TempDir(), "sys", nil)

	out, err := c.callOpenAICompatible([]Message{{Role: "user", Content: "read"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[EXEC:file_read") {
		t.Fatalf("expected translated exec call, got: %q", out)
	}
	if strings.Contains(strings.Join(gotTools, ","), "shell_exec") || strings.Contains(strings.Join(gotTools, ","), "skill_exec") || strings.Contains(strings.Join(gotTools, ","), "file_write") {
		t.Fatalf("expected tools gated by policy, got: %#v", gotTools)
	}
	if !strings.Contains(strings.Join(gotTools, ","), "file_read") {
		t.Fatalf("expected file_read to be present, got: %#v", gotTools)
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
