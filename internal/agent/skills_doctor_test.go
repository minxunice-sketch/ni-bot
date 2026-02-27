package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiagnoseSkills_WarnsUnsupportedScriptOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only expectation")
	}
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(ws, "skills", "x", "scripts")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "skills", "x", "SKILL.md"), []byte("---\nname: X\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "run.sh"), []byte("echo ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	issues, err := DiagnoseSkills(ws)
	if err != nil {
		t.Fatalf("diagnose failed: %v", err)
	}
	found := false
	for _, it := range issues {
		if it.Skill == "x" && it.Level == "warn" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warn issue for unsupported script on windows, got: %#v", issues)
	}
}

