package agent

import (
	"strings"
	"testing"
)

func TestParseImportedMemories_MultiFormats(t *testing.T) {
	in := strings.Join([]string{
		"```",
		"- [2026-03-01] - Prefer Chinese bullet points",
		"2026-03-02 - Use Go for backend",
		"1.  Output format: markdown",
		"*  Avoid emojis",
		"```",
		"",
		"- Prefer Chinese bullet points",
	}, "\n")

	got := parseImportedMemories(in, 50)
	if len(got) < 4 {
		t.Fatalf("expected >= 4 items, got=%d: %#v", len(got), got)
	}
	joined := strings.ToLower(strings.Join(got, "\n"))
	for _, want := range []string{
		"prefer chinese bullet points",
		"use go for backend",
		"output format: markdown",
		"avoid emojis",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestSQLiteStore_UpsertMemory_DedupAndMergeTags(t *testing.T) {
	t.Setenv("NIBOT_MEMORY_DB", "sqlite")
	ws := t.TempDir()
	s, err := OpenSQLiteStore(ws)
	if err != nil || s == nil {
		t.Fatalf("OpenSQLiteStore err=%v store=%v", err, s)
	}
	defer s.Close()

	id1, act1, err := s.UpsertMemory("global", "a,b", "Hello World", "import")
	if err != nil {
		t.Fatalf("upsert1: %v", err)
	}
	if act1 != "inserted" || id1 <= 0 {
		t.Fatalf("unexpected act/id: act=%s id=%d", act1, id1)
	}

	id2, act2, err := s.UpsertMemory("global", "b,c", "Hello   World", "import")
	if err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id, got id1=%d id2=%d", id1, id2)
	}
	if act2 != "updated" && act2 != "unchanged" {
		t.Fatalf("unexpected act2=%s", act2)
	}

	items, err := s.SearchMemories("global", "hello", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected search results")
	}
	if !strings.Contains(items[0].Tags, "a") || !strings.Contains(items[0].Tags, "b") || !strings.Contains(items[0].Tags, "c") {
		t.Fatalf("expected merged tags, got=%q", items[0].Tags)
	}
}

