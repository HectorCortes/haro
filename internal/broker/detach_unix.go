//go:build !windows

package broker

import (
	"os/exec"
	"syscall"
)

// detachAttrs marks cmd as a detached session leader so a launched broker
// daemon outlives the CLI (Setsid).
func detachAttrs(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
