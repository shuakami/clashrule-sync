//go:build !windows

package process

import (
	"os/exec"
)

// setNoWindowFlag 在非 Windows 平台上是空操作
func setNoWindowFlag(cmd *exec.Cmd) {
	// 非 Windows 平台上不需要任何操作
} 