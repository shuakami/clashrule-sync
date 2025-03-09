package common

import (
	"html/template"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

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

	// 获取执行文件所在目录
	execDir, err := utils.GetExecutableDir()
	if err != nil {
		logger.Errorf("获取执行文件目录失败: %v", err)
		// 尝试使用相对路径作为后备方案
		execDir = "."
	}

	// 尝试多个可能的路径
	templatePaths := []string{
		filepath.Join(execDir, "templates", templateName),        // 1. 可执行文件目录下的templates
		filepath.Join("templates", templateName),                 // 2. 当前工作目录下的templates
		filepath.Join(execDir, "..", "templates", templateName),  // 3. 可执行文件上级目录的templates
	}

	// 尝试加载模板
	var lastErr error
	for _, path := range templatePaths {
		tmpl, err := template.ParseFiles(path)
		if err == nil {
			// 缓存模板
			templateCache[templateName] = tmpl
			return tmpl, nil
		}
		lastErr = err
		logger.Debugf("尝试加载模板 %s 失败: %v", path, err)
	}

	// 所有路径都失败了，返回最后一个错误
	return nil, lastErr
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

// MakeSafeFilename 创建安全的文件名，移除非法字符
func MakeSafeFilename(name string) string {
	// 移除或替换非法字符
	reg := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	safe := reg.ReplaceAllString(name, "_")
	
	// 去除前后空格
	safe = strings.TrimSpace(safe)
	
	// 确保文件名不为空
	if safe == "" {
		safe = "rule"
	}
	
	return safe
}

// ResolvePath 解析相对路径为绝对路径
func ResolvePath(path string) string {
	// 如果是绝对路径，直接返回
	if filepath.IsAbs(path) {
		return path
	}
	
	// 如果是相对于规则目录的路径
	if !strings.HasPrefix(path, "rules/") && !strings.HasPrefix(path, "rules\\") {
		// 获取规则目录
		rulesDir := GetRulesDir()
		return filepath.Join(rulesDir, path)
	}
	
	// 如果已经是相对于规则目录的路径，去掉前缀
	path = strings.TrimPrefix(path, "rules/")
	path = strings.TrimPrefix(path, "rules\\")
	
	// 获取规则目录
	rulesDir := GetRulesDir()
	return filepath.Join(rulesDir, path)
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
