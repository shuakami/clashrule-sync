//go:build !windows
// +build !windows

package process

import (
	"os/exec"
)

// 在非 Windows 平台上，设置进程为隐藏窗口模式是空操作
func setNoWindowFlag(cmd *exec.Cmd) {
	// 非 Windows 平台上不需要任何操作
} 