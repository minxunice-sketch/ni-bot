package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadToolPolicy_Defaults(t *testing.T) {
	ws := t.TempDir()
	p := LoadToolPolicy(ws)
	if !p.AllowFSWrite || !p.AllowRuntimeExec || !p.AllowSkillExec {
		t.Fatalf("unexpected allow defaults: %+v", p)
	}
	if !p.RequireFSWrite || !p.RequireRuntimeExec || !p.RequireSkillExec {
		t.Fatalf("unexpected require defaults: %+v", p)
	}
}

func TestLoadToolPolicy_FromFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "allow_runtime_exec = false\nrequire_approval_fs_write = false\nallowed_runtime_prefixes = \"go,git\"\n"
	if err := os.WriteFile(filepath.Join(ws, "data", "policy.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := LoadToolPolicy(ws)
	if p.AllowRuntimeExec != false {
		t.Fatalf("expected allow_runtime_exec=false, got %+v", p)
	}
	if p.RequireFSWrite != false {
		t.Fatalf("expected require_approval_fs_write=false, got %+v", p)
	}
	if len(p.AllowedRuntimePrefixes) != 2 || p.AllowedRuntimePrefixes[0] != "go" || p.AllowedRuntimePrefixes[1] != "git" {
		t.Fatalf("unexpected allowed prefixes: %#v", p.AllowedRuntimePrefixes)
	}
}

func TestExecuteCalls_DeniedByPolicy(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("NIBOT_ENABLE_SKILLS", "1")

	scriptsDir := filepath.Join(ws, "skills", "echo", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptName := "echo.sh"
	scriptBody := "echo ok\n"
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		scriptName = "echo.cmd"
		scriptBody = "@echo ok\r\n"
		perm = 0o644
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, scriptName), []byte(scriptBody), perm); err != nil {
		t.Fatal(err)
	}

	ctx := ExecContext{Workspace: ws, Policy: ToolPolicy{
		Loaded:        true,
		AllowSkillExec: false,
	}}
	results := ExecuteCalls(ctx, []ExecCall{
		{Tool: "skill.exec", ArgsRaw: `{"skill":"echo","script":"` + scriptName + `","args":[],"timeoutSeconds":10}`},
	}, nil)

	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected denied result, got: %#v", results)
	}
	if results[0].Error != "disabled by policy" {
		t.Fatalf("unexpected error: %#v", results[0])
	}
}

func TestPolicy_AllowsWritePath(t *testing.T) {
	p := DefaultToolPolicy()
	p.AllowedWritePrefixes = []string{"memory/notes.md"}
	if !p.AllowsWritePath("memory/notes.md") {
		t.Fatalf("expected notes allowed")
	}
	if p.AllowsWritePath("memory/facts.md") {
		t.Fatalf("expected facts denied")
	}
}

func TestToolSkillExec_DeniedByPolicy(t *testing.T) {
	t.Setenv("NIBOT_ENABLE_SKILLS", "1")

	ws := t.TempDir()
	scriptsDir := filepath.Join(ws, "skills", "echo", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptName := "echo.sh"
	scriptBody := "echo ok\n"
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		scriptName = "echo.cmd"
		scriptBody = "@echo ok\r\n"
		perm = 0o644
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, scriptName), []byte(scriptBody), perm); err != nil {
		t.Fatal(err)
	}

	p := DefaultToolPolicy()
	p.AllowedSkillNames = []string{"weather"}
	ctx := ExecContext{Workspace: ws, Policy: p}
	_, err := toolSkillExec(ctx, `{"skill":"echo","script":"`+scriptName+`","args":[],"timeoutSeconds":5}`)
	if err == nil {
		t.Fatalf("expected denied by policy")
	}
	if !strings.Contains(err.Error(), "denied by policy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

