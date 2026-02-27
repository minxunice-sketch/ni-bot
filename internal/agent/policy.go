package agent

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type ToolPolicy struct {
	Loaded           bool
	AllowFSWrite      bool
	AllowRuntimeExec  bool
	AllowSkillExec    bool
	RequireFSWrite    bool
	RequireRuntimeExec bool
	RequireSkillExec  bool

	AllowedRuntimePrefixes []string
	AllowedWritePrefixes   []string
	AllowedSkillNames      []string
	AllowedSkillScripts    []string
}

func DefaultToolPolicy() ToolPolicy {
	return ToolPolicy{
		Loaded:            true,
		AllowFSWrite:       true,
		AllowRuntimeExec:   true,
		AllowSkillExec:     true,
		RequireFSWrite:     true,
		RequireRuntimeExec: true,
		RequireSkillExec:   true,
		AllowedWritePrefixes: []string{"memory/", "skills/", "logs/"},
	}
}

func LoadToolPolicy(workspace string) ToolPolicy {
	p := DefaultToolPolicy()
	filePolicy, ok := readPolicyToml(filepath.Join(workspace, "data", "policy.toml"))
	if ok {
		if filePolicy.AllowFSWrite != nil {
			p.AllowFSWrite = *filePolicy.AllowFSWrite
		}
		if filePolicy.AllowRuntimeExec != nil {
			p.AllowRuntimeExec = *filePolicy.AllowRuntimeExec
		}
		if filePolicy.AllowSkillExec != nil {
			p.AllowSkillExec = *filePolicy.AllowSkillExec
		}
		if filePolicy.RequireFSWrite != nil {
			p.RequireFSWrite = *filePolicy.RequireFSWrite
		}
		if filePolicy.RequireRuntimeExec != nil {
			p.RequireRuntimeExec = *filePolicy.RequireRuntimeExec
		}
		if filePolicy.RequireSkillExec != nil {
			p.RequireSkillExec = *filePolicy.RequireSkillExec
		}
		if len(filePolicy.AllowedRuntimePrefixes) > 0 {
			p.AllowedRuntimePrefixes = filePolicy.AllowedRuntimePrefixes
		}
		if len(filePolicy.AllowedWritePrefixes) > 0 {
			p.AllowedWritePrefixes = filePolicy.AllowedWritePrefixes
		}
		if len(filePolicy.AllowedSkillNames) > 0 {
			p.AllowedSkillNames = filePolicy.AllowedSkillNames
		}
		if len(filePolicy.AllowedSkillScripts) > 0 {
			p.AllowedSkillScripts = filePolicy.AllowedSkillScripts
		}
	}

	if v, ok := os.LookupEnv("NIBOT_POLICY_ALLOW_RUNTIME_EXEC"); ok && strings.TrimSpace(v) != "" {
		p.AllowRuntimeExec = parseBool(v, p.AllowRuntimeExec)
	}
	if v, ok := os.LookupEnv("NIBOT_POLICY_ALLOW_SKILL_EXEC"); ok && strings.TrimSpace(v) != "" {
		p.AllowSkillExec = parseBool(v, p.AllowSkillExec)
	}
	if v, ok := os.LookupEnv("NIBOT_POLICY_ALLOW_FS_WRITE"); ok && strings.TrimSpace(v) != "" {
		p.AllowFSWrite = parseBool(v, p.AllowFSWrite)
	}
	return p
}

func (p ToolPolicy) AllowsTool(tool string) bool {
	switch tool {
	case "fs.write", "file_write":
		return p.AllowFSWrite
	case "runtime.exec", "shell_exec":
		return p.AllowRuntimeExec
	case "skill.exec", "skill_exec":
		return p.AllowSkillExec
	default:
		return true
	}
}

func (p ToolPolicy) RequiresApproval(tool string) bool {
	switch tool {
	case "fs.write", "file_write":
		return p.RequireFSWrite
	case "runtime.exec", "shell_exec":
		return p.RequireRuntimeExec
	case "skill.exec", "skill_exec":
		return p.RequireSkillExec
	default:
		return false
	}
}

