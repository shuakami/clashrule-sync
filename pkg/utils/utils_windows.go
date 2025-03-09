//go:build windows
// +build windows

package utils

import (
	"os/exec"
	"syscall"
)

// hideWindowsProcess 为 Windows 平台设置隐藏窗口属性
func hideWindowsProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
} 