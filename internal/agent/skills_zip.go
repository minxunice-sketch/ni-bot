package agent

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func skillsMaxTotalBytes() int64 {
	const defaultMax = int64(20 * 1024 * 1024)
	if v, ok := os.LookupEnv("NIBOT_SKILLS_MAX_TOTAL_BYTES"); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				return n
			}
		}
	}
	return defaultMax
}

func skillsMaxZipBytes() int64 {
	const defaultMax = int64(50 * 1024 * 1024)
	if v, ok := os.LookupEnv("NIBOT_SKILLS_MAX_ZIP_BYTES"); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				return n
			}
		}
	}
	return defaultMax
}

func installSkillsFromZip(workspace, zipPath string) ([]string, error) {
	st, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}
	if st.Size() > skillsMaxZipBytes() {
		return nil, fmt.Errorf("zip too large: %d bytes exceeds NIBOT_SKILLS_MAX_ZIP_BYTES=%d", st.Size(), skillsMaxZipBytes())
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	tmp := filepath.Join(os.TempDir(), "nibot_skill_zip_"+safeBaseName(zipPath))
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	maxFile := skillsMaxFileBytes()
	maxTotal := skillsMaxTotalBytes()
	var totalCopied int64

	for _, f := range r.File {
		if f == nil {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}

		rel, ok := safeZipRelPath(f.Name)
		if !ok {
			continue
		}
		if zipPathHasIgnoredSegment(rel) {
			continue
		}

		declared := int64(f.UncompressedSize64)
		if maxFile > 0 && declared > maxFile {
			return nil, fmt.Errorf("zip entry too large: %s (%d bytes) exceeds NIBOT_SKILLS_MAX_FILE_BYTES=%d", rel, declared, maxFile)
		}
		if maxTotal > 0 && totalCopied >= maxTotal {
			return nil, fmt.Errorf("zip total too large: exceeds NIBOT_SKILLS_MAX_TOTAL_BYTES=%d", maxTotal)
		}

		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return nil, err
		}

		var copyErr error
		var n int64
		if maxFile > 0 || maxTotal > 0 {
			limit := int64(0)
			if maxFile > 0 {
				limit = maxFile + 1
			}
			if maxTotal > 0 {
				remain := (maxTotal - totalCopied) + 1
				if limit == 0 || remain < limit {
					limit = remain
				}
			}
			n, copyErr = io.Copy(out, io.LimitReader(rc, limit))
		} else {
			n, copyErr = io.Copy(out, rc)
		}
		_ = out.Close()
		_ = rc.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if maxFile > 0 && n > maxFile {
			return nil, fmt.Errorf("zip entry too large: %s exceeds NIBOT_SKILLS_MAX_FILE_BYTES=%d", rel, maxFile)
		}
		totalCopied += n
		if maxTotal > 0 && totalCopied > maxTotal {
			return nil, fmt.Errorf("zip total too large: exceeds NIBOT_SKILLS_MAX_TOTAL_BYTES=%d", maxTotal)
		}
	}

	return InstallSkillsFromPath(workspace, tmp)
}

func safeBaseName(path string) string {
	b := filepath.Base(path)
	b = strings.TrimSpace(b)
	b = strings.TrimSuffix(b, filepath.Ext(b))
	b = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, b)
	if b == "" {
		return "zip"
	}
	return b
}

func safeZipRelPath(name string) (string, bool) {
	n := strings.ReplaceAll(name, "\\", "/")
	n = strings.TrimLeft(n, "/")
	if n == "" {
		return "", false
	}
	if strings.Contains(n, ":") {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(n))
	if clean == "." || clean == "" {
		return "", false
	}
	if filepath.IsAbs(clean) {
		return "", false
	}
	if strings.HasPrefix(clean, "..") {
		return "", false
	}
	return clean, true
}

func zipPathHasIgnoredSegment(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		if shouldIgnoreSkillDir(p) {
			return true
		}
	}
	return false
}

