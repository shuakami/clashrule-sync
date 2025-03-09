package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/rules"
	"github.com/shuakami/clashrule-sync/pkg/web/common"
)

// RulesHandler 处理规则相关的请求
type RulesHandler struct {
	Config      *config.Config
	RuleUpdater *rules.RuleUpdater
	ClashAPI    *api.ClashAPI
}

// NewRulesHandler 创建规则处理器
func NewRulesHandler(cfg *config.Config, ruleUpdater *rules.RuleUpdater, clashAPI *api.ClashAPI) *RulesHandler {
	return &RulesHandler{
		Config:      cfg,
		RuleUpdater: ruleUpdater,
		ClashAPI:    clashAPI,
	}
}

// HandleRules 处理获取规则列表请求
func (h *RulesHandler) HandleRules(w http.ResponseWriter, r *http.Request) {
	if !common.RequireGetMethod(w, r) {
		return
	}

	// 获取规则提供者列表
	ruleProviders := h.Config.RuleProviders

	// 准备响应
	resp := map[string]interface{}{
		"status":         "ok",
		"rule_providers": ruleProviders,
	}

	// 发送 JSON 响应
	common.SendJSONResponse(w, resp)
}

// HandleAddRule 处理添加规则请求
func (h *RulesHandler) HandleAddRule(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 解析请求
	var req struct {
		Rule config.RuleProvider `json:"rule"`
	}

	if !common.ParseJSON(w, r, &req) {
		return
	}

	// 验证规则数据
	if req.Rule.Name == "" || req.Rule.URL == "" || req.Rule.Type == "" || req.Rule.Behavior == "" || req.Rule.Path == "" {
		common.SendBadRequest(w, "规则数据不完整", nil)
		return
	}

	// 检查名称是否已存在
	for _, rule := range h.Config.RuleProviders {
		if rule.Name == req.Rule.Name {
			common.SendBadRequest(w, "规则名称已存在", nil)
			return
		}
	}

	// 添加规则
	h.Config.RuleProviders = append(h.Config.RuleProviders, req.Rule)

	// 保存配置
	if err := h.Config.SaveConfig(); err != nil {
		common.SendInternalError(w, "保存配置失败", err)
		return
	}

	// 更新规则
	if req.Rule.Enabled {
		success, err := h.RuleUpdater.UpdateRuleProvider(req.Rule.Name)
		if err != nil {
			logger.Errorf("更新规则失败: %v", err)
		} else if success {
			logger.Infof("规则 %s 更新成功", req.Rule.Name)
		}

		// 尝试重新加载 Clash 配置
		err = h.ClashAPI.ReloadConfig()
		if err != nil {
			logger.Errorf("重新加载 Clash 配置失败: %v", err)
		}
	}

	// 发送成功响应
	common.SendSuccessResponse(w, "添加规则成功", nil)
}

// HandleEditRule 处理编辑规则请求
func (h *RulesHandler) HandleEditRule(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 解析请求
	var req struct {
		Index int                  `json:"index"`
		Rule  config.RuleProvider `json:"rule"`
	}

	if !common.ParseJSON(w, r, &req) {
		return
	}

	// 验证规则数据
	if req.Rule.Name == "" || req.Rule.URL == "" || req.Rule.Type == "" || req.Rule.Behavior == "" || req.Rule.Path == "" {
		common.SendBadRequest(w, "规则数据不完整", nil)
		return
	}

	// 检查索引是否有效
	if req.Index < 0 || req.Index >= len(h.Config.RuleProviders) {
		common.SendBadRequest(w, "无效的规则索引", nil)
		return
	}

	// 检查名称是否重复（除了自身）
	for i, rule := range h.Config.RuleProviders {
		if i != req.Index && rule.Name == req.Rule.Name {
			common.SendBadRequest(w, "规则名称已存在", nil)
			return
		}
	}

	// 更新规则
	h.Config.RuleProviders[req.Index] = req.Rule

	// 保存配置
	if err := h.Config.SaveConfig(); err != nil {
		common.SendInternalError(w, "保存配置失败", err)
		return
	}

	// 更新规则
	if req.Rule.Enabled {
		success, err := h.RuleUpdater.UpdateRuleProvider(req.Rule.Name)
		if err != nil {
			logger.Errorf("更新规则失败: %v", err)
		} else if success {
			logger.Infof("规则 %s 更新成功", req.Rule.Name)
		}

		// 尝试重新加载 Clash 配置
		err = h.ClashAPI.ReloadConfig()
		if err != nil {
			logger.Errorf("重新加载 Clash 配置失败: %v", err)
		}
	}

	// 发送成功响应
	common.SendSuccessResponse(w, "编辑规则成功", nil)
}