func (p ToolPolicy) AllowsRuntimeCommand(command string) bool {
	if len(p.AllowedRuntimePrefixes) == 0 {
		return true
	}
	tokens := splitCommandLine(command)
	if len(tokens) == 0 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(tokens[0]))
	for _, pref := range p.AllowedRuntimePrefixes {
		pref = strings.ToLower(strings.TrimSpace(pref))
		if pref != "" && first == pref {
			return true
		}
	}
	return false
}

func (p ToolPolicy) AllowsWritePath(relPath string) bool {
	prefixes := p.AllowedWritePrefixes
	if len(prefixes) == 0 {
		return true
	}
	path := filepath.ToSlash(strings.TrimSpace(relPath))
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return false
	}
	for _, pref := range prefixes {
		pref = filepath.ToSlash(strings.TrimSpace(pref))
		pref = strings.TrimLeft(pref, "/")
		if pref == "*" {
			return true
		}
		if pref != "" && strings.HasPrefix(strings.ToLower(path), strings.ToLower(pref)) {
			return true
		}
	}
	return false
}

func (p ToolPolicy) AllowsSkillExec(skill, script string) bool {
	skill = strings.ToLower(strings.TrimSpace(skill))
	script = strings.ToLower(strings.TrimSpace(script))
	if skill == "" || script == "" {
		return false
	}
	if len(p.AllowedSkillNames) > 0 {
		ok := false
		for _, it := range p.AllowedSkillNames {
			it = strings.ToLower(strings.TrimSpace(it))
			if it == "*" || it == skill {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(p.AllowedSkillScripts) == 0 {
		return true
	}
	key := skill + "/" + script
	for _, it := range p.AllowedSkillScripts {
		it = strings.ToLower(strings.TrimSpace(it))
		if it == "*" {
			return true
		}
		if it == script || it == key {
			return true
		}
	}
	return false
}

type policyFile struct {
	AllowFSWrite       *bool
	AllowRuntimeExec   *bool
	AllowSkillExec     *bool
	RequireFSWrite     *bool
	RequireRuntimeExec *bool
	RequireSkillExec   *bool
	AllowedRuntimePrefixes []string
	AllowedWritePrefixes   []string
	AllowedSkillNames      []string
	AllowedSkillScripts    []string
}

func readPolicyToml(path string) (policyFile, bool) {
	f, err := os.Open(path)
	if err != nil {
		return policyFile{}, false
	}
	defer f.Close()

	var pf policyFile
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		val = strings.Trim(val, "'")

		switch key {
		case "allow_fs_write":
			b := parseBool(val, true)
			pf.AllowFSWrite = &b
		case "allow_runtime_exec":
			b := parseBool(val, true)
			pf.AllowRuntimeExec = &b
		case "allow_skill_exec":
			b := parseBool(val, true)
			pf.AllowSkillExec = &b
		case "require_approval_fs_write":
			b := parseBool(val, true)
			pf.RequireFSWrite = &b
		case "require_approval_runtime_exec":
			b := parseBool(val, true)
			pf.RequireRuntimeExec = &b
		case "require_approval_skill_exec":
			b := parseBool(val, true)
			pf.RequireSkillExec = &b
		case "allowed_runtime_prefixes":
			pf.AllowedRuntimePrefixes = splitCSV(val)
		case "allowed_write_prefixes":
			pf.AllowedWritePrefixes = splitCSV(val)
		case "allowed_skill_names":
			pf.AllowedSkillNames = splitCSV(val)
		case "allowed_skill_scripts":
			pf.AllowedSkillScripts = splitCSV(val)
		}
	}

	if pf.AllowFSWrite == nil && pf.AllowRuntimeExec == nil && pf.AllowSkillExec == nil &&
		pf.RequireFSWrite == nil && pf.RequireRuntimeExec == nil && pf.RequireSkillExec == nil &&
		len(pf.AllowedRuntimePrefixes) == 0 && len(pf.AllowedWritePrefixes) == 0 &&
		len(pf.AllowedSkillNames) == 0 && len(pf.AllowedSkillScripts) == 0 {
		return policyFile{}, false
	}
	return pf, true
}

func parseBool(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	var out []string
	for _, it := range raw {
		it = strings.TrimSpace(it)
		it = strings.Trim(it, "\"'")
		if it != "" {
			out = append(out, it)
		}
	}
	return out
}

