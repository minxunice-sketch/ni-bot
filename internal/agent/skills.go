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
	overridesRoot, localRoot, upstreamRoot := skillRoots(workspace)
	skillNames := map[string]struct{}{}
	for _, r := range []string{overridesRoot, localRoot, upstreamRoot} {
		names, _ := listSkillDirs(r)
		for _, n := range names {
			skillNames[n] = struct{}{}
		}
	}
	if len(skillNames) == 0 {
		return nil, nil
	}

	var scripts []SkillScript
	for name := range skillNames {
		scriptsSet := map[string]struct{}{}
		for _, r := range []string{upstreamRoot, localRoot, overridesRoot} {
			dir := filepath.Join(r, name, "scripts")
			scs, err := discoverScriptsInDir(dir)
			if err != nil {
				continue
			}
			for _, sc := range scs {
				scriptsSet[sc] = struct{}{}
			}
		}
		var merged []string
		for sc := range scriptsSet {
			merged = append(merged, sc)
		}
		sort.Strings(merged)
		for _, sc := range merged {
			scripts = append(scripts, SkillScript{Skill: name, Script: sc})
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
	overridesRoot, localRoot, upstreamRoot := skillRoots(workspace)
	skillNames := map[string]struct{}{}
	for _, r := range []string{overridesRoot, localRoot, upstreamRoot} {
		names, _ := listSkillDirs(r)
		for _, n := range names {
			skillNames[n] = struct{}{}
		}
	}
	if len(skillNames) == 0 {
		return nil, nil
	}

	var skills []Skill
	for name := range skillNames {
		overrideDir := filepath.Join(overridesRoot, name)
		localDir := filepath.Join(localRoot, name)
		upstreamDir := filepath.Join(upstreamRoot, name)

		layer := ""
		primaryDir := ""
		if dirExists(overrideDir) {
			layer = "override"
			primaryDir = overrideDir
		} else if dirExists(localDir) {
			layer = "local"
			primaryDir = localDir
		} else if dirExists(upstreamDir) {
			layer = "upstream"
			primaryDir = upstreamDir
		} else {
			continue
		}

		info := loadSkillInfo(primaryDir, name)

		origin := firstNonEmpty(readSkillOrigin(overrideDir), readSkillOrigin(localDir), readSkillOrigin(upstreamDir))
		parts := []string{}
		if strings.TrimSpace(info.Source) != "" {
			parts = append(parts, strings.TrimSpace(info.Source))
		}
		if layer != "" {
			parts = append(parts, "layer="+layer)
		}
		if origin != "" {
			parts = append(parts, "origin="+origin)
		}
		info.Source = strings.Join(parts, "; ")

		scripts := map[string]struct{}{}
		for _, r := range []string{upstreamDir, localDir, overrideDir} {
			scs, err := discoverScriptsInDir(filepath.Join(r, "scripts"))
			if err != nil {
				continue
			}
			for _, sc := range scs {
				scripts[sc] = struct{}{}
			}
		}
		var merged []string
		for sc := range scripts {
			merged = append(merged, sc)
		}
		sort.Strings(merged)
		info.Scripts = merged
		skills = append(skills, info)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

func skillRoots(workspace string) (overridesRoot, localRoot, upstreamRoot string) {
	localRoot = filepath.Join(workspace, "skills")
	overridesRoot = filepath.Join(localRoot, "_overrides")
	upstreamRoot = filepath.Join(localRoot, "_upstream")
	return overridesRoot, localRoot, upstreamRoot
}

func listSkillDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

type skillOriginMeta struct {
	Origin string `json:"origin"`
}

func readSkillOrigin(skillDir string) string {
	b, err := os.ReadFile(filepath.Join(skillDir, ".nibot_source.json"))
	if err != nil {
		return ""
	}
	var m skillOriginMeta
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return strings.TrimSpace(m.Origin)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
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
