package handlers

import (
	"fmt"
	"os"
	"runtime"

	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
)

// SystemHandler 处理系统相关操作，如自启动
type SystemHandler struct {
	Config *config.Config
}

// NewSystemHandler 创建系统处理器
func NewSystemHandler(cfg *config.Config) *SystemHandler {
	return &SystemHandler{
		Config: cfg,
	}
}

// HandleSystemAutoStart 应用系统自启动设置
func (h *SystemHandler) HandleSystemAutoStart() error {
	// 获取可执行文件路径
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	if h.Config.SystemAutoStartEnabled {
		// 启用系统自启动
		return h.SetSystemAutoStart(executable)
	} else {
		// 禁用系统自启动
		return h.RemoveSystemAutoStart()
	}
}

// SetSystemAutoStart 设置应用程序随系统启动
func (h *SystemHandler) SetSystemAutoStart(executable string) error {
	switch runtime.GOOS {
	case "windows":
		return h.setWindowsAutoStart(executable)
	case "darwin":
		return h.setMacOSAutoStart(executable)
	case "linux":
		return h.setLinuxAutoStart(executable)
	default:
		logger.Warnf("不支持的操作系统: %s", runtime.GOOS)
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// RemoveSystemAutoStart 移除应用程序随系统启动
func (h *SystemHandler) RemoveSystemAutoStart() error {
	switch runtime.GOOS {
	case "windows":
		return h.removeWindowsAutoStart()
	case "darwin":
		return h.removeMacOSAutoStart()
	case "linux":
		return h.removeLinuxAutoStart()
	default:
		logger.Warnf("不支持的操作系统: %s", runtime.GOOS)
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// 各平台的实现
func (h *SystemHandler) setWindowsAutoStart(executable string) error {
	logger.Info("在Windows中设置自启动")
	// 在Windows中的具体实现会在编译时选择
	return h.setDummyAutoStart(executable)
}

func (h *SystemHandler) removeWindowsAutoStart() error {
	logger.Info("在Windows中移除自启动")
	// 在Windows中的具体实现会在编译时选择
	return h.removeDummyAutoStart()
}

func (h *SystemHandler) setMacOSAutoStart(executable string) error {
	logger.Info("在macOS中设置自启动")
	return h.setDummyAutoStart(executable)
}

func (h *SystemHandler) removeMacOSAutoStart() error {
	logger.Info("在macOS中移除自启动")
	return h.removeDummyAutoStart()
}

func (h *SystemHandler) setLinuxAutoStart(executable string) error {
	logger.Info("在Linux中设置自启动")
	return h.setDummyAutoStart(executable)
}

func (h *SystemHandler) removeLinuxAutoStart() error {
	logger.Info("在Linux中移除自启动")
	return h.removeDummyAutoStart()
}

// 通用的空实现
func (h *SystemHandler) setDummyAutoStart(executable string) error {
	logger.Info("模拟设置自启动: " + executable)
	return nil
}

func (h *SystemHandler) removeDummyAutoStart() error {
	logger.Info("模拟移除自启动")
	return nil
} 