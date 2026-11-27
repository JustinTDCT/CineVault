//go:build unix

package transcode

import (
	"os"
	"syscall"
)

// suspend pauses a process without ending it (SIGSTOP).
func suspend(p *os.Process) bool { return p.Signal(syscall.SIGSTOP) == nil }

// resumeProcess continues a suspended process (SIGCONT).
func resumeProcess(p *os.Process) { _ = p.Signal(syscall.SIGCONT) }
