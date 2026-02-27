package agent

import "testing"

func TestRedactSecrets_EnvAssignment(t *testing.T) {
	in := `$env:LLM_API_KEY="nvapi-abcdef1234567890"`
	out := redactSecrets(in)
	if out == in {
		t.Fatalf("expected redaction, got unchanged: %q", out)
	}
	if out != `$env:LLM_API_KEY="<redacted>"` {
		t.Fatalf("unexpected redacted output: %q", out)
	}
}

func TestRedactSecrets_TomlAPIKey(t *testing.T) {
	in := `api_key = "sk-abcdef1234567890"`
	out := redactSecrets(in)
	if out != `api_key="<redacted>"` {
		t.Fatalf("unexpected redacted output: %q", out)
	}
}

func TestRedactSecrets_JSONAPIKey(t *testing.T) {
	in := `{"api_key":"sk-abcdef1234567890","model":"gpt"}`
	out := redactSecrets(in)
	if out != `{"api_key":"<redacted>","model":"gpt"}` {
		t.Fatalf("unexpected redacted output: %q", out)
	}
}

func TestRedactSecrets_AuthorizationBearer(t *testing.T) {
	in := `Authorization: Bearer sk-abcdef1234567890`
	out := redactSecrets(in)
	if out == in {
		t.Fatalf("expected redaction, got unchanged: %q", out)
	}
}

