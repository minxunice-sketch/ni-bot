package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecMaxOutput_TruncatesSkillOutput(t *testing.T) {
	t.Setenv("NIBOT_ENABLE_SKILLS", "1")
	t.Setenv("NIBOT_EXEC_MAX_OUTPUT_BYTES", "1024")

	ws := t.TempDir()
	scriptsDir := filepath.Join(ws, "skills", "spam", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	script := "spam.sh"
	body := "for i in $(seq 1 500); do echo 1234567890; done\n"
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script = "spam.cmd"
		body = "@for /L %%i in (1,1,500) do @echo 1234567890\r\n"
		perm = 0o644
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, script), []byte(body), perm); err != nil {
		t.Fatal(err)
	}

	ctx := ExecContext{Workspace: ws, Policy: DefaultToolPolicy()}
	out, err := toolSkillExec(ctx, `{"skill":"spam","script":"`+script+`","args":[],"timeoutSeconds":5}`)
	if err != nil {
		t.Fatalf("skill.exec failed: %v output=%q", err, out)
	}
	if !strings.Contains(out, "[TRUNCATED]") {
		t.Fatalf("expected output truncated marker, got: %q", out)
	}
}

