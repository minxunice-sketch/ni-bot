package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtractExecCalls_AllowsJSONArrayArgs(t *testing.T) {
	text := `[EXEC:skill.exec {"skill":"weather","script":"weather.ps1","args":["Beijing"],"timeoutSeconds":30}]`
	calls := ExtractExecCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Tool != "skill.exec" {
		t.Fatalf("expected tool skill.exec, got %q", calls[0].Tool)
	}
	var a skillExecArgs
	if err := json.Unmarshal([]byte(calls[0].ArgsRaw), &a); err != nil {
		t.Fatalf("args should be valid JSON: %v (raw=%q)", err, calls[0].ArgsRaw)
	}
	if a.Skill != "weather" || a.Script != "weather.ps1" || len(a.Args) != 1 || a.Args[0] != "Beijing" {
		t.Fatalf("unexpected args: %+v", a)
	}
}

func TestExtractExecCalls_DoesNotTerminateOnBracketInString(t *testing.T) {
	text := `[EXEC:fs.write {"path":"memory/notes.md","content":"hello ] world","mode":"append"}]`
	calls := ExtractExecCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Tool != "fs.write" {
		t.Fatalf("expected tool fs.write, got %q", calls[0].Tool)
	}
	if !strings.Contains(calls[0].ArgsRaw, `hello ] world`) {
		t.Fatalf("expected bracket to remain inside JSON string, raw=%q", calls[0].ArgsRaw)
	}
}

func TestToolFSWrite_AppendAddsNewlineBetweenEntries(t *testing.T) {
	ws := t.TempDir()
	ctx := ExecContext{Workspace: ws}

	first := `{"path":"memory/notes.md","content":"first","mode":"append"}`
	if _, err := toolFSWrite(ctx, first); err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	second := `{"path":"memory/notes.md","content":"second","mode":"append"}`
	if _, err := toolFSWrite(ctx, second); err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(ws, "memory", "notes.md"))
	if err != nil {
		t.Fatalf("read notes.md: %v", err)
	}
	got := string(b)
	if got != "first\nsecond" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestToolFSWrite_OverwriteDeniedForProtectedFiles(t *testing.T) {
	ws := t.TempDir()
	ctx := ExecContext{Workspace: ws}

	_, err := toolFSWrite(ctx, `{"path":"memory/facts.md","content":"x","mode":"overwrite"}`)
	if err == nil {
		t.Fatalf("expected overwrite to be denied")
	}
	if !strings.Contains(err.Error(), "overwrite denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolSkillExec_RunsLocalScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("NIBOT_ENABLE_SKILLS", "1")

		ws := t.TempDir()
		scriptsDir := filepath.Join(ws, "skills", "echo", "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		script := filepath.Join(scriptsDir, "echo.cmd")
		if err := os.WriteFile(script, []byte("@echo hello-skill\r\n"), 0o644); err != nil {
			t.Fatalf("write script: %v", err)
		}

		ctx := ExecContext{Workspace: ws}
		out, err := toolSkillExec(ctx, `{"skill":"echo","script":"echo.cmd","args":[],"timeoutSeconds":10}`)
		if err != nil {
			t.Fatalf("skill.exec failed: %v output=%q", err, out)
		}
		if !strings.Contains(out, "hello-skill") {
			t.Fatalf("expected output to contain hello-skill, got %q", out)
		}
		return
	}

	t.Setenv("NIBOT_ENABLE_SKILLS", "1")
	ws := t.TempDir()
	scriptsDir := filepath.Join(ws, "skills", "echo", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	script := filepath.Join(scriptsDir, "echo.sh")
	if err := os.WriteFile(script, []byte("echo hello-skill\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctx := ExecContext{Workspace: ws}
	out, err := toolSkillExec(ctx, `{"skill":"echo","script":"echo.sh","args":[],"timeoutSeconds":10}`)
	if err != nil {
		t.Fatalf("skill.exec failed: %v output=%q", err, out)
	}
	if !strings.Contains(out, "hello-skill") {
		t.Fatalf("expected output to contain hello-skill, got %q", out)
	}
}

func TestToolSkillExec_PrefersOverrideOverUpstream(t *testing.T) {
	t.Setenv("NIBOT_ENABLE_SKILLS", "1")
	ws := t.TempDir()

	up := filepath.Join(ws, "skills", "_upstream", "echo", "scripts")
	ov := filepath.Join(ws, "skills", "_overrides", "echo", "scripts")
	if err := os.MkdirAll(up, 0o755); err != nil {
		t.Fatalf("mkdir upstream: %v", err)
	}
	if err := os.MkdirAll(ov, 0o755); err != nil {
		t.Fatalf("mkdir overrides: %v", err)
	}

	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(up, "echo.cmd"), []byte("@echo upstream\r\n"), 0o644); err != nil {
			t.Fatalf("write upstream: %v", err)
		}
		if err := os.WriteFile(filepath.Join(ov, "echo.cmd"), []byte("@echo override\r\n"), 0o644); err != nil {
			t.Fatalf("write override: %v", err)
		}
		ctx := ExecContext{Workspace: ws}
		out, err := toolSkillExec(ctx, `{"skill":"echo","script":"echo.cmd","args":[],"timeoutSeconds":10}`)
		if err != nil {
			t.Fatalf("skill.exec failed: %v output=%q", err, out)
		}
		if !strings.Contains(out, "override") {
			t.Fatalf("expected override output, got %q", out)
		}
		return
	}

	if err := os.WriteFile(filepath.Join(up, "echo.sh"), []byte("echo upstream\n"), 0o755); err != nil {
		t.Fatalf("write upstream: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ov, "echo.sh"), []byte("echo override\n"), 0o755); err != nil {
		t.Fatalf("write override: %v", err)
	}
	ctx := ExecContext{Workspace: ws}
	out, err := toolSkillExec(ctx, `{"skill":"echo","script":"echo.sh","args":[],"timeoutSeconds":10}`)
	if err != nil {
		t.Fatalf("skill.exec failed: %v output=%q", err, out)
	}
	if !strings.Contains(out, "override") {
		t.Fatalf("expected override output, got %q", out)
	}
}
