package agent

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillsFromPath_SkillsRoot(t *testing.T) {
	ws := t.TempDir()
	src := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "skills", "a", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "a", "scripts", "a.cmd"), []byte("@echo a\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	installed, err := InstallSkillsFromPath(ws, src)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 || installed[0] != "a" {
		t.Fatalf("unexpected installed: %#v", installed)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "a", "scripts", "a.cmd")); err != nil {
		t.Fatalf("expected installed file: %v", err)
	}
}

func TestInstallSkillsFromPath_IgnoresNoiseDirs(t *testing.T) {
	ws := t.TempDir()
	src := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "skills", "a", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "a", "scripts", "a.cmd"), []byte("@echo a\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "skills", "a", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "a", ".git", "config"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	installed, err := InstallSkillsFromPath(ws, src)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 || installed[0] != "a" {
		t.Fatalf("unexpected installed: %#v", installed)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "a", ".git", "config")); err == nil {
		t.Fatalf("expected .git to be ignored")
	}
}

func TestInstallSkillsFromPath_EnforcesMaxFileBytes(t *testing.T) {
	t.Setenv("NIBOT_SKILLS_MAX_FILE_BYTES", "10")
	ws := t.TempDir()
	src := filepath.Join(t.TempDir(), "big")
	if err := os.MkdirAll(filepath.Join(src, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "scripts", "big.cmd"), []byte("@echo 12345678901\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallSkillsFromPath(ws, src)
	if err == nil {
		t.Fatalf("expected max file bytes error")
	}
}

func TestInstallSkillsFromPath_Zip(t *testing.T) {
	ws := t.TempDir()
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "skills.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	add := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	add("skills/a/scripts/a.cmd", "@echo a\r\n")
	add("skills/a/SKILL.md", "---\nname: a\ndescription: a\n---\n")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	installed, err := InstallSkillsFromPath(ws, zipPath)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 || installed[0] != "a" {
		t.Fatalf("unexpected installed: %#v", installed)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "a", "scripts", "a.cmd")); err != nil {
		t.Fatalf("expected installed file: %v", err)
	}
}

func TestInstallSkillsFromPath_SingleSkillDir(t *testing.T) {
	ws := t.TempDir()
	src := filepath.Join(t.TempDir(), "weather")
	if err := os.MkdirAll(filepath.Join(src, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "scripts", "weather.ps1"), []byte("Write-Output ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	installed, err := InstallSkillsFromPath(ws, src)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 || installed[0] != "weather" {
		t.Fatalf("unexpected installed: %#v", installed)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "weather", "scripts", "weather.ps1")); err != nil {
		t.Fatalf("expected installed file: %v", err)
	}
}

func TestInstallSkillsFromPath_ScriptsOnly(t *testing.T) {
	ws := t.TempDir()
	root := t.TempDir()
	srcScripts := filepath.Join(root, "echo", "scripts")
	if err := os.MkdirAll(srcScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcScripts, "echo.cmd"), []byte("@echo hi\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	installed, err := InstallSkillsFromPath(ws, srcScripts)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 || installed[0] != "echo" {
		t.Fatalf("unexpected installed: %#v", installed)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "echo", "scripts", "echo.cmd")); err != nil {
		t.Fatalf("expected installed file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "skills", "echo", "SKILL.md")); err != nil {
		t.Fatalf("expected generated SKILL.md: %v", err)
	}
}

