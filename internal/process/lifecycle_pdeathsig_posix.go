//go:build !linux && !windows

package process

import "syscall"

func configurePdeathsig(_ *syscall.SysProcAttr) {}
