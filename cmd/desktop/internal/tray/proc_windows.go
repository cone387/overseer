//go:build windows

package tray

import "syscall"

func hiddenWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow: true,
	}
}
