package agent

import "testing"

func TestFormatToolResultsMeta_DoesNotLeakFullOutput(t *testing.T) {
	results := []ToolResult{
		{Tool: "fs.read", OK: true, Output: "line1\napi_key = \"sk-abcdef\"\nline3", Error: ""},
	}
	meta := formatToolResultsMeta(results)
	if meta == "" {
		t.Fatalf("expected meta output")
	}
	if contains(meta, "sk-abcdef") {
		t.Fatalf("expected secret to be redacted in meta output: %q", meta)
	}
	if !contains(meta, "output_bytes:") || !contains(meta, "output_preview:") {
		t.Fatalf("expected meta fields, got: %q", meta)
	}
}

func TestNormalizeLogLevel_DefaultsToFull(t *testing.T) {
	if normalizeLogLevel("") != "full" {
		t.Fatalf("expected default full")
	}
	if normalizeLogLevel("META") != "meta" {
		t.Fatalf("expected meta")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (stringIndex(s, sub) >= 0))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

