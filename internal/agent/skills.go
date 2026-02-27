package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name        string
	DisplayName string
	Description string
	Docs        string
	Scripts     []string
	Source      string
}

type SkillScript struct {
	Skill  string
	Script string
}

func DiscoverSkillScripts(workspace string) ([]SkillScript, error) {
	root := filepath.Join(workspace, "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var scripts []SkillScript
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillName := e.Name()
		scriptsDir := filepath.Join(root, skillName, "scripts")
		scriptsEntries, err := os.ReadDir(scriptsDir)
		if err != nil {
			continue
		}
		for _, se := range scriptsEntries {
			if se.IsDir() {
				continue
			}
			name := se.Name()
			if isExecutableScript(name) {
				scripts = append(scripts, SkillScript{Skill: skillName, Script: name})
			}
		}
	}

	sort.Slice(scripts, func(i, j int) bool {
		if scripts[i].Skill == scripts[j].Skill {
			return scripts[i].Script < scripts[j].Script
		}
		return scripts[i].Skill < scripts[j].Skill
	})
	return scripts, nil
}

func DiscoverSkills(workspace string) ([]Skill, error) {
	root := filepath.Join(workspace, "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(root, name)
		info := loadSkillInfo(dir, name)

		scripts, _ := discoverScriptsInDir(filepath.Join(dir, "scripts"))
		info.Scripts = scripts
		skills = append(skills, info)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

func discoverScriptsInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isExecutableScript(e.Name()) {
			scripts = append(scripts, e.Name())
		}
	}
	sort.Strings(scripts)
	return scripts, nil
}

type jsonSkillManifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

func loadSkillInfo(skillDir, fallbackName string) Skill {
	s := Skill{Name: fallbackName, DisplayName: fallbackName, Source: "directory"}

	if doc, src, ok := loadSkillMD(skillDir, "SKILL.md"); ok {
		s.Docs = doc
		s.Source = src
		s.DisplayName, s.Description = parseSkillDocHeader(doc)
		if s.DisplayName == "" {
			s.DisplayName = fallbackName
		}
		return s
	}
	if doc, src, ok := loadSkillMD(skillDir, "skill.md"); ok {
		s.Docs = doc
		s.Source = src
		s.DisplayName, s.Description = parseSkillDocHeader(doc)
		if s.DisplayName == "" {
			s.DisplayName = fallbackName
		}
		return s
	}

	for _, name := range []string{"skill.json", "manifest.json", "skill.manifest.json"} {
		p := filepath.Join(skillDir, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var m jsonSkillManifest
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		if strings.TrimSpace(m.Name) != "" {
			s.Name = strings.TrimSpace(m.Name)
		}
		if strings.TrimSpace(m.DisplayName) != "" {
			s.DisplayName = strings.TrimSpace(m.DisplayName)
		} else {
			s.DisplayName = s.Name
		}
		s.Description = strings.TrimSpace(m.Description)
		s.Source = name
		s.Docs = formatSkillDocFromMeta(s.DisplayName, s.Description)
		return s
	}

	for _, name := range []string{"skill.yaml", "skill.yml", "manifest.yaml", "manifest.yml"} {
		p := filepath.Join(skillDir, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		m := parseFlatYAML(string(b))
		if v := strings.TrimSpace(m["name"]); v != "" {
			s.Name = v
		}
		if v := strings.TrimSpace(m["display_name"]); v != "" {
			s.DisplayName = v
		} else {
			s.DisplayName = s.Name
		}
		s.Description = strings.TrimSpace(m["description"])
		s.Source = name
		s.Docs = formatSkillDocFromMeta(s.DisplayName, s.Description)
		return s
	}

	if b, err := os.ReadFile(filepath.Join(skillDir, "package.json")); err == nil {
		var pkg struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if json.Unmarshal(b, &pkg) == nil {
			if strings.TrimSpace(pkg.Name) != "" {
				s.Name = strings.TrimSpace(pkg.Name)
				s.DisplayName = s.Name
			}
			s.Description = strings.TrimSpace(pkg.Description)
			s.Source = "package.json"
			s.Docs = formatSkillDocFromMeta(s.DisplayName, s.Description)
			return s
		}
	}

	s.Docs = formatSkillDocFromMeta(s.DisplayName, s.Description)
	return s
}

func loadSkillMD(skillDir, file string) (doc string, source string, ok bool) {
	p := filepath.Join(skillDir, file)
	b, err := os.ReadFile(p)
	if err != nil {
		return "", "", false
	}
	return parseSkillMD(string(b)), file, true
}

func parseSkillDocHeader(doc string) (name, desc string) {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(doc, "\r\n", "\n"), "\r", "\n"), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		low := strings.ToLower(l)
		if name == "" && strings.HasPrefix(low, "name:") {
			name = strings.TrimSpace(l[len("name:"):])
			continue
		}
		if desc == "" && strings.HasPrefix(low, "description:") {
			desc = strings.TrimSpace(l[len("description:"):])
			continue
		}
	}
	return name, desc
}

func parseFlatYAML(content string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n"), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "---") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)
		if k != "" {
			out[strings.ToLower(k)] = v
		}
	}
	return out
}

func formatSkillDocFromMeta(name, desc string) string {
	name = strings.TrimSpace(name)
	desc = strings.TrimSpace(desc)
	var b strings.Builder
	if name != "" {
		b.WriteString(fmt.Sprintf("Name: %s\n", name))
	}
	if desc != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", desc))
	}
	return strings.TrimRight(b.String(), "\n")
}

func isExecutableScript(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".sh") || strings.HasSuffix(l, ".ps1") || strings.HasSuffix(l, ".bat") || strings.HasSuffix(l, ".cmd") || strings.HasSuffix(l, ".exe")
}

