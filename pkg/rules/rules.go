package rules

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/utils"
)

// RuleUpdater 负责更新规则
type RuleUpdater struct {
	cfg           *config.Config
	updateHistory []UpdateRecord
	mutex         sync.RWMutex
	// 添加HTTP客户端，避免每次创建新的
	client *http.Client
}

// UpdateRecord 记录规则更新历史
type UpdateRecord struct {
	Time      time.Time        `json:"time"`
	Providers []ProviderRecord `json:"providers"`
}

// ProviderRecord 记录单个规则提供者的更新情况
type ProviderRecord struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NewRuleUpdater 创建一个新的规则更新器
func NewRuleUpdater(cfg *config.Config) *RuleUpdater {
	// 创建一个自定义的HTTP客户端用于规则下载
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     60 * time.Second,
		},
	}

	return &RuleUpdater{
		cfg:           cfg,
		updateHistory: []UpdateRecord{},
		client:        client,
	}
}

// GetUpdateHistory 获取更新历史
func (ru *RuleUpdater) GetUpdateHistory() []UpdateRecord {
	ru.mutex.RLock()
	defer ru.mutex.RUnlock()

	// 返回副本避免外部修改
	history := make([]UpdateRecord, len(ru.updateHistory))
	copy(history, ru.updateHistory)
	return history
}

// UpdateAllRules 更新所有规则
func (ru *RuleUpdater) UpdateAllRules() (bool, error) {
	ru.mutex.Lock()
	defer ru.mutex.Unlock()

	logger.Info("开始更新所有规则...")

	// 创建一个新的更新记录
	record := UpdateRecord{
		Time:      time.Now(),
		Providers: []ProviderRecord{},
	}

	// 创建规则目录
	rulesDir := ru.getRulesDir()
	if err := utils.EnsureDirExists(rulesDir); err != nil {
		return false, fmt.Errorf("创建规则目录失败: %v", err)
	}

	allSuccess := true

	// 用于收集所有规则内容
	var allRuleContents []string

	// 并行下载规则
	var wg sync.WaitGroup

	// 使用channel传递结果
	type ruleResult struct {
		provider    config.RuleProvider
		record      ProviderRecord
		ruleContent string
	}
	resultChan := make(chan ruleResult, len(ru.cfg.RuleProviders))

	// 遍历所有规则提供者
	for _, provider := range ru.cfg.RuleProviders {
		if !provider.Enabled {
			continue
		}

		wg.Add(1)
		go func(provider config.RuleProvider) {
			defer wg.Done()

			logger.Infof("更新规则: %s", provider.Name)

			providerRecord := ProviderRecord{
				Name:    provider.Name,
				Success: false,
				Message: "",
			}

			// 下载并处理规则
			ruleFilePath := filepath.Join(rulesDir, provider.Path)

			var ruleContent string
			err := ru.downloadAndProcessRule(provider, ruleFilePath)
			if err != nil {
				logger.Errorf("更新规则 %s 失败: %v", provider.Name, err)
				providerRecord.Message = err.Error()
				resultChan <- ruleResult{provider, providerRecord, ""}
				return
			}

			logger.Infof("规则 %s 更新成功", provider.Name)
			providerRecord.Success = true
			providerRecord.Message = "更新成功"

			// 读取所有规则文件内容
			content, err := os.ReadFile(ruleFilePath)
			if err == nil {
				ruleContent = string(content)
				logger.Infof("已添加规则 %s 到规则列表，规则大小: %d", provider.Name, len(content))
			} else {
				logger.Errorf("读取规则文件 %s 失败: %v", ruleFilePath, err)
			}

			resultChan <- ruleResult{provider, providerRecord, ruleContent}
		}(provider)
	}

	// 等待所有下载完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	for result := range resultChan {
		record.Providers = append(record.Providers, result.record)
		if !result.record.Success {
			allSuccess = false
		}
		if result.ruleContent != "" {
			allRuleContents = append(allRuleContents, result.ruleContent)
		}
	}

	// 更新最后一次更新时间
	ru.cfg.UpdateLastUpdateTime()

	// 将记录添加到更新历史
	ru.updateHistory = append(ru.updateHistory, record)
	if len(ru.updateHistory) > 10 {
		ru.updateHistory = ru.updateHistory[len(ru.updateHistory)-10:]
	}

	// 无条件同步所有规则
	if len(allRuleContents) > 0 {
		combinedRules := strings.Join(allRuleContents, "\n")
		logger.Infof("同步所有规则到CFW绕过配置，规则总数: %d", len(allRuleContents))
		err := api.SyncBypassRulesFromDomainList(combinedRules)
		if err != nil {
			logger.Errorf("同步规则到CFW绕过配置失败: %v", err)
		} else {
			logger.Info("成功将规则同步到CFW绕过配置")
		}
	}

	logger.Info("规则更新完成")

	return allSuccess, nil
}

