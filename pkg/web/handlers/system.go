package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows/registry"

	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/utils"
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

	// 另一种实现方式：通过systemd用户单元（可选）
	systemdUserDir := filepath.Join(homeDir, ".config", "systemd", "user")

	// 检查systemd用户目录是否存在
	if _, err := os.Stat(systemdUserDir); err == nil {
		serviceFilePath := filepath.Join(systemdUserDir, "clashrule-sync.service")

		// 创建systemd服务文件内容
		serviceContent := fmt.Sprintf(`[Unit]
Description=ClashRuleSync - Clash Rule Automatic Sync Tool
After=network.target

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target`, executable)

		// 写入服务文件
		if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0644); err != nil {
			logger.Warnf("写入systemd服务文件警告: %v", err)
		} else {
			// 启用服务
			enableCmd := exec.Command("systemctl", "--user", "enable", "clashrule-sync.service")
			if err := enableCmd.Run(); err != nil {
				logger.Warnf("启用systemd服务警告: %v", err)
			}

			// 尝试启动服务
			startCmd := exec.Command("systemctl", "--user", "start", "clashrule-sync.service")
			if err := startCmd.Run(); err != nil {
				logger.Warnf("启动systemd服务警告: %v", err)
			}

			logger.Info("已添加Linux systemd用户服务")
		}
	}

	return nil
}

func (h *SystemHandler) removeLinuxAutoStart() error {
	homeDir := utils.GetHomeDir()

	// 删除desktop文件
	desktopFilePath := filepath.Join(homeDir, ".config", "autostart", "clashrule-sync.desktop")
	if _, err := os.Stat(desktopFilePath); err == nil {
		if err := os.Remove(desktopFilePath); err != nil {
			logger.Warnf("删除desktop文件警告: %v", err)
		}
	}

	// 检查并移除systemd服务
	systemdUserDir := filepath.Join(homeDir, ".config", "systemd", "user")
	serviceFilePath := filepath.Join(systemdUserDir, "clashrule-sync.service")

	if _, err := os.Stat(serviceFilePath); err == nil {
		// 停止并禁用服务
		stopCmd := exec.Command("systemctl", "--user", "stop", "clashrule-sync.service")
		if err := stopCmd.Run(); err != nil {
			logger.Warnf("停止systemd服务警告: %v", err)
		}

		disableCmd := exec.Command("systemctl", "--user", "disable", "clashrule-sync.service")
		if err := disableCmd.Run(); err != nil {
			logger.Warnf("禁用systemd服务警告: %v", err)
		}

		// 删除服务文件
		if err := os.Remove(serviceFilePath); err != nil {
			logger.Warnf("删除systemd服务文件警告: %v", err)
		}
	}

	logger.Info("已移除Linux系统自启动项")
	return nil
}
