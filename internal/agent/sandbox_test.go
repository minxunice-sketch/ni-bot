package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWrapWithSandbox_DisabledReturnsSame(t *testing.T) {
	t.Setenv("NIBOT_EXEC_SANDBOX", "")
	in := []string{"sh", "-lc", "echo ok"}
	out, err := wrapWithSandbox(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) || out[0] != in[0] {
		t.Fatalf("unexpected out: %#v", out)
	}
}

func TestRuntimeExec_SandboxMissingFailsFast(t *testing.T) {
	t.Setenv("NIBOT_ENABLE_EXEC", "1")
	t.Setenv("NIBOT_EXEC_SANDBOX", "1")
	t.Setenv("NIBOT_SANDBOX_BIN", "nonexistent_sandbox_bin_12345")

	ws := t.TempDir()
	ctx := ExecContext{Workspace: ws, Policy: DefaultToolPolicy()}
	_, err := toolRuntimeExec(ctx, `{"command":"echo ok","timeoutSeconds":1}`)
	if err == nil {
		t.Fatalf("expected sandbox missing error")
	}
}

func TestSkillExec_SandboxMissingFailsFast(t *testing.T) {
	t.Setenv("NIBOT_ENABLE_SKILLS", "1")
	t.Setenv("NIBOT_EXEC_SANDBOX", "1")
	t.Setenv("NIBOT_SANDBOX_BIN", "nonexistent_sandbox_bin_12345")

	ws := t.TempDir()
	scriptsDir := filepath.Join(ws, "skills", "echo", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptName := "echo.sh"
	body := "echo ok\n"
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		scriptName = "echo.cmd"
		body = "@echo ok\r\n"
		perm = 0o644
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, scriptName), []byte(body), perm); err != nil {
		t.Fatal(err)
	}

	ctx := ExecContext{Workspace: ws, Policy: DefaultToolPolicy()}
	_, err := toolSkillExec(ctx, `{"skill":"echo","script":"`+scriptName+`","args":[],"timeoutSeconds":1}`)
	if err == nil {
		t.Fatalf("expected sandbox missing error")
	}
}

