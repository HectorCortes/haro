//go:build windows

package process

import "os/exec"

func configure(_ *exec.Cmd) {}

// Windows uses the safe direct-process fallback. A Job Object is deliberately
// not introduced here because the existing project has no Windows process
// ownership abstraction to integrate with.
func kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
