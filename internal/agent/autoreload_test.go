package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeWorkspaceState_ChangesWhenFileUpdated(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(ws, "memory", "notes.md")
	if err := os.WriteFile(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1, err := computeWorkspaceState(ws)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(p, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := t1.Add(2 * time.Second)
	_ = os.Chtimes(p, future, future)
	t2, err := computeWorkspaceState(ws)
	if err != nil {
		t.Fatal(err)
	}

	if !t2.After(t1) {
		t.Fatalf("unexpected timestamps: t1=%v t2=%v", t1, t2)
	}
}

func TestAutoReloadInterval_Defaults(t *testing.T) {
	t.Setenv("NIBOT_AUTO_RELOAD_INTERVAL_MS", "")
	if autoReloadInterval() <= 0 {
		t.Fatalf("expected positive interval")
	}
}

