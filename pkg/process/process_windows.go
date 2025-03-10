 //go:build windows

package process

import (
	"os/exec"
	"syscall"
)

// setNoWindowFlag 设置进程为隐藏窗口模式（Windows 特有）
func setNoWindowFlag(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}