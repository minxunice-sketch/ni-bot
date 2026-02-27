package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func sandboxEnabled() bool {
	return strings.TrimSpace(os.Getenv("NIBOT_EXEC_SANDBOX")) == "1"
}

func sandboxBin() string {
	if v := strings.TrimSpace(os.Getenv("NIBOT_SANDBOX_BIN")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return "trae-sandbox.exe"
	}
	return "trae-sandbox"
}

func wrapWithSandbox(argv []string) ([]string, error) {
	if !sandboxEnabled() {
		return argv, nil
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command argv")
	}
	bin := sandboxBin()
	if filepath.IsAbs(bin) {
		if _, err := os.Stat(bin); err != nil {
			return nil, fmt.Errorf("sandbox binary not found: %s", bin)
		}
		return append([]string{bin}, argv...), nil
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("sandbox enabled but %s not found in PATH", bin)
	}
	return append([]string{bin}, argv...), nil
}

