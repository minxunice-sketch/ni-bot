package agent

import (
	"encoding/json"
	"testing"
)

func TestToolMemoryImport_WritesAndRecalls(t *testing.T) {
	t.Setenv("NIBOT_MEMORY_DB", "sqlite")
	ws := t.TempDir()

	ctx := ExecContext{Workspace: ws, Policy: DefaultToolPolicy()}
	args, _ := json.Marshal(map[string]any{
		"source": "smoke",
		"scope":  "global",
		"tags":   "import",
		"text":   "- Prefer Chinese bullet points\n- Use Go\n",
		"limit":  10,
	})
	out, err := toolMemoryImport(ctx, string(args))
	if err != nil {
		t.Fatalf("memory.import err=%v out=%s", err, out)
	}

	recallArgs, _ := json.Marshal(map[string]any{
		"scope": "global",
		"query": "Prefer",
		"limit": 10,
	})
	out2, err := toolMemoryRecall(ctx, string(recallArgs))
	if err != nil {
		t.Fatalf("memory.recall err=%v out=%s", err, out2)
	}
	if out2 == "(no matches)" {
		t.Fatalf("expected matches after import, got: %s", out2)
	}
}
