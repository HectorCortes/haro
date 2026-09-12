//go:build windows

package broker

import (
	"os/exec"
	"syscall"
)

// detachProcessFlags detaches the child: no console and its own process
// group, so the daemon outlives the CLI.
const detachedProcessFlags = 0x00000008 | syscall.CREATE_NEW_PROCESS_GROUP

func detachAttrs(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcessFlags}
}
