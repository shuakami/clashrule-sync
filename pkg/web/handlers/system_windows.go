//go:build windows
// +build windows

package handlers

import (
	"fmt"

	"golang.org/x/sys/windows/registry"

	"github.com/shuakami/clashrule-sync/pkg/logger"
)

// Windows系统自启动设置
func (h *SystemHandler) setWindowsAutoStart(executable string) error {
	// 创建启动项注册表键值
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.WRITE)
	if err != nil {
		return fmt.Errorf("打开注册表键失败: %v", err)
	}
	defer key.Close()

	// 设置启动项，带上命令行参数
	err = key.SetStringValue("ClashRuleSync", fmt.Sprintf(`"%s" -service`, executable))
	if err != nil {
		return fmt.Errorf("设置注册表键值失败: %v", err)
	}

	logger.Info("已添加Windows系统自启动项")
	return nil
}

func (h *SystemHandler) removeWindowsAutoStart() error {
	// 打开启动项注册表键值
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.WRITE)
	if err != nil {
		return fmt.Errorf("打开注册表键失败: %v", err)
	}
	defer key.Close()

	// 检查键值是否存在
	_, _, err = key.GetStringValue("ClashRuleSync")
	if err != nil {
		if err == registry.ErrNotExist {
			// 如果键值不存在，视为成功
			logger.Info("Windows系统自启动项不存在，无需移除")
			return nil
		}
		return fmt.Errorf("检查注册表键值失败: %v", err)
	}

	// 删除启动项
	err = key.DeleteValue("ClashRuleSync")
	if err != nil {
		if err == registry.ErrNotExist {
			// 如果删除时发现键值不存在，也视为成功
			return nil
		}
		return fmt.Errorf("删除注册表键值失败: %v", err)
	}

	logger.Info("已移除Windows系统自启动项")
	return nil
} 