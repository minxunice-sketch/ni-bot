package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sort"
	"strings"
	"time"
)

func skillsMaxFileBytes() int64 {
	const defaultMax = int64(5 * 1024 * 1024)
	if v, ok := os.LookupEnv("NIBOT_SKILLS_MAX_FILE_BYTES"); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				return n
			}
		}
	}
	return defaultMax
}

func shouldIgnoreSkillDir(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case ".git", ".github", ".idea", ".vscode", "node_modules", "dist", "build", "target", "vendor", ".venv", "venv", "__pycache__":
		return true
	default:
		return false
	}
}

func InstallSkillsFromPath(workspace string, src string) ([]string, error) {
	return InstallSkillsFromPathWithOrigin(workspace, src, "", "local")
}

func InstallSkillsFromPathWithOrigin(workspace string, src string, origin string, defaultLayer string) ([]string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty source path")
	}

	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return nil, err
	}
	if origin == "" {
		origin = srcAbs
	}

	layer := strings.ToLower(strings.TrimSpace(os.Getenv("NIBOT_SKILLS_INSTALL_LAYER")))
	if layer == "" {
		layer = strings.ToLower(strings.TrimSpace(defaultLayer))
	}
	dstRoot := filepath.Join(workspace, "skills")
	if layer == "upstream" {
		dstRoot = filepath.Join(dstRoot, "_upstream")
	}
	_ = os.MkdirAll(dstRoot, 0o755)

	st, err := os.Stat(srcAbs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		if strings.EqualFold(filepath.Ext(srcAbs), ".zip") {
			return installSkillsFromZip(workspace, srcAbs, origin, defaultLayer)
		}
		return nil, fmt.Errorf("source is not a directory: %s", srcAbs)
	}

	if dirExists(filepath.Join(srcAbs, "skills")) {
		installed, err := installSkillsFromSkillsRoot(dstRoot, filepath.Join(srcAbs, "skills"))
		if err != nil {
			return nil, err
		}
		for _, name := range installed {
			_ = writeSkillSourceMeta(filepath.Join(dstRoot, name), origin, layer)
		}
		return installed, nil
	}

	if dirExists(filepath.Join(srcAbs, "scripts")) {
		name := filepath.Base(srcAbs)
		if err := installOneSkillDir(dstRoot, srcAbs, name); err != nil {
			return nil, err
		}
		_ = writeSkillSourceMeta(filepath.Join(dstRoot, name), origin, layer)
		return []string{name}, nil
	}

	if strings.EqualFold(filepath.Base(srcAbs), "scripts") {
		skillDir := filepath.Dir(srcAbs)
		name := filepath.Base(skillDir)
		tmp := filepath.Join(os.TempDir(), "nibot_skill_install_"+name)
		_ = os.RemoveAll(tmp)
		if err := os.MkdirAll(filepath.Join(tmp, "scripts"), 0o755); err != nil {
			return nil, err
		}
		if err := copyDir(srcAbs, filepath.Join(tmp, "scripts"), skillsMaxFileBytes()); err != nil {
			return nil, err
		}
		if err := ensureDefaultSkillMD(tmp, name); err != nil {
			return nil, err
		}
		if err := installOneSkillDir(dstRoot, tmp, name); err != nil {
			return nil, err
		}
		_ = writeSkillSourceMeta(filepath.Join(dstRoot, name), origin, layer)
		return []string{name}, nil
	}

	children, err := os.ReadDir(srcAbs)
	if err != nil {
		return nil, err
	}
	var installed []string
	for _, c := range children {
		if !c.IsDir() {
			continue
		}
		cDir := filepath.Join(srcAbs, c.Name())
		if dirExists(filepath.Join(cDir, "scripts")) {
			if err := installOneSkillDir(dstRoot, cDir, c.Name()); err != nil {
				return nil, err
			}
			_ = writeSkillSourceMeta(filepath.Join(dstRoot, c.Name()), origin, layer)
			installed = append(installed, c.Name())
		}
	}
	if len(installed) > 0 {
		sort.Strings(installed)
		return installed, nil
	}

	return nil, fmt.Errorf("no skills found under: %s", srcAbs)
}

type skillSourceMeta struct {
	Origin      string `json:"origin"`
	Layer       string `json:"layer"`
	InstalledAt string `json:"installed_at"`
}

func writeSkillSourceMeta(skillDir string, origin string, layer string) error {
	origin = strings.TrimSpace(origin)
	layer = strings.ToLower(strings.TrimSpace(layer))
	if origin == "" {
		return nil
	}
	m := skillSourceMeta{
		Origin:      origin,
		Layer:       layer,
		InstalledAt: time.Now().Format(time.RFC3339Nano),
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillDir, ".nibot_source.json"), b, 0o644)
}

func installSkillsFromSkillsRoot(dstRoot, skillsRoot string) ([]string, error) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, err
	}
	var installed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if shouldIgnoreSkillDir(e.Name()) {
			continue
		}
		name := e.Name()
		srcSkill := filepath.Join(skillsRoot, name)
		if !dirExists(filepath.Join(srcSkill, "scripts")) {
			continue
		}
		if err := installOneSkillDir(dstRoot, srcSkill, name); err != nil {
			return nil, err
		}
		installed = append(installed, name)
	}
	if len(installed) == 0 {
		return nil, fmt.Errorf("no skills found under: %s", skillsRoot)
	}
	sort.Strings(installed)
	return installed, nil
}

func installOneSkillDir(dstRoot, srcSkillDir, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty skill name")
	}
	dst := filepath.Join(dstRoot, name)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("skill already exists: %s", name)
	}
	if err := copyDir(srcSkillDir, dst, skillsMaxFileBytes()); err != nil {
		return err
	}
	return nil
}

func ensureDefaultSkillMD(skillDir, name string) error {
	p := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: Imported skill\n---\n", name)
	return os.WriteFile(p, []byte(content), 0o644)
}

func copyDir(src, dst string, maxFileBytes int64) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if shouldIgnoreSkillDir(e.Name()) {
			continue
		}
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		if e.IsDir() {
			if err := copyDir(s, d, maxFileBytes); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d, maxFileBytes); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, maxFileBytes int64) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if maxFileBytes > 0 && st.Size() > maxFileBytes {
		return fmt.Errorf("file too large: %s (%d bytes) exceeds NIBOT_SKILLS_MAX_FILE_BYTES=%d", src, st.Size(), maxFileBytes)
	}
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	df, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	return err
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
