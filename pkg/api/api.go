package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shuakami/clashrule-sync/pkg/utils"
)

// ClashAPI 提供了与 Clash API 进行通信的功能
type ClashAPI struct {
	baseURL string
	secret  string
	client  *http.Client
	// 添加缓存
	configCache     *ClashConfig
	configCacheTime time.Time
	mutex           sync.RWMutex
}

// ClashConfig 表示部分 Clash 配置
type ClashConfig struct {
	Mode string `json:"mode"`
}

// 配置缓存有效期
const configCacheTTL = 5 * time.Minute

// NewClashAPI 创建一个新的 Clash API 实例
func NewClashAPI(baseURL, secret string) *ClashAPI {
	// 使用工具函数规范化URL
	baseURL = utils.NormalizeURL(baseURL)

	// 创建带有超时的 HTTP 客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     60 * time.Second,
		},
	}

	return &ClashAPI{
		baseURL: baseURL,
		secret:  secret,
		client:  client,
	}
}

// TestConnection 测试与 Clash API 的连接
func (c *ClashAPI) TestConnection() (bool, error) {
	log.Printf("尝试连接到Clash API: %s", c.baseURL)

	// 尝试不同的API路径，以支持不同版本的Clash
	endpoints := []string{"/version", "/"}

	var lastErr error
	for _, endpoint := range endpoints {
		log.Printf("尝试API端点: %s", endpoint)

		// 发送请求
		resp, err := c.doRequest("GET", endpoint, nil)
		if err != nil {
			log.Printf("连接端点 %s 失败: %v", endpoint, err)
			lastErr = err
			continue
		}

		// 即使能连接，也检查下状态码
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("端点 %s 返回非200状态码: %d", endpoint, resp.StatusCode)
			lastErr = fmt.Errorf("API返回状态码: %d", resp.StatusCode)
			continue
		}

		// 读取并关闭响应体
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("读取端点 %s 响应失败: %v", endpoint, err)
			lastErr = err
			continue
		}

		log.Printf("成功连接到Clash API端点 %s，响应: %s", endpoint, string(body[:utils.Min(100, len(body))]))
		return true, nil
	}

	log.Printf("所有API端点连接尝试均失败，最后错误: %v", lastErr)
	return false, lastErr
}

// GetConfig 获取当前 Clash 配置（带缓存）
func (c *ClashAPI) GetConfig() (*ClashConfig, error) {
	c.mutex.RLock()
	// 检查缓存是否有效
	if c.configCache != nil && time.Since(c.configCacheTime) < configCacheTTL {
		config := *c.configCache // 复制一份返回，避免外部修改
		c.mutex.RUnlock()
		return &config, nil
	}
	c.mutex.RUnlock()

	// 缓存无效，需要请求新数据
	resp, err := c.doRequest("GET", "/configs", nil)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var config ClashConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("解析配置响应失败: %v", err)
	}

	// 更新缓存
	c.mutex.Lock()
	c.configCache = &config
	c.configCacheTime = time.Now()
	c.mutex.Unlock()

	return &config, nil
}

// ReloadConfig 重新加载 Clash 配置（仅更新本地文件，不再调用API）
func (c *ClashAPI) ReloadConfig() error {
	// 获取 CFW 设置文件路径
	settingsPath := GetClashCFWSettingsPath()
	if settingsPath == "" {
		return fmt.Errorf("未找到 CFW 设置文件")
	}

	log.Printf("已更新 Clash 配置文件: %s", settingsPath)
	log.Println("成功完成配置更新")

	// 清除配置缓存
	c.mutex.Lock()
	c.configCache = nil
	c.mutex.Unlock()

	return nil
}

// UpdateRuleProviders 更新规则提供者
func (c *ClashAPI) UpdateRuleProviders(providerNames []string) error {
	if len(providerNames) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(providerNames))

	for _, name := range providerNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			endpoint := fmt.Sprintf("/providers/rules/%s", name)
			resp, err := c.doRequest("PUT", endpoint, nil)
			if err != nil {
				errs <- fmt.Errorf("更新规则提供者 %s 失败: %v", name, err)
				return
			}
			defer resp.Body.Close()

			// 检查响应状态
			if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("更新规则提供者 %s 失败，状态码: %d", name, resp.StatusCode)
				return
			}

			log.Printf("成功更新规则提供者: %s", name)
		}(name)
	}

	// 等待所有更新完成
	wg.Wait()
	close(errs)

	// 收集所有错误
	var errMsgs []string
	for err := range errs {
		errMsgs = append(errMsgs, err.Error())
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf("部分规则提供者更新失败: %s", strings.Join(errMsgs, "; "))
	}

	return nil
}

