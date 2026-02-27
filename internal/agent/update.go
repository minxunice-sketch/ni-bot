package agent

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func runSelfUpdate(out io.Writer) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found in PATH")
	}

	if _, err := runUpdateCmd(out, dir, gitPath, []string{"rev-parse", "--is-inside-work-tree"}, 30*time.Second); err != nil {
		return fmt.Errorf("not a git repository")
	}

	dirtyOut, err := runUpdateCmd(out, dir, gitPath, []string{"status", "--porcelain"}, 30*time.Second)
	if err != nil {
		return err
	}
	stashed := false
	if strings.TrimSpace(dirtyOut) != "" {
		if _, err := runUpdateCmd(out, dir, gitPath, []string{"stash", "push", "-u", "-m", "nibot-auto-update"}, 2*time.Minute); err != nil {
			return err
		}
		stashed = true
	}

	if _, err := runUpdateCmd(out, dir, gitPath, []string{"pull", "--rebase"}, 5*time.Minute); err != nil {
		return err
	}
	if stashed {
		_, _ = runUpdateCmd(out, dir, gitPath, []string{"stash", "pop"}, 2*time.Minute)
	}

	if _, err := runUpdateCmd(out, dir, goPath, []string{"mod", "tidy"}, 5*time.Minute); err != nil {
		return err
	}

	bin := "nibot"
	if runtime.GOOS == "windows" {
		bin = "nibot.exe"
	}
	outPath := filepath.Join(dir, bin)
	if _, err := runUpdateCmd(out, dir, goPath, []string{"build", "-o", outPath, "./cmd/nibot"}, 10*time.Minute); err != nil {
		return err
	}
	return nil
}

func runUpdateCmd(out io.Writer, dir string, exe string, args []string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(exe) == "" {
		return "", fmt.Errorf("empty exe")
	}
	argv := append([]string{exe}, args...)
	argv, err := wrapWithSandbox(argv)
	if err != nil {
		return "", err
	}
	release := acquireExecSlot()
	defer release()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir

	maxOut := execMaxOutputBytes()
	stdout := newCappedBuffer(maxOut)
	stderr := newCappedBuffer(maxOut)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if out != nil {
		_, _ = fmt.Fprintf(out, "\n$ %s %s\n", filepath.Base(exe), strings.Join(args, " "))
	}

	if err := runWithTimeout(cmd, timeout); err != nil {
		return formatExecOutput(strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())), err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
