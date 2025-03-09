package utils

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// GetHomeDir 获取用户主目录，如果获取失败则返回当前目录
func GetHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("获取用户主目录失败: %v，使用当前目录", err)
		return "."
	}
	return homeDir
}

// EnsureDirExists 确保目录存在，如果不存在则创建
func EnsureDirExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

// IsPortAvailable 检查指定端口是否可用
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// FindAvailablePort 查找可用端口，从指定端口开始尝试
func FindAvailablePort(startPort int) int {
	port := startPort
	for i := 0; i < 100; i++ { // 最多尝试100个端口
		if IsPortAvailable(port) {
			return port
		}
		port++
	}
	// 如果所有端口都不可用，返回原始端口（调用方需要处理错误）
	return startPort
}

// GetConfigDir 获取配置目录路径
func GetConfigDir() string {
	homeDir := GetHomeDir()
	return filepath.Join(homeDir, ".config", "clashrule-sync")
}

// NormalizeURL 标准化URL，确保以http://或https://开头
func NormalizeURL(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "http://" + url
	}
	return url
}

// Min 返回两个int中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CreateCommand 创建系统命令对象
func CreateCommand(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	return cmd
}

// IsWindows 检查是否运行在Windows系统上
func IsWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CreateBackgroundProcess 创建后台进程
func CreateBackgroundProcess(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	return cmd
}

// CreateHiddenWindowsProcess 创建一个在Windows上隐藏窗口的进程
func CreateHiddenWindowsProcess(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	
	// 使用平台特定的实现
	hideWindowsProcess(cmd)
	
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd
}