// UpdateRuleProvider 更新单个规则提供者
func (ru *RuleUpdater) UpdateRuleProvider(providerName string) (bool, error) {
	ru.mutex.Lock()
	defer ru.mutex.Unlock()

	// 查找指定的规则提供者
	var provider *config.RuleProvider
	for i := range ru.cfg.RuleProviders {
		if ru.cfg.RuleProviders[i].Name == providerName {
			provider = &ru.cfg.RuleProviders[i]
			break
		}
	}

	if provider == nil {
		return false, fmt.Errorf("未找到规则提供者: %s", providerName)
	}

	if !provider.Enabled {
		return false, fmt.Errorf("规则提供者已禁用: %s", providerName)
	}

	logger.Infof("更新规则: %s", provider.Name)

	// 创建规则目录
	rulesDir := ru.getRulesDir()
	if err := utils.EnsureDirExists(rulesDir); err != nil {
		return false, fmt.Errorf("创建规则目录失败: %v", err)
	}

	// 下载并处理规则
	ruleFilePath := filepath.Join(rulesDir, provider.Path)
	err := ru.downloadAndProcessRule(*provider, ruleFilePath)
	if err != nil {
		// 记录更新历史
		ru.recordUpdateHistory(provider.Name, false, err.Error())
		return false, err
	}

	// 如果这是直连域名规则，同步到CFW的绕过配置
	if (provider.Name == "cn_domain" || strings.Contains(provider.Name, "direct")) &&
		(provider.Type == "domain" || provider.Type == "mixed") {
		// 读取规则文件内容
		ruleContent, err := os.ReadFile(ruleFilePath)
		if err == nil {
			// 尝试同步到CFW绕过配置
			err := api.SyncBypassRulesFromDomainList(string(ruleContent))
			if err != nil {
				logger.Errorf("同步直连规则到CFW绕过配置失败: %v", err)
			} else {
				logger.Info("成功将直连规则同步到CFW绕过配置")
			}
		}
	}

	// 记录更新历史
	ru.recordUpdateHistory(provider.Name, true, "更新成功")

	return true, nil
}

// recordUpdateHistory 记录单个规则的更新历史
func (ru *RuleUpdater) recordUpdateHistory(name string, success bool, message string) {
	// 查找最新的记录
	var record *UpdateRecord
	if len(ru.updateHistory) > 0 {
		// 如果最新记录不到1分钟，则更新该记录
		latestRecord := &ru.updateHistory[len(ru.updateHistory)-1]
		if time.Since(latestRecord.Time) < time.Minute {
			record = latestRecord
		}
	}

	// 如果没有合适的记录，创建新记录
	if record == nil {
		newRecord := UpdateRecord{
			Time:      time.Now(),
			Providers: []ProviderRecord{},
		}
		ru.updateHistory = append(ru.updateHistory, newRecord)
		record = &ru.updateHistory[len(ru.updateHistory)-1]

		// 限制历史记录数量
		if len(ru.updateHistory) > 10 {
			ru.updateHistory = ru.updateHistory[len(ru.updateHistory)-10:]
		}
	}

	// 添加或更新提供者记录
	providerRecord := ProviderRecord{
		Name:    name,
		Success: success,
		Message: message,
	}

	// 检查是否已存在此提供者的记录
	for i, pr := range record.Providers {
		if pr.Name == name {
			record.Providers[i] = providerRecord
			return
		}
	}

	// 不存在则添加新记录
	record.Providers = append(record.Providers, providerRecord)
}

