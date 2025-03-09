package handlers

import (
	"net/http"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/web/common"
)

// ConfigHandler 处理配置相关的请求
type ConfigHandler struct {
	Config   *config.Config
	ClashAPI *api.ClashAPI
	// 系统自启动处理函数
	HandleSystemAutoStart func() error
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(cfg *config.Config, clashAPI *api.ClashAPI, systemAutoStartHandler func() error) *ConfigHandler {
	return &ConfigHandler{
		Config:                cfg,
		ClashAPI:              clashAPI,
		HandleSystemAutoStart: systemAutoStartHandler,
	}
}

// HandleConfig 处理配置请求
func (h *ConfigHandler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 返回当前配置
		common.SendJSONResponse(w, h.Config)
	} else if r.Method == http.MethodPost {
		// 解析新配置
		var updatedConfig config.Config
		if !common.ParseJSON(w, r, &updatedConfig) {
			return
		}

		// 更新部分可以更改的配置
		h.Config.ClashAPIURL = updatedConfig.ClashAPIURL
		h.Config.ClashAPISecret = updatedConfig.ClashAPISecret
		h.Config.ClashConfigPath = updatedConfig.ClashConfigPath
		h.Config.UpdateInterval = updatedConfig.UpdateInterval
		h.Config.AutoStartEnabled = updatedConfig.AutoStartEnabled
		h.Config.SystemAutoStartEnabled = updatedConfig.SystemAutoStartEnabled

		// 保存配置
		err := h.Config.SaveConfig()
		if err != nil {
			common.SendInternalError(w, "保存配置失败", err)
			return
		}

		// 处理系统自启动
		if h.HandleSystemAutoStart != nil {
			err = h.HandleSystemAutoStart()
			if err != nil {
				// 不中断响应，只记录日志
			}
		}

		// 发送成功响应
		common.SendSuccessResponse(w, "配置更新成功", nil)
	} else {
		common.SendMethodNotAllowed(w)
	}
}

// HandleToggleAutoStart 处理修改自动启动设置请求
func (h *ConfigHandler) HandleToggleAutoStart(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 读取请求体
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if !common.ParseJSON(w, r, &req) {
		return
	}

	// 更新配置
	h.Config.AutoStartEnabled = req.Enabled

	// 保存配置
	if err := h.Config.SaveConfig(); err != nil {
		common.SendInternalError(w, "保存配置失败", err)
		return
	}

	// 发送成功响应
	common.SendSuccessResponse(w, "自动启动设置已更新", nil)
}

// HandleToggleSystemAutoStart 处理修改系统自启动设置请求
func (h *ConfigHandler) HandleToggleSystemAutoStart(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePostMethod(w, r) {
		return
	}

	// 读取请求体
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if !common.ParseJSON(w, r, &req) {
		return
	}

	// 更新配置
	h.Config.SystemAutoStartEnabled = req.Enabled

	// 保存配置
	if err := h.Config.SaveConfig(); err != nil {
		common.SendInternalError(w, "保存配置失败", err)
		return
	}

	// 设置或移除系统自启动
	if h.HandleSystemAutoStart != nil {
		err := h.HandleSystemAutoStart()
		if err != nil {
			common.SendInternalError(w, "设置系统自启动失败", err)
			return
		}
	}

	// 发送成功响应
	common.SendSuccessResponse(w, "系统自启动设置已更新", nil)
}
