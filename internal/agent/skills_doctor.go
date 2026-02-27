package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type SkillIssue struct {
	Skill   string
	Level   string
	Message string
}

func DiagnoseSkills(workspace string) ([]SkillIssue, error) {
	skills, err := DiscoverSkills(workspace)
	if err != nil {
		return nil, err
	}
	var issues []SkillIssue
	for _, s := range skills {
		if len(s.Scripts) == 0 {
			issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: "no executable scripts under scripts/"})
		}
		if strings.TrimSpace(s.Docs) == "" {
			issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: "no metadata found (SKILL.md/skill.json/manifest.json/skill.yaml)"})
		}
		for _, sc := range s.Scripts {
			p := filepath.Join(workspace, "skills", s.Name, "scripts", sc)
			if !fileExists(p) {
				issues = append(issues, SkillIssue{Skill: s.Name, Level: "error", Message: fmt.Sprintf("missing script file: %s", sc)})
				continue
			}
			if !isScriptSupportedOnThisOS(sc) {
				issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: fmt.Sprintf("script may not run on %s: %s", runtime.GOOS, sc)})
			}
			if st, err := os.Stat(p); err == nil {
				max := skillsMaxFileBytes()
				if max > 0 && st.Size() > max {
					issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: fmt.Sprintf("script size %d exceeds NIBOT_SKILLS_MAX_FILE_BYTES=%d: %s", st.Size(), max, sc)})
				}
			}
		}
	}
	return issues, nil
}

func isScriptSupportedOnThisOS(script string) bool {
	ext := strings.ToLower(filepath.Ext(script))
	switch runtime.GOOS {
	case "windows":
		return ext == ".ps1" || ext == ".cmd" || ext == ".bat" || ext == ".exe"
	default:
		return ext == ".sh" || ext == ".exe"
	}
}

