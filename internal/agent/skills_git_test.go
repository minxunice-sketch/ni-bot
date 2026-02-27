package agent

import "testing"

func TestIsSafeGitURL(t *testing.T) {
	if !isSafeGitURL("https://example.com/repo.git") {
		t.Fatalf("expected https url allowed")
	}
	if isSafeGitURL("http://example.com/repo.git") {
		t.Fatalf("expected http denied")
	}
	if isSafeGitURL("git@github.com:org/repo.git") {
		t.Fatalf("expected ssh denied")
	}
	if isSafeGitURL("file:///c:/repo") {
		t.Fatalf("expected file scheme denied")
	}
}

func TestInstallSkillsFromGitURL_DisabledByEnv(t *testing.T) {
	ws := t.TempDir()
	_, err := InstallSkillsFromGitURL(ws, "https://example.com/repo.git")
	if err == nil {
		t.Fatalf("expected disabled error")
	}
}