// getRulesDir 获取规则目录
func (ru *RuleUpdater) getRulesDir() string {
	// 获取用户主目录
	configDir := utils.GetConfigDir()
	return filepath.Join(configDir, "rules")
}

// downloadAndProcessRule 下载并处理规则
func (ru *RuleUpdater) downloadAndProcessRule(provider config.RuleProvider, outputPath string) error {
	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := utils.EnsureDirExists(outputDir); err != nil {
		return errors.Wrap(err, "创建输出目录失败")
	}

	// 下载规则文件
	resp, err := ru.client.Get(provider.URL)
	if err != nil {
		return errors.Wrap(err, "下载规则失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载规则失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "读取响应内容失败")
	}

	// 处理规则内容
	content := string(body)

	// 检查内容是否已经是YAML格式
	if strings.Contains(content, "payload:") ||
		strings.Contains(content, "bypass:") ||
		strings.Contains(content, "domain:") ||
		strings.Contains(content, "ip-cidr:") {
		// 已经是YAML格式，直接写入
		err = os.WriteFile(outputPath, body, 0644)
		if err != nil {
			return errors.Wrap(err, "写入规则文件失败")
		}
		return nil
	}

	// 根据类型处理规则
	var processedRules string
	switch provider.Type {
	case "domain":
		processedRules = processDomainRules(content, provider.Name)
	case "ipcidr":
		processedRules = processIPCIDRRules(content, provider.Name)
	case "mixed":
		processedRules = processMixedRules(content, provider.Name)
	default:
		// 默认作为混合规则处理
		processedRules = processMixedRules(content, provider.Name)
	}

	// 写入文件
	err = os.WriteFile(outputPath, []byte(processedRules), 0644)
	if err != nil {
		return errors.Wrap(err, "写入规则文件失败")
	}

	return nil
}

// processDomainRules 处理域名规则
func processDomainRules(content, providerName string) string {
	lines := strings.Split(content, "\n")
	var domains []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, line)
	}

	// 构造YAML格式的域名规则
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("payload:\n"))

	// 如果没有有效规则，添加注释
	if len(domains) == 0 {
		builder.WriteString(fmt.Sprintf("  # 空规则文件 - %s\n", providerName))
	} else {
		for _, domain := range domains {
			builder.WriteString(fmt.Sprintf("  - '%s'\n", domain))
		}
	}

	return builder.String()
}

// processIPCIDRRules 处理IP CIDR规则
func processIPCIDRRules(content, providerName string) string {
	lines := strings.Split(content, "\n")
	var ipCidrs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ipCidrs = append(ipCidrs, line)
	}

	// 构造YAML格式的IP CIDR规则
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("payload:\n"))

	// 如果没有有效规则，添加注释
	if len(ipCidrs) == 0 {
		builder.WriteString(fmt.Sprintf("  # 空规则文件 - %s\n", providerName))
	} else {
		for _, ipCidr := range ipCidrs {
			builder.WriteString(fmt.Sprintf("  - '%s'\n", ipCidr))
		}
	}

	return builder.String()
}

// processMixedRules 处理混合规则（同时包含域名和IP）
func processMixedRules(content, providerName string) string {
	lines := strings.Split(content, "\n")
	var rules []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rules = append(rules, line)
	}

	// 构造YAML格式的混合规则
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("payload:\n"))

	// 如果没有有效规则，添加注释
	if len(rules) == 0 {
		builder.WriteString(fmt.Sprintf("  # 空规则文件 - %s\n", providerName))
	} else {
		for _, rule := range rules {
			builder.WriteString(fmt.Sprintf("  - '%s'\n", rule))
		}
	}

	return builder.String()
}
