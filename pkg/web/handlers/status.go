package handlers

import (
	"net/http"
	"time"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/process"
	"github.com/shuakami/clashrule-sync/pkg/rules"
	"github.com/shuakami/clashrule-sync/pkg/web/common"
)

// StatusResponse 表示状态响应
type StatusResponse struct {
	Status              string            `json:"status"`
	StatusMessage       string            `json:"status_message"`
	ClashRunning        bool              `json:"clash_running"`
	ProcessDetected     bool              `json:"process_detected"`
	APIConnected        bool              `json:"api_connected"`
	LastUpdateTime      time.Time         `json:"last_update_time"`
	NextUpdateTime      time.Time         `json:"next_update_time"`
	UpdateHistory       []rules.UpdateRecord `json:"update_history"`
	AutoStartEnabled    bool              `json:"auto_start_enabled"`
	SystemAutoStartEnabled bool           `json:"system_auto_start_enabled"`
}

// 处理状态相关的函数需要访问WebServer的字段
type StatusHandler struct {
	Config      *config.Config
	RuleUpdater *rules.RuleUpdater
	ClashAPI    *api.ClashAPI
}

// NewStatusHandler 创建状态处理器
func NewStatusHandler(cfg *config.Config, ruleUpdater *rules.RuleUpdater, clashAPI *api.ClashAPI) *StatusHandler {
	return &StatusHandler{
		Config:      cfg,
		RuleUpdater: ruleUpdater,
		ClashAPI:    clashAPI,
	}
}

// HandleStatus 处理状态请求
func (h *StatusHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	// 通过两种方式检查Clash状态
	processRunning := false
	apiRunning := false
	
	// 检查进程是否在运行
	running, err := process.CheckClashRunning()
	if err != nil {
		processRunning = false
	} else if running {
		processRunning = true
	}
	
	// 检查API是否可连接
	_, err = h.ClashAPI.TestConnection()
	if err == nil {
		apiRunning = true
	}
	
	// 状态信息和标志
	status := "error"
	clashRunning := false
	statusMessage := ""
	
	// 根据检测结果设置状态
	if apiRunning {
		// API连接成功，状态为已连接
		status = "connected"
		clashRunning = true
		statusMessage = "Clash正在运行，API连接正常。"
	} else if processRunning {
		// 只有进程检测成功，API连接失败
		status = "process_only"
		clashRunning = false
		statusMessage = "检测到Clash正在运行，但无法连接到Clash API。请确保Clash API已启用。"
	} else {
		// 两种检测都失败
		status = "disconnected"
		clashRunning = false
		statusMessage = "未检测到Clash运行，请先启动Clash。"
	}
	
	// 计算下一次更新时间
	nextUpdateTime := h.Config.LastUpdateTime.Add(h.Config.UpdateInterval)

	// 准备响应
	resp := StatusResponse{
		Status:               status,
		StatusMessage:        statusMessage,
		ClashRunning:         clashRunning,
		ProcessDetected:      processRunning,
		APIConnected:         apiRunning,
		LastUpdateTime:       h.Config.LastUpdateTime,
		NextUpdateTime:       nextUpdateTime,
		UpdateHistory:        h.RuleUpdater.GetUpdateHistory(),
		AutoStartEnabled:     h.Config.AutoStartEnabled,
		SystemAutoStartEnabled: h.Config.SystemAutoStartEnabled,
	}

	// 发送 JSON 响应
	common.SendJSONResponse(w, resp)
}

// HandleUpdate 处理更新请求
func (h *StatusHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 执行规则更新
	success, err := h.RuleUpdater.UpdateAllRules()
	if err != nil {
		common.SendInternalError(w, "更新规则失败", err)
		return
	}

	// 尝试重新加载 Clash 配置
	err = h.ClashAPI.ReloadConfig()
	if err != nil {
		common.SendInternalError(w, "重新加载 Clash 配置失败", err)
		return
	}

	// 发送成功响应
	resp := map[string]interface{}{
		"status":  "ok",
		"success": success,
		"message": "规则更新成功",
	}

	common.SendJSONResponse(w, resp)
} 