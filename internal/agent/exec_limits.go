package agent

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

func execMaxOutputBytes() int {
	const def = 256 * 1024
	if v := strings.TrimSpace(os.Getenv("NIBOT_EXEC_MAX_OUTPUT_BYTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n < 1024 {
				n = 1024
			}
			if n > 8*1024*1024 {
				n = 8 * 1024 * 1024
			}
			return n
		}
	}
	return def
}

func execMaxConcurrent() int {
	const def = 2
	if v := strings.TrimSpace(os.Getenv("NIBOT_EXEC_MAX_CONCURRENT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 32 {
				n = 32
			}
			return n
		}
	}
	return def
}

var (
	execSemOnce sync.Once
	execSem     chan struct{}
)

func acquireExecSlot() func() {
	execSemOnce.Do(func() {
		execSem = make(chan struct{}, execMaxConcurrent())
	})
	execSem <- struct{}{}
	return func() { <-execSem }
}

type cappedBuffer struct {
	max       int
	buf       []byte
	truncated bool
}

func newCappedBuffer(max int) *cappedBuffer {
	if max <= 0 {
		max = 1024
	}
	return &cappedBuffer{max: max, buf: make([]byte, 0, minInt(max, 4096))}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(b.buf) >= b.max {
		b.truncated = true
		return len(p), nil
	}
	remain := b.max - len(b.buf)
	if len(p) <= remain {
		b.buf = append(b.buf, p...)
		return len(p), nil
	}
	b.buf = append(b.buf, p[:remain]...)
	b.truncated = true
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	s := string(b.buf)
	if b.truncated {
		return s + "\n[TRUNCATED]"
	}
	return s
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