// doRequest 执行 HTTP 请求
func (c *ClashAPI) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	// 构造完整的 URL
	url := c.baseURL + path

	// 如果有请求体，先读取内容用于日志
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			log.Printf("读取请求体失败: %v", err)
			return nil, err
		}

		if log.Default().Writer() != io.Discard { // 仅在开启日志时才记录
			log.Printf("请求体: %s", string(bodyBytes))
		}

		// 重新创建 reader
		body = bytes.NewBuffer(bodyBytes)
	}

	// 创建请求
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return nil, err
	}

	// 设置认证头
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	// 设置内容类型
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/json")
	}

	if log.Default().Writer() != io.Discard { // 仅在开启日志时才记录
		log.Printf("发送请求: %s %s", req.Method, req.URL)
	}

	// 执行请求
	return c.client.Do(req)
}

// SetRuleProviderConfig 设置规则提供者配置
func (c *ClashAPI) SetRuleProviderConfig(name, url, path, behavior, interval string) error {
	// 构造请求体
	reqBody := map[string]interface{}{
		"type":     "http",
		"url":      url,
		"path":     path,
		"behavior": behavior,
		"interval": interval,
	}

	// 将请求体转换为 JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %v", err)
	}

	// 构造请求路径
	endpoint := fmt.Sprintf("/providers/rules/%s", name)

	// 发送请求
	resp, err := c.doRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("设置规则提供者配置失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("设置规则提供者配置失败，状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Printf("成功设置规则提供者配置: %s", name)
	return nil
}

// DetectClashAPIConfig 自动检测Clash的API配置
func DetectClashAPIConfig() (string, int, string, error) {
	// 默认配置
	defaultURL := "http://127.0.0.1:9090"
	defaultPort := 9090
	defaultSecret := ""

	// 尝试常见的配置文件位置
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return defaultURL, defaultPort, defaultSecret, fmt.Errorf("获取用户主目录失败: %v", err)
	}

	// 尝试读取Clash配置文件
	configPaths := []string{
		filepath.Join(homeDir, ".config", "clash", "config.yaml"),
		filepath.Join(homeDir, ".config", "clash", "config.yml"),
		filepath.Join(os.Getenv("APPDATA"), "Clash for Windows", "config.yaml"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Clash for Windows", "config.yaml"),
	}

	for _, configPath := range configPaths {
		url, port, secret, err := readClashConfig(configPath)
		if err == nil {
			return url, port, secret, nil
		}
	}

	// 没有找到配置，返回默认值
	return defaultURL, defaultPort, defaultSecret, nil
}

// readClashConfig 从配置文件中读取Clash API配置
func readClashConfig(configPath string) (string, int, string, error) {
	// 默认配置
	defaultURL := "http://127.0.0.1:9090"
	defaultPort := 9090
	defaultSecret := ""

	// 检查文件是否存在
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		return defaultURL, defaultPort, defaultSecret, err
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return defaultURL, defaultPort, defaultSecret, err
	}

	// 使用正则表达式提取external-controller和secret
	reController := regexp.MustCompile(`external-controller:\s*([^#\r\n]+)`)
	reSecret := regexp.MustCompile(`secret:\s*([^#\r\n]+)`)

	// 提取控制器配置
	matchController := reController.FindSubmatch(data)
	if len(matchController) > 1 {
		controllerConfig := strings.TrimSpace(string(matchController[1]))

		// 解析地址和端口
		reAddr := regexp.MustCompile(`(127\.0\.0\.1|localhost):(\d+)`)
		matchAddr := reAddr.FindStringSubmatch(controllerConfig)

		if len(matchAddr) > 2 {
			host := matchAddr[1]
			port, err := strconv.Atoi(matchAddr[2])
			if err == nil {
				url := fmt.Sprintf("http://%s:%d", host, port)

				// 提取密钥配置
				matchSecret := reSecret.FindSubmatch(data)
				if len(matchSecret) > 1 {
					secret := strings.TrimSpace(string(matchSecret[1]))
					return url, port, secret, nil
				}

				return url, port, defaultSecret, nil
			}
		}
	}

	return defaultURL, defaultPort, defaultSecret, fmt.Errorf("无法从配置文件解析API配置")
}

