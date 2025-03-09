package common

import (
	"html/template"
	"path/filepath"
	"runtime"

	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/utils"
)

// 定义模板缓存结构
var templateCache = make(map[string]*template.Template)
var templateCacheMutex = make(chan struct{}, 1)

// LoadTemplate 加载模板文件并缓存
func LoadTemplate(templateName string) (*template.Template, error) {
	// 使用通道实现互斥锁
	templateCacheMutex <- struct{}{}
	defer func() { <-templateCacheMutex }()

	// 检查缓存
	if tmpl, ok := templateCache[templateName]; ok {
		return tmpl, nil
	}

	// 加载模板
	templatePath := filepath.Join("templates", templateName)
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, err
	}

	// 缓存模板
	templateCache[templateName] = tmpl
	return tmpl, nil
}

// GetRulesDir 获取规则目录
func GetRulesDir() string {
	configDir := utils.GetConfigDir()
	return filepath.Join(configDir, "rules")
}

// EnsureRulesDir 确保规则目录存在
func EnsureRulesDir() (string, error) {
	rulesDir := GetRulesDir()
	err := utils.EnsureDirExists(rulesDir)
	return rulesDir, err
}

// OpenBrowser 打开浏览器
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		logger.Warnf("不支持自动打开浏览器的平台: %s", runtime.GOOS)
		return nil
	}

	return runCommand(cmd, args...)
}

// runCommand 执行系统命令，避免在各处创建cmd对象
func runCommand(command string, args ...string) error {
	cmd := utils.CreateCommand(command, args...)
	return cmd.Start()
} 