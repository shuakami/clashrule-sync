package utils

import (
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GetHomeDir 获取用户主目录
func GetHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("获取用户主目录失败: %v，使用当前目录", err)
		dir, _ := os.Getwd()
		return dir
	}
	return home
}

// EnsureDirExists 确保目录存在，如果不存在则创建
func EnsureDirExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

// IsPortAvailable 检查端口是否可用
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// FindAvailablePort 从指定端口开始寻找可用端口
func FindAvailablePort(startPort int) int {
	port := startPort
	for port < 65535 {
		if IsPortAvailable(port) {
			return port
		}
		port++
	}
	// 如果找不到可用端口，返回原始端口并记录警告
	log.Printf("无法找到可用端口，返回原始端口: %d", startPort)
	return startPort
}

// GetConfigDir 获取配置目录
func GetConfigDir() string {
	homeDir := GetHomeDir()
	return filepath.Join(homeDir, ".config", "clashrule-sync")
}

// NormalizeURL 标准化URL格式
func NormalizeURL(url string) string {
	// 确保URL以http://或https://开头
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}
	return url
}

// Min 返回两个数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CreateCommand 创建系统命令
func CreateCommand(command string, args ...string) *exec.Cmd {
	return exec.Command(command, args...)
}

// IsWindows 判断当前系统是否是Windows
func IsWindows() bool {
	return filepath.Separator == '\\' && filepath.ListSeparator == ';'
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CreateBackgroundProcess 创建后台进程
func CreateBackgroundProcess(command string, args ...string) *exec.Cmd {
	return exec.Command(command, args...)
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

// GetExecutableDir 获取可执行文件所在目录
func GetExecutableDir() (string, error) {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	
	// 获取可执行文件所在目录
	execDir := filepath.Dir(execPath)
	return execDir, nil
}