// HandleDeleteRule 处理删除规则请求
func (h *RulesHandler) HandleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 解析请求
	var req struct {
		Name string `json:"name"`
	}

	if !common.ParseJSON(w, r, &req) {
		return
	}

	// 验证请求
	if req.Name == "" {
		common.SendBadRequest(w, "规则名称不能为空", nil)
		return
	}

	// 查找并删除规则
	found := false
	for i, rule := range h.Config.RuleProviders {
		if rule.Name == req.Name {
			// 删除规则
			h.Config.RuleProviders = append(h.Config.RuleProviders[:i], h.Config.RuleProviders[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		common.SendBadRequest(w, "规则不存在", nil)
		return
	}

	// 保存配置
	if err := h.Config.SaveConfig(); err != nil {
		common.SendInternalError(w, "保存配置失败", err)
		return
	}

	// 尝试重新加载 Clash 配置
	err := h.ClashAPI.ReloadConfig()
	if err != nil {
		logger.Errorf("重新加载 Clash 配置失败: %v", err)
	}

	// 发送成功响应
	common.SendSuccessResponse(w, "删除规则成功", nil)
}

// HandleSyncBypass 处理绕过规则同步请求
func (h *RulesHandler) HandleSyncBypass(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 解析请求体
	var req struct {
		BypassRules string   `json:"bypass_rules"`
		RuleNames   []string `json:"rule_names"`
	}
	
	if !common.ParseJSON(w, r, &req) {
		return
	}

	var success bool
	var message string

	// 如果提供了直接的绕过规则
	if req.BypassRules != "" {
		// 直接同步绕过规则
		err := api.UpdateBypassRules("", req.BypassRules)
		if err != nil {
			common.SendInternalError(w, "更新绕过规则失败", err)
			return
		}
		
		message = "成功更新绕过规则"
		success = true
	} else if len(req.RuleNames) > 0 {
		// 根据规则名称同步
		var rulesToSync []string
		
		// 收集规则内容
		for _, ruleName := range req.RuleNames {
			for _, provider := range h.Config.RuleProviders {
				if provider.Name == ruleName && provider.Enabled {
					// 找到规则文件
					rulesDir := common.GetRulesDir()
					ruleFilePath := filepath.Join(rulesDir, provider.Path)
					
					// 读取规则内容
					content, err := os.ReadFile(ruleFilePath)
					if err != nil {
						logger.Errorf("读取规则 %s 失败: %v", ruleName, err)
						continue
					}
					
					rulesToSync = append(rulesToSync, string(content))
				}
			}
		}
		
		// 如果找到了规则，进行同步
		if len(rulesToSync) > 0 {
			combinedRules := strings.Join(rulesToSync, "\n")
			err := api.SyncBypassRulesFromDomainList(combinedRules)
			if err != nil {
				common.SendInternalError(w, "同步规则到绕过配置失败", err)
				return
			}
			
			message = "成功同步规则到绕过配置"
			success = true
		} else {
			common.SendBadRequest(w, "未找到指定的规则", nil)
			return
		}
	} else {
		// 没有提供任何规则
		common.SendBadRequest(w, "未提供任何规则", nil)
		return
	}

	// 发送成功响应
	resp := map[string]interface{}{
		"status":  "ok",
		"message": message,
		"success": success,
	}
	
	common.SendJSONResponse(w, resp)
} 