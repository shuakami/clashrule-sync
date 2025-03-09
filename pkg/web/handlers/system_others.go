//go:build !windows
// +build !windows

package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/utils"
)

// 为非 Windows 平台提供空实现
func (h *SystemHandler) setWindowsAutoStart(executable string) error {
	logger.Warn("Windows自启动设置不适用于当前平台")
	return fmt.Errorf("不支持的操作系统")
}

func (h *SystemHandler) removeWindowsAutoStart() error {
	logger.Warn("Windows自启动设置不适用于当前平台")
	return fmt.Errorf("不支持的操作系统")
}

// macOS系统自启动设置
func (h *SystemHandler) setMacOSAutoStart(executable string) error {
	homeDir := utils.GetHomeDir()
	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")

	// 确保LaunchAgents目录存在
	err := utils.EnsureDirExists(launchAgentsDir)
	if err != nil {
		return fmt.Errorf("创建LaunchAgents目录失败: %v", err)
	}

	plistPath := filepath.Join(launchAgentsDir, "com.shuakami.clashrule-sync.plist")

	// 创建plist文件内容
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.shuakami.clashrule-sync</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>`, executable)

	// 写入plist文件
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("写入plist文件失败: %v", err)
	}

	// 加载plist文件
	loadCmd := exec.Command("launchctl", "load", plistPath)
	if err := loadCmd.Run(); err != nil {
		logger.Warnf("加载启动项警告: %v", err)
		// 继续执行，因为plist文件已创建，下次登录时会生效
	}

	logger.Info("已添加macOS系统自启动项")
	return nil
}

func (h *SystemHandler) removeMacOSAutoStart() error {
	homeDir := utils.GetHomeDir()
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.shuakami.clashrule-sync.plist")

	// 检查文件是否存在
	if _, err := os.Stat(plistPath); err == nil {
		// 卸载plist文件
		unloadCmd := exec.Command("launchctl", "unload", plistPath)
		if err := unloadCmd.Run(); err != nil {
			logger.Warnf("卸载启动项警告: %v", err)
		}

		// 删除plist文件
		if err := os.Remove(plistPath); err != nil {
			logger.Warnf("删除plist文件警告: %v", err)
		}
	}

	logger.Info("已移除macOS系统自启动项")
	return nil
}

// Linux系统自启动设置
func (h *SystemHandler) setLinuxAutoStart(executable string) error {
	homeDir := utils.GetHomeDir()

	// 创建.config/autostart目录
	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	err := utils.EnsureDirExists(autostartDir)
	if err != nil {
		return fmt.Errorf("创建autostart目录失败: %v", err)
	}

	desktopFilePath := filepath.Join(autostartDir, "clashrule-sync.desktop")

	// 创建desktop文件内容
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=ClashRuleSync
Exec=%s
Terminal=false
StartupNotify=false
Hidden=false
X-GNOME-Autostart-enabled=true`, executable)

	// 写入desktop文件
	if err := os.WriteFile(desktopFilePath, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("写入desktop文件失败: %v", err)
	}

	logger.Info("已添加Linux系统自启动项")
	return nil
}

func (h *SystemHandler) removeLinuxAutoStart() error {
	homeDir := utils.GetHomeDir()
	desktopFilePath := filepath.Join(homeDir, ".config", "autostart", "clashrule-sync.desktop")

	// 检查文件是否存在
	if _, err := os.Stat(desktopFilePath); err == nil {
		// 删除desktop文件
		if err := os.Remove(desktopFilePath); err != nil {
			return fmt.Errorf("删除desktop文件失败: %v", err)
		}
	}

	logger.Info("已移除Linux系统自启动项")
	return nil
} 