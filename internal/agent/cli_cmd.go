package agent

import "strings"

func splitCommandLine(input string) []string {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	inQuotes := false
	escape := false

	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}

	for _, r := range s {
		if escape {
			b.WriteRune(r)
			escape = false
			continue
		}
		if r == '\\' && inQuotes {
			escape = true
			continue
		}
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && (r == ' ' || r == '\t') {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return out
}

