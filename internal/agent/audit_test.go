package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoop_WritesAuditToLogger(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "AGENT.md"), []byte("agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "memory", "facts.md"), []byte("facts"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp(t.TempDir(), "session_*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	client := NewLLMClient(Config{}, ws, "dummy", nil)
	in := bytes.NewBufferString("读取 facts\nexit\n")
	var out bytes.Buffer
	client.Loop(in, &out, f)

	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "### Audit") {
		t.Fatalf("expected audit section, got: %s", s)
	}
	if !strings.Contains(s, "tool=fs.read") {
		t.Fatalf("expected fs.read audit, got: %s", s)
	}
}
