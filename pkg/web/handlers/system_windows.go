//go:build windows
// +build windows

package handlers

import (
	"fmt"

	"github.com/shuakami/clashrule-sync/pkg/logger"
	"golang.org/x/sys/windows/registry"
)

// setWindowsAutoStart 在Windows中设置应用程序自启动
func (h *SystemHandler) setWindowsAutoStart(executable string) error {
	logger.Info("在Windows中设置自启动")
	// 使用Windows注册表实现自启动
	key := `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	appName := "ClashRuleSync"
	
	// 使用golang.org/x/sys/windows/registry包访问注册表
	k, err := registry.OpenKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表键失败: %v", err)
	}
	defer k.Close()

	// 设置自启动值
	err = k.SetStringValue(appName, executable)
	if err != nil {
		return fmt.Errorf("设置注册表值失败: %v", err)
	}

	logger.Info("Windows自启动设置成功")
	return nil
}

// removeWindowsAutoStart 在Windows中移除应用程序自启动
func (h *SystemHandler) removeWindowsAutoStart() error {
	logger.Info("在Windows中移除自启动")
	// 使用Windows注册表删除自启动
	key := `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	appName := "ClashRuleSync"
	
	// 打开注册表键
	k, err := registry.OpenKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表键失败: %v", err)
	}
	defer k.Close()

	// 删除自启动值
	err = k.DeleteValue(appName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("删除注册表值失败: %v", err)
	}

	logger.Info("Windows自启动移除成功")
	return nil
} 