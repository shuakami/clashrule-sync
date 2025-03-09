package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/rules"
	"github.com/shuakami/clashrule-sync/pkg/utils"
	"github.com/shuakami/clashrule-sync/pkg/web/common"
	"github.com/shuakami/clashrule-sync/pkg/web/handlers"
	"github.com/shuakami/clashrule-sync/pkg/web/middleware"
)

// Version 应用程序版本
const Version = "0.1.9"

// WebServer 表示 Web 服务器
type WebServer struct {
	config      *config.Config
	ruleUpdater *rules.RuleUpdater
	clashAPI    *api.ClashAPI
	server      *http.Server
	port        int
	router      *http.ServeMux

	// 处理器
	pageHandler   *handlers.PageHandler
	statusHandler *handlers.StatusHandler
	configHandler *handlers.ConfigHandler
	rulesHandler  *handlers.RulesHandler
	systemHandler *handlers.SystemHandler
	logHandler    *handlers.LogHandler
}

// NewWebServer 创建一个新的 Web 服务器
func NewWebServer(cfg *config.Config, ruleUpdater *rules.RuleUpdater, clashAPI *api.ClashAPI) *WebServer {
	ws := &WebServer{
		config:      cfg,
		ruleUpdater: ruleUpdater,
		clashAPI:    clashAPI,
		port:        cfg.WebPort,
	}

	// 创建各个处理器
	ws.systemHandler = handlers.NewSystemHandler(cfg)
	ws.pageHandler = handlers.NewPageHandler(cfg, ruleUpdater, clashAPI, Version)
	ws.statusHandler = handlers.NewStatusHandler(cfg, ruleUpdater, clashAPI)
	ws.rulesHandler = handlers.NewRulesHandler(cfg, ruleUpdater, clashAPI)
	ws.configHandler = handlers.NewConfigHandler(cfg, clashAPI, ws.systemHandler.HandleSystemAutoStart)
	ws.logHandler = handlers.NewLogHandler(cfg)

	return ws
}

// Start 启动 Web 服务器
func (ws *WebServer) Start() error {
	// 应用系统自启动设置
	err := ws.systemHandler.HandleSystemAutoStart()
	if err != nil {
		logger.Warnf("应用系统自启动设置失败: %v", err)
		// 继续执行，不影响主要功能
	}

	// 创建路由器
	ws.registerRoutes()

	// 找一个可用的端口
	port := ws.findAvailablePort()
	if port != ws.port {
		logger.Infof("端口 %d 被占用，使用端口 %d", ws.port, port)
		ws.port = port
	}

	// 创建服务器
	ws.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           ws.applyMiddleware(ws.router),
		ReadHeaderTimeout: 5 * time.Second,   // 避免慢客户端攻击
		WriteTimeout:      10 * time.Second,  // 避免挂起的连接
		IdleTimeout:       120 * time.Second, // 空闲连接超时
	}

	// 启动服务器
	logger.Infof("Web 服务器启动在 http://localhost:%d", port)

	// 首次运行时自动打开浏览器
	if ws.config.FirstRun {
		common.OpenBrowser(fmt.Sprintf("http://localhost:%d/setup", port))
		ws.config.FirstRun = false
		ws.config.SaveConfig()
	}

	// 在新 goroutine 中启动服务
	go func() {
		if err := ws.server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Errorf("HTTP 服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止 Web 服务器
func (ws *WebServer) Stop() error {
	if ws.server != nil {
		logger.Info("正在停止 Web 服务器...")
		return ws.server.Close()
	}
	return nil
}

// 应用中间件
func (ws *WebServer) applyMiddleware(handler http.Handler) http.Handler {
	// 应用顺序很重要，从内到外
	handler = middleware.CacheControlMiddleware(handler) // 缓存控制
	handler = middleware.RecoveryMiddleware(handler)     // 紧急恢复
	handler = middleware.LoggingMiddleware(handler)      // 日志记录
	return handler
}

// 注册HTTP路由
func (ws *WebServer) registerRoutes() {
	router := http.NewServeMux()

	// 静态文件
	router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// API 路由 - 状态
	router.HandleFunc("/api/status", ws.statusHandler.HandleStatus)
	router.HandleFunc("/api/update", ws.statusHandler.HandleUpdate)

	// API 路由 - 配置
	router.HandleFunc("/api/config", ws.configHandler.HandleConfig)
	router.HandleFunc("/api/toggle-autostart", ws.configHandler.HandleToggleAutoStart)
	router.HandleFunc("/api/toggle-system-autostart", ws.configHandler.HandleToggleSystemAutoStart)

	// API 路由 - 规则管理
	router.HandleFunc("/api/rules", ws.rulesHandler.HandleRules)
	router.HandleFunc("/api/rules/add", ws.rulesHandler.HandleAddRule)
	router.HandleFunc("/api/rules/edit", ws.rulesHandler.HandleEditRule)
	router.HandleFunc("/api/rules/delete", ws.rulesHandler.HandleDeleteRule)
	router.HandleFunc("/api/sync-bypass", ws.rulesHandler.HandleSyncBypass)

	// API 路由 - 日志管理
	router.HandleFunc("/api/logs", ws.logHandler.HandleGetLog)
	router.HandleFunc("/api/logs/config", ws.logHandler.HandleSetLogConfig)
	router.HandleFunc("/api/logs/clean", ws.logHandler.HandleCleanLogs)

	// API 路由 - 测试
	router.HandleFunc("/api/test-connection", ws.pageHandler.HandleTestConnection)

	// API 路由 - 设置向导
	router.HandleFunc("/api/setup", ws.pageHandler.HandleSetup)

	// 页面路由
	router.HandleFunc("/setup", ws.pageHandler.HandleSetupPage)
	router.HandleFunc("/logs", ws.pageHandler.HandleLogsPage)
	router.HandleFunc("/", ws.pageHandler.HandleIndex)

	ws.router = router
}

// 找一个可用的端口
func (ws *WebServer) findAvailablePort() int {
	port := ws.port

	// 如果当前端口不可用，尝试找一个可用的端口
	if !utils.IsPortAvailable(port) {
		// 从当前端口开始，尝试查找可用端口
		port = utils.FindAvailablePort(port)
	}

	return port
}
