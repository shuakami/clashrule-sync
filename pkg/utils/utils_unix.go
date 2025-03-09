//go:build !windows
// +build !windows

package utils

import (
	"os/exec"
)

// hideWindowsProcess 在非 Windows 平台上是空操作
func hideWindowsProcess(cmd *exec.Cmd) {
	// 非 Windows 平台无需特殊处理
} 