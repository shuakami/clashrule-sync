//go:build !windows
// +build !windows

package handlers

import (
	"github.com/shuakami/clashrule-sync/pkg/logger"
)

// setWindowsAutoStart 在非Windows平台模拟Windows自启动设置
func (h *SystemHandler) setWindowsAutoStart(executable string) error {
	logger.Info("在非Windows平台模拟设置Windows自启动")
	return h.setDummyAutoStart(executable)
}

// removeWindowsAutoStart 在非Windows平台模拟Windows自启动移除
func (h *SystemHandler) removeWindowsAutoStart() error {
	logger.Info("在非Windows平台模拟移除Windows自启动")
	return h.removeDummyAutoStart()
} 