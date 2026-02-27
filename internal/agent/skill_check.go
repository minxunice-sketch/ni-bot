package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func CheckSkill(workspace, name string) ([]SkillIssue, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty skill name")
	}
	skills, err := DiscoverSkills(workspace)
	if err != nil {
		return nil, err
	}
	var s *Skill
	for i := range skills {
		if strings.EqualFold(skills[i].Name, name) || strings.EqualFold(skills[i].DisplayName, name) {
			s = &skills[i]
			break
		}
	}
	if s == nil {
		return []SkillIssue{{Skill: name, Level: "error", Message: "skill not found"}}, nil
	}

	var issues []SkillIssue
	if strings.TrimSpace(s.Docs) == "" {
		issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: "no metadata found"})
	}
	if len(s.Scripts) == 0 {
		issues = append(issues, SkillIssue{Skill: s.Name, Level: "error", Message: "no scripts under scripts/"})
		return issues, nil
	}

	if os.Getenv("NIBOT_ENABLE_SKILLS") != "1" {
		issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: "skill.exec disabled (set NIBOT_ENABLE_SKILLS=1 to enable)"})
	}

	base := filepath.Join(workspace, "skills", s.Name, "scripts")
	max := skillsMaxFileBytes()
	for _, sc := range s.Scripts {
		p := filepath.Join(base, sc)
		st, err := os.Stat(p)
		if err != nil {
			issues = append(issues, SkillIssue{Skill: s.Name, Level: "error", Message: fmt.Sprintf("missing script file: %s", sc)})
			continue
		}
		if !isScriptSupportedOnThisOS(sc) {
			issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: fmt.Sprintf("script may not run on %s: %s", runtime.GOOS, sc)})
		}
		if max > 0 && st.Size() > max {
			issues = append(issues, SkillIssue{Skill: s.Name, Level: "warn", Message: fmt.Sprintf("script size %d exceeds NIBOT_SKILLS_MAX_FILE_BYTES=%d: %s", st.Size(), max, sc)})
		}
	}
	return issues, nil
}

