package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/rules"
	"github.com/shuakami/clashrule-sync/pkg/web/common"
)

// PageHandler 处理页面相关的请求
type PageHandler struct {
	Config      *config.Config
	RuleUpdater *rules.RuleUpdater
	ClashAPI    *api.ClashAPI
	Version     string
}

// NewPageHandler 创建页面处理器
func NewPageHandler(cfg *config.Config, ruleUpdater *rules.RuleUpdater, clashAPI *api.ClashAPI, version string) *PageHandler {
	return &PageHandler{
		Config:      cfg,
		RuleUpdater: ruleUpdater,
		ClashAPI:    clashAPI,
		Version:     version,
	}
}

// HandleIndex 处理首页请求
func (h *PageHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// 如果是首次运行，重定向到设置向导
	if h.Config.FirstRun {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	// 读取模板文件
	tmpl, err := common.LoadTemplate("index.html")
	if err != nil {
		logger.Errorf("解析模板失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 准备模板数据
	data := map[string]interface{}{
		"Version":        h.Version,
		"BuildID":        time.Now().Format("060102"),
		"LastUpdateTime": h.Config.LastUpdateTime.Format("2006-01-02 15:04:05"),
		"NextUpdateTime": h.Config.LastUpdateTime.Add(h.Config.UpdateInterval).Format("2006-01-02 15:04:05"),
		"AutoStartEnabled": h.Config.AutoStartEnabled,
		"SystemAutoStartEnabled": h.Config.SystemAutoStartEnabled,
	}

	// 渲染模板
	err = tmpl.Execute(w, data)
	if err != nil {
		logger.Errorf("渲染模板失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}

// HandleSetupPage 处理设置向导页面请求
func (h *PageHandler) HandleSetupPage(w http.ResponseWriter, r *http.Request) {
	// 读取模板文件
	tmpl, err := common.LoadTemplate("setup.html")
	if err != nil {
		logger.Errorf("解析设置向导模板失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 准备模板数据
	data := map[string]interface{}{
		"Version": h.Version,
	}

	// 渲染模板
	err = tmpl.Execute(w, data)
	if err != nil {
		logger.Errorf("渲染设置向导模板失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// HandleTestConnection 处理测试API连接请求
func (h *PageHandler) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 解析请求体
	var req struct {
		ClashAPIURL    string `json:"clash_api_url"`
		ClashAPISecret string `json:"clash_api_secret"`
	}
	
	if !common.ParseJSON(w, r, &req) {
		return
	}

	// 创建临时API客户端来测试连接
	tempAPI := api.NewClashAPI(req.ClashAPIURL, req.ClashAPISecret)
	
	// 测试连接
	connected, err := tempAPI.TestConnection()
	
	// 构建响应
	resp := map[string]interface{}{
		"status":    "ok",
		"connected": connected,
	}
	
	// 如果连接失败，添加错误信息
	if !connected {
		resp["message"] = "无法连接到Clash API"
		if err != nil {
			resp["error"] = err.Error()
		}
	}

	// 返回响应
	common.SendJSONResponse(w, resp)
}

// HandleSetup 处理设置向导请求
func (h *PageHandler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	logger.Println("===== Setup 流程开始 =====")
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 读取并记录原始请求体
	var rawBody []byte
	rawBody, _ = io.ReadAll(r.Body)
	r.Body.Close()
	logger.Printf("Setup 原始请求体: %s", string(rawBody))
	
	// 重新创建请求体以供后续处理
	r.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	// 解析请求
	var req struct {
		ClashAPIURL            string `json:"clashApiUrl"`           // 修正字段名与前端匹配
		ClashAPISecret         string `json:"clashApiSecret"`        
		UpdateInterval         int64  `json:"updateInterval"`        // 小写开头
		AutoStartEnabled       bool   `json:"autoStartEnabled"`      // 小写开头
		SystemAutoStartEnabled bool   `json:"systemAutoStartEnabled"`// 小写开头
		SelectedRules          []struct {  // 接收前端发送的规则列表
			Name string `json:"name"`
			URL  string `json:"url"`
			Type string `json:"type"`
		} `json:"selectedRules"`
	}

	if !common.ParseJSON(w, r, &req) {
		return
	}
	
	logger.Printf("Setup 请求数据: API URL=%s, 更新间隔=%d小时, 自启动=%v, 系统自启动=%v", 
		req.ClashAPIURL, req.UpdateInterval, req.AutoStartEnabled, req.SystemAutoStartEnabled)
	logger.Printf("Setup 选中规则数量: %d", len(req.SelectedRules))

	// 更新配置
	logger.Println("Setup: 更新配置...")
	h.Config.ClashAPIURL = req.ClashAPIURL
	h.Config.ClashAPISecret = req.ClashAPISecret
	h.Config.UpdateInterval = time.Duration(req.UpdateInterval) * time.Hour  // 将小时转换为 Duration
	h.Config.AutoStartEnabled = req.AutoStartEnabled
	h.Config.SystemAutoStartEnabled = req.SystemAutoStartEnabled
	h.Config.FirstRun = false  // 设置标记表示已完成首次设置

	// 根据规则设置更新规则提供者
	// 清空现有规则
	logger.Println("Setup: 清空现有规则提供者...")
	h.Config.RuleProviders = []config.RuleProvider{}

	// 添加用户选择的规则
	logger.Println("Setup: 添加用户选择的规则...")
	for _, rule := range req.SelectedRules {
		logger.Printf("Setup: 添加规则 %s, URL=%s, 类型=%s", rule.Name, rule.URL, rule.Type)
		
		// 根据规则类型确定behavior
		behavior := rule.Type
		if behavior == "classical" {
			behavior = "classical"
		}
		
		// 生成规则路径
		ruleName := strings.ReplaceAll(rule.Name, " ", "_")
		ruleName = strings.ReplaceAll(ruleName, ":", "")
		ruleName = strings.ReplaceAll(ruleName, "/", "_")
		rulePath := fmt.Sprintf("rules/%s.yaml", ruleName)
		
		// 添加规则
		h.Config.RuleProviders = append(h.Config.RuleProviders, config.RuleProvider{
			Name:     ruleName,
			URL:      rule.URL,
			Type:     rule.Type,
			Behavior: behavior,
			Path:     rulePath,
			Enabled:  true,
		})
	}

	// 保存配置
	logger.Printf("Setup: 保存配置到 %s...", config.GetConfigPath())
	if err := h.Config.SaveConfig(); err != nil {
		logger.Printf("Setup 错误: 保存配置失败: %v", err)
		common.SendInternalError(w, "保存配置失败", err)
		return
	}
	logger.Println("Setup: 配置保存成功")

	// 创建规则目录
	rulesDir := common.GetRulesDir()
	logger.Printf("Setup: 检查规则目录 %s...", rulesDir)
	var err error
	rulesDir, err = common.EnsureRulesDir()
	if err != nil {
		logger.Printf("Setup: 警告: 创建规则目录失败: %v", err)
	} else {
		logger.Println("Setup: 规则目录检查/创建成功")
	}

	// 确保所有规则都正确标记了 Enabled 状态
	logger.Println("Setup: 确保所有规则都已启用...")
	for i := range h.Config.RuleProviders {
		h.Config.RuleProviders[i].Enabled = true
	}

	// 再次保存配置以确保规则目录和规则启用状态被保存
	logger.Println("Setup: 再次保存更新后的配置...")
	if err := h.Config.SaveConfig(); err != nil {
		logger.Printf("Setup 警告: 保存更新后的配置失败: %v", err)
	}

	// 只有在用户选择了规则时才更新规则
	if len(h.Config.RuleProviders) > 0 {
		logger.Printf("Setup: 发现 %d 个规则提供者，开始更新规则...", len(h.Config.RuleProviders))
		// 立即更新规则
		success, err := h.RuleUpdater.UpdateAllRules()
		if err != nil {
			logger.Printf("Setup 警告: 首次更新规则失败: %v", err)
		} else if success {
			logger.Println("Setup: 首次更新规则成功")
		} else {
			logger.Println("Setup: 首次更新规则部分成功，部分失败")
		}

		// 尝试重新加载 Clash 配置
		logger.Println("Setup: 尝试重新加载 Clash 配置...")
		err = h.ClashAPI.ReloadConfig()
		if err != nil {
			logger.Printf("Setup 警告: 重新加载 Clash 配置失败: %v", err)
		} else {
			logger.Println("Setup: 重新加载 Clash 配置成功")
		}
	} else {
		logger.Println("Setup: 用户未选择任何规则，跳过规则更新")
	}

	// 发送成功响应
	logger.Println("Setup: 发送成功响应...")
	common.SendSuccessResponse(w, "设置完成", nil)
	logger.Println("===== Setup 流程完成 =====")
}

// HandleLogsPage 处理日志页面请求
func (h *PageHandler) HandleLogsPage(w http.ResponseWriter, r *http.Request) {
	// 响应头
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 读取模板文件
	tmpl, err := common.LoadTemplate("logs.html")
	if err != nil {
		logger.Errorf("解析日志页面模板失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 准备模板数据
	data := map[string]interface{}{
		"Version": h.Version,
		"BuildID": time.Now().Format("060102"),
	}

	// 渲染模板
	err = tmpl.Execute(w, data)
	if err != nil {
		logger.Errorf("渲染日志页面模板失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}