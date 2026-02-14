package internal

import (
	"sync"
)

const maxAPILogLines = 500

var (
	apiLogMu   sync.Mutex
	apiLogBuf  []string
)

// AppendAPILogLine adds a line to the in-app API log (for TUI display).
// Safe to call from any goroutine.
func AppendAPILogLine(line string) {
	apiLogMu.Lock()
	defer apiLogMu.Unlock()
	apiLogBuf = append(apiLogBuf, line)
	if len(apiLogBuf) > maxAPILogLines {
		apiLogBuf = apiLogBuf[len(apiLogBuf)-maxAPILogLines:]
	}
}

// GetAPILogLines returns a copy of the current API log lines for display.
func GetAPILogLines() []string {
	apiLogMu.Lock()
	defer apiLogMu.Unlock()
	out := make([]string, len(apiLogBuf))
	copy(out, apiLogBuf)
	return out
}
