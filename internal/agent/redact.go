package agent

import "regexp"

var redactRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\b(LLM_API_KEY|NVIDIA_API_KEY|OPENAI_API_KEY)\b\s*=\s*(".*?"|'.*?'|\S+)`), `$1="<redacted>"`},
	{regexp.MustCompile(`(?i)\b(api_key)\b\s*=\s*(".*?"|'.*?'|\S+)`), `$1="<redacted>"`},
	{regexp.MustCompile(`(?i)("(?:api[_-]?key|llm_api_key|nvidia_api_key|openai_api_key)"\s*:\s*)"(.*?)"`), `$1"<redacted>"`},
	{regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)(\S+)`), `$1<redacted>`},
	{regexp.MustCompile(`(?i)\b(bearer)\s+([A-Za-z0-9._~+/=-]{12,})\b`), `$1 <redacted>`},
	{regexp.MustCompile(`\b(nvapi-[A-Za-z0-9_\-]{8,})\b`), `nvapi-<redacted>`},
	{regexp.MustCompile(`\b(sk-[A-Za-z0-9_\-]{8,})\b`), `sk-<redacted>`},
	{regexp.MustCompile(`(?i)([?&](?:api_key|apikey|key|token)=)([^&\s]+)`), `$1<redacted>`},
}

func redactSecrets(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, r := range redactRules {
		out = r.re.ReplaceAllString(out, r.repl)
	}
	return out
}

func RedactForLog(s string) string {
	return redactSecrets(s)
}