// GetClashCFWSettingsPath 获取Clash for Windows设置文件路径
func GetClashCFWSettingsPath() string {
	// 默认路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("获取用户主目录失败: %v", err)
		return ""
	}

	// Clash for Windows设置文件路径
	cfwSettingsPath := filepath.Join(homeDir, ".config", "clash", "cfw-settings.yaml")

	// 检查文件是否存在
	if _, err := os.Stat(cfwSettingsPath); os.IsNotExist(err) {
		// 尝试备用路径
		cfwSettingsPath = filepath.Join(os.Getenv("APPDATA"), "Clash for Windows", "cfw-settings.yaml")
		if _, err := os.Stat(cfwSettingsPath); os.IsNotExist(err) {
			return ""
		}
	}

	return cfwSettingsPath
}

// ReadClashCFWSettings 读取Clash for Windows设置文件
func ReadClashCFWSettings(settingsPath string) (string, error) {
	// 如果路径为空，尝试获取默认路径
	if settingsPath == "" {
		settingsPath = GetClashCFWSettingsPath()
		if settingsPath == "" {
			return "", fmt.Errorf("未找到CFW设置文件")
		}
	}

	// 读取文件内容
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("读取CFW设置文件失败: %v", err)
	}

	return string(data), nil
}

// GetBypassRules 从CFW设置中获取绕过规则
func GetBypassRules(settingsContent string) (string, error) {
	// 使用正则表达式提取bypassText部分
	re := regexp.MustCompile(`(?m)^bypassText:\s*\|([\s\S]*?)(?:^[a-zA-Z]|$)`)
	matches := re.FindStringSubmatch(settingsContent)

	if len(matches) < 2 {
		return "", fmt.Errorf("未找到bypassText配置")
	}

	// 提取内容部分
	content := matches[1]
	content = strings.TrimSpace(content)

	return content, nil
}

// UpdateBypassRules 更新CFW设置中的绕过规则
func UpdateBypassRules(settingsPath string, newBypassRules string) error {
	// 读取当前设置文件
	settingsContent, err := ReadClashCFWSettings(settingsPath)
	if err != nil {
		return err
	}

	// 使用正则表达式匹配bypassText部分
	re := regexp.MustCompile(`(?m)(^bypassText:\s*\|)([\s\S]*?)((?:^[a-zA-Z]|$))`)

	// 确保新规则有正确的缩进格式
	indentedRules := ensureIndentation(newBypassRules, 2)

	// 替换bypassText内容
	if re.MatchString(settingsContent) {
		// 如果找到了bypassText，替换内容，确保下一行有换行和正确缩进
		updatedContent := re.ReplaceAllString(settingsContent, "${1}\n"+indentedRules+"\n$3")

		// 写入文件前验证格式
		verifyRe := regexp.MustCompile(`bypassText:\s*\|\n\s{2}bypass:`)
		if !verifyRe.MatchString(updatedContent) {
			log.Println("警告：生成的配置格式可能不正确，尝试修正...")

			// 如果格式不符合预期，尝试手动修正
			bypassFixRe := regexp.MustCompile(`(bypassText:\s*\|)\n(bypass:)`)
			if bypassFixRe.MatchString(updatedContent) {
				updatedContent = bypassFixRe.ReplaceAllString(updatedContent, "${1}\n  ${2}")
			}
		}

		// 写入文件
		err = os.WriteFile(settingsPath, []byte(updatedContent), 0644)
		if err != nil {
			return fmt.Errorf("写入CFW设置文件失败: %v", err)
		}

		log.Println("成功更新绕过规则")
		return nil
	}

	return fmt.Errorf("未找到bypassText配置")
}

// ensureIndentation 确保文本有正确的缩进
func ensureIndentation(text string, spaces int) string {
	lines := strings.Split(text, "\n")
	indent := strings.Repeat(" ", spaces)

	var result []string
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// 跳过空行
		if trimmedLine == "" {
			continue
		}

		if i == 0 && strings.HasPrefix(trimmedLine, "bypass:") {
			// 第一行 'bypass:' 保持正确缩进
			result = append(result, indent+"bypass:")
			continue
		}

		// 处理列表项
		if strings.HasPrefix(trimmedLine, "-") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "-"))
			// 移除可能的引号
			item = strings.Trim(item, "'\"")
			// 添加两个缩进单位(两倍的spaces)给列表项
			result = append(result, indent+indent+"- "+item)
		} else if !strings.HasPrefix(trimmedLine, "#") { // 不是注释
			// 如果是普通文本，保持其格式但确保正确缩进
			result = append(result, indent+trimmedLine)
		} else {
			// 是注释，保持原样但确保正确缩进
			result = append(result, indent+trimmedLine)
		}
	}

	return strings.Join(result, "\n")
}

