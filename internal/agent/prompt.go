package agent

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ConstructSystemPrompt(workspace string) (string, error) {
	var sb strings.Builder

	identityPath := filepath.Join(workspace, "AGENT.md")
	if content, err := os.ReadFile(identityPath); err == nil {
		sb.WriteString("=== IDENTITY ===\n")
		sb.Write(content)
		sb.WriteString("\n\n")
	} else {
		return "", fmt.Errorf("failed to read AGENT.md: %w", err)
	}

	memoryDir := filepath.Join(workspace, "memory")
	sb.WriteString("=== MEMORY ===\n")
	memoryFiles, _ := listMarkdownFiles(memoryDir)
	memoryFiles = reorderMemoryFiles(memoryFiles)
	for _, name := range memoryFiles {
		content, err := os.ReadFile(filepath.Join(memoryDir, name))
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("--- %s ---\n", name))
		sb.Write(content)
		sb.WriteString("\n\n")
	}

	skills, err := DiscoverSkills(workspace)
	if err == nil && len(skills) > 0 {
		sb.WriteString("=== SKILLS ===\n")
		for _, s := range skills {
			if strings.TrimSpace(s.Docs) == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("Skill: %s\n%s\n---\n", s.Name, s.Docs))
		}
	}

	scripts, _ := DiscoverSkillScripts(workspace)
	if len(scripts) > 0 {
		sb.WriteString("\n=== SKILL SCRIPTS ===\n")
		sb.WriteString("The following scripts are available to run via tool skill.exec:\n")
		for _, sc := range scripts {
			sb.WriteString("- " + sc.Skill + "/" + sc.Script + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n=== TOOLS ===\n")
	sb.WriteString("Use these tools by outputting one or more tags in your reply:\n")
	sb.WriteString("[EXEC:fs.read {\"path\":\"memory/facts.md\"}]\n")
	sb.WriteString("[EXEC:fs.write {\"path\":\"memory/notes.md\",\"content\":\"...\",\"mode\":\"append\"}]\n")
	sb.WriteString("[EXEC:runtime.exec {\"command\":\"...\",\"timeoutSeconds\":30}]\n\n")
	sb.WriteString("[EXEC:skill.exec {\"skill\":\"weather\",\"script\":\"weather.ps1\",\"args\":[\"Beijing\"],\"timeoutSeconds\":30}]\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Always use relative paths under workspace.\n")
	sb.WriteString("- fs.write is only allowed under memory/, skills/, logs/.\n")
	sb.WriteString("- fs.write default mode is append; overwrite is restricted.\n")
	sb.WriteString("- runtime.exec may be disabled; if disabled, do not retry.\n")
	sb.WriteString("- skill.exec may be disabled; if disabled, do not retry.\n")
	sb.WriteString("- Write/exec require user approval.\n")
	sb.WriteString("- Never write secrets (API keys, tokens, passwords) to files.\n")

	return sb.String(), nil
}

func loadSkillDocs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var docs []string
	for _, e := range entries {
		if e.IsDir() {
			info := loadSkillInfo(filepath.Join(dir, e.Name()), e.Name())
			if strings.TrimSpace(info.Docs) != "" {
				docs = append(docs, fmt.Sprintf("Skill: %s\n%s", e.Name(), info.Docs))
			}
		}
	}
	return docs, nil
}

func parseSkillMD(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var frontmatter []string
	var body []string
	inFrontmatter := false
	frontmatterDone := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" && !frontmatterDone {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			frontmatterDone = true
			continue
		}

		if inFrontmatter && !frontmatterDone {
			frontmatter = append(frontmatter, line)
			continue
		}
		body = append(body, line)
	}

	name := ""
	description := ""
	for _, l := range frontmatter {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(l, "name:"))
		}
		if strings.HasPrefix(l, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(l, "description:"))
		}
	}

	var sb strings.Builder
	if name != "" {
		sb.WriteString("Name: " + name + "\n")
	}
	if description != "" {
		sb.WriteString("Description: " + description + "\n")
	}
	bodyText := strings.TrimSpace(strings.Join(body, "\n"))
	if bodyText != "" {
		sb.WriteString("\n")
		sb.WriteString(bodyText)
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func listMarkdownFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func reorderMemoryFiles(names []string) []string {
	if len(names) == 0 {
		return names
	}
	var prioritized []string
	var rest []string
	for _, n := range names {
		l := strings.ToLower(n)
		if l == "facts.md" || l == "reflections.md" {
			prioritized = append(prioritized, n)
		} else {
			rest = append(rest, n)
		}
	}
	sort.SliceStable(prioritized, func(i, j int) bool {
		li := strings.ToLower(prioritized[i])
		lj := strings.ToLower(prioritized[j])
		if li == lj {
			return prioritized[i] < prioritized[j]
		}
		if li == "facts.md" {
			return true
		}
		if lj == "facts.md" {
			return false
		}
		if li == "reflections.md" {
			return true
		}
		if lj == "reflections.md" {
			return false
		}
		return prioritized[i] < prioritized[j]
	})
	return append(prioritized, rest...)
}
