//go:build linux

package process

import "syscall"

func configurePdeathsig(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