// AddRuleToBypass 将规则添加到绕过配置中
func AddRuleToBypass(rule string) error {
	// 获取CFW设置文件路径
	settingsPath := GetClashCFWSettingsPath()
	if settingsPath == "" {
		return fmt.Errorf("未找到CFW设置文件")
	}

	// 读取当前设置
	settingsContent, err := ReadClashCFWSettings(settingsPath)
	if err != nil {
		return err
	}

	// 获取当前绕过规则
	bypassRules, err := GetBypassRules(settingsContent)
	if err != nil {
		return err
	}

	// 检查规则是否已存在
	if strings.Contains(bypassRules, rule) {
		return nil // 规则已存在，无需添加
	}

	// 解析现有规则结构
	lines := strings.Split(bypassRules, "\n")

	// 找到适合插入的位置
	insertPos := len(lines)
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			continue // 跳过注释和空行
		}

		if strings.HasPrefix(trimmedLine, "bypass:") {
			continue // 跳过bypass:行
		}

		// 默认在最后一个规则后插入
		insertPos = i + 1
	}

	// 插入新规则
	newRule := "  - '" + rule + "'"
	if insertPos >= len(lines) {
		lines = append(lines, newRule)
	} else {
		lines = append(lines[:insertPos], append([]string{newRule}, lines[insertPos:]...)...)
	}

	// 更新绕过规则
	updatedBypass := strings.Join(lines, "\n")
	return UpdateBypassRules(settingsPath, updatedBypass)
}

// SyncBypassRulesFromDomainList 从域名列表同步绕过规则
func SyncBypassRulesFromDomainList(domainRules string) error {
	// 获取CFW设置文件路径
	settingsPath := GetClashCFWSettingsPath()
	if settingsPath == "" {
		return fmt.Errorf("未找到CFW设置文件")
	}

	// 读取当前设置
	settingsContent, err := ReadClashCFWSettings(settingsPath)
	if err != nil {
		return err
	}

	log.Printf("成功读取CFW设置文件，大小: %d 字节", len(settingsContent))

	// 解析域名规则
	domains := parseDomainRules(domainRules)
	log.Printf("处理 %d 个域名规则", len(domains))

	// 构建新的绕过规则
	staticRules := []string{
		"localhost",
		"127.*",
		"10.*",
		"172.16.*",
		"172.17.*",
		"172.18.*",
		"172.19.*",
		"172.20.*",
		"172.21.*",
		"172.22.*",
		"172.23.*",
		"192.168.*",
		"<local>",
	}

	var bypassLines []string
	bypassLines = append(bypassLines, "bypass:")

	// 添加静态规则
	for _, rule := range staticRules {
		bypassLines = append(bypassLines, "  - "+rule)
	}

	// 添加域名规则
	for _, domain := range domains {
		bypassLines = append(bypassLines, "  - "+domain)
	}

	log.Printf("最终构建的绕过规则包含 %d 个静态规则和 %d 个域名规则", len(staticRules), len(domains))

	// 更新绕过规则
	newBypass := strings.Join(bypassLines, "\n")
	return UpdateBypassRules(settingsPath, newBypass)
}

// parseDomainRules 解析域名规则文本，支持多种格式
func parseDomainRules(rulesText string) []string {
	var domains []string
	lines := strings.Split(rulesText, "\n")

	inPayloadSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 检测payload部分开始
		if strings.HasPrefix(line, "payload:") {
			inPayloadSection = true
			continue
		}

		// 处理规则行
		if inPayloadSection || !strings.Contains(line, ":") {
			// 这可能是一个payload格式的域名项 (- 'domain.com')
			if strings.HasPrefix(line, "-") {
				domain := strings.TrimSpace(strings.TrimPrefix(line, "-"))
				domain = strings.Trim(domain, "'\"")
				if domain != "" {
					domains = append(domains, domain)
				}
			} else if !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "-") {
				// 可能是一个普通的域名行
				if domain := strings.TrimSpace(line); domain != "" {
					domains = append(domains, domain)
				}
			}
		}
	}

	return domains
}
