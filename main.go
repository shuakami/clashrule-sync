package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"github.com/shuakami/clashrule-sync/pkg/api"
	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/process"
	"github.com/shuakami/clashrule-sync/pkg/rules"
	"github.com/shuakami/clashrule-sync/pkg/web"
)

// 程序版本
const version = "0.1.11"

// 程序名称
const appName = "ClashRuleSync"

// 服务配置
type program struct {
	cfg            *config.Config
	processMonitor *process.ProcessMonitor
	ruleUpdater    *rules.RuleUpdater
	clashAPI       *api.ClashAPI
	webServer      *web.WebServer
	updateTicker   *time.Ticker
	logCleanTicker *time.Ticker // 日志清理定时器
	stopChan       chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	// 清理工作
	logger.Info("ClashRuleSync 服务停止")

	// 取消上下文
	p.cancel()

	// 停止更新定时器
	if p.updateTicker != nil {
		p.updateTicker.Stop()
	}

	// 停止日志清理定时器
	if p.logCleanTicker != nil {
		p.logCleanTicker.Stop()
	}

	// 停止进程监控
	if p.processMonitor != nil {
		p.processMonitor.Stop()
	}

	// 停止 Web 服务器
	if p.webServer != nil {
		p.webServer.Stop()
	}

	// 关闭停止通道
	close(p.stopChan)

	return nil
}

func (p *program) run() {
	// 主程序逻辑
	logger.Info("ClashRuleSync 服务启动")

	// 创建上下文
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// 加载配置
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("加载配置失败: %v", err)
	}
	p.cfg = cfg

	// 应用日志配置
	logConfig := &logger.LogConfig{
		LogLevel:   cfg.LogConfig.LogLevel,
		MaxSize:    cfg.LogConfig.MaxSize,
		MaxBackups: cfg.LogConfig.MaxBackups,
		MaxAge:     cfg.LogConfig.MaxAge,
		Compress:   cfg.LogConfig.Compress,
	}
	logger.ApplyConfig(logConfig)

	// 启动日志清理定时器
	p.startLogCleanTicker()

	// 自动检测Clash API配置
	apiURL, apiPort, apiSecret, err := api.DetectClashAPIConfig()
	if err != nil {
		logger.Warnf("自动检测Clash API配置失败，将使用默认配置: %v", err)
	} else {
		logger.Infof("自动检测到Clash API配置: %s", apiURL)
		// 更新配置
		cfg.ClashAPIURL = apiURL
		cfg.ClashAPIPort = apiPort
		cfg.ClashAPISecret = apiSecret

		// 保存配置
		err = cfg.SaveConfig()
		if err != nil {
			logger.Warnf("保存配置失败: %v", err)
		}
	}

	// 创建 Clash API 客户端
	p.clashAPI = api.NewClashAPI(cfg.ClashAPIURL, cfg.ClashAPISecret)

	// 创建规则更新器
	p.ruleUpdater = rules.NewRuleUpdater(cfg)

	// 创建 Web 服务器
	p.webServer = web.NewWebServer(cfg, p.ruleUpdater, p.clashAPI)

	// 创建停止通道
	p.stopChan = make(chan struct{})

	// 定义 Clash 启动和停止的回调函数
	onClashStart := func() {
		logger.Info("检测到 Clash 启动，激活服务...")

		// 启动 Web 服务器
		err := p.webServer.Start()
		if err != nil {
			logger.Errorf("启动 Web 服务器失败: %v", err)
		}

		// 启动规则更新定时器
		p.startUpdateTicker()

		// 首次启动时尝试更新规则
		go func() {
			// 等待一段时间，确保 Clash 完全启动
			time.Sleep(5 * time.Second)

			// 尝试更新规则
			success, err := p.ruleUpdater.UpdateAllRules()
			if err != nil {
				logger.Errorf("首次更新规则失败: %v", err)
			} else if success {
				logger.Info("首次更新规则成功")

				// 保存配置
				p.cfg.SaveConfig()

				// 尝试重新加载 Clash 配置
				err = p.clashAPI.ReloadConfig()
				if err != nil {
					logger.Errorf("重新加载 Clash 配置失败: %v", err)
				}

				// 安全重启 Clash
				logger.Info("正在重启 Clash 以应用新规则...")
				err = process.RestartClash()
				if err != nil {
					logger.Errorf("重启 Clash 失败: %v", err)
				} else {
					logger.Info("Clash 重启成功，新规则已生效")
				}
			}
		}()
	}

	onClashStop := func() {
		logger.Info("检测到 Clash 停止，暂停服务...")

		// 停止更新定时器
		if p.updateTicker != nil {
			p.updateTicker.Stop()
			p.updateTicker = nil
		}

		// 停止 Web 服务器
		if p.webServer != nil {
			p.webServer.Stop()
		}
	}

	// 创建进程监控器
	p.processMonitor = process.NewProcessMonitor(5*time.Second, onClashStart, onClashStop)
	p.processMonitor.Start()

	// 等待停止信号
	<-p.stopChan
}

// 启动规则更新定时器
func (p *program) startUpdateTicker() {
	// 如果已经有定时器在运行，先停止它
	if p.updateTicker != nil {
		p.updateTicker.Stop()
	}

	// 创建新的定时器
	p.updateTicker = time.NewTicker(p.cfg.UpdateInterval)

	// 在新的 goroutine 中处理定时更新
	go func() {
		for {
			select {
			case <-p.updateTicker.C:
				// 检查 Clash 是否在运行
				if !p.processMonitor.IsClashRunning() {
					continue
				}

				logger.Info("定时更新规则...")
				success, err := p.ruleUpdater.UpdateAllRules()
				if err != nil {
					logger.Errorf("定时更新规则失败: %v", err)
					continue
				}

				if success {
					logger.Info("定时更新规则成功")

					// 保存配置
					p.cfg.SaveConfig()

					// 尝试重新加载 Clash 配置
					err = p.clashAPI.ReloadConfig()
					if err != nil {
						logger.Errorf("重新加载 Clash 配置失败: %v", err)
					}

					// 安全重启 Clash
					logger.Info("正在重启 Clash 以应用新规则...")
					err = process.RestartClash()
					if err != nil {
						logger.Errorf("重启 Clash 失败: %v", err)
					} else {
						logger.Info("Clash 重启成功，新规则已生效")
					}
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// 启动日志清理定时器
func (p *program) startLogCleanTicker() {
	// 如果已经有定时器在运行，先停止它
	if p.logCleanTicker != nil {
		p.logCleanTicker.Stop()
	}

	// 创建新的定时器，每天执行一次日志清理
	p.logCleanTicker = time.NewTicker(24 * time.Hour)

	// 立即执行一次日志清理
	logger.CleanOldLogs()

	// 在新的 goroutine 中处理定时清理
	go func() {
		for {
			select {
			case <-p.logCleanTicker.C:
				logger.Debug("执行定时日志清理...")
				logger.CleanOldLogs()
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

func main() {
	// 解析命令行参数
	var (
		showVersion      = flag.Bool("v", false, "显示版本信息")
		runAsService     = flag.Bool("service", false, "作为服务运行")
		installService   = flag.Bool("install", false, "安装为系统服务")
		uninstallService = flag.Bool("uninstall", false, "卸载系统服务")
	)
	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("%s v%s\n", appName, version)
		return
	}

	// 注册日志
	// 注意：不再需要设置标准日志选项，因为我们使用自定义日志模块

	// 创建服务配置
	svcConfig := &service.Config{
		Name:        "ClashRuleSync",
		DisplayName: "Clash Rule Sync Service",
		Description: "Clash 规则自动同步服务",
	}

	// 创建程序实例
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Fatalf("无法创建服务: %v", err)
	}

	// 处理服务相关操作
	if *installService {
		err = s.Install()
		if err != nil {
			logger.Fatalf("安装服务失败: %v", err)
		}
		logger.Info("服务安装成功")
		return
	}

	if *uninstallService {
		err = s.Uninstall()
		if err != nil {
			logger.Fatalf("卸载服务失败: %v", err)
		}
		logger.Info("服务卸载成功")
		return
	}

	if *runAsService {
		err = s.Run()
		if err != nil {
			logger.Fatalf("运行服务失败: %v", err)
		}
		return
	}

	// 直接运行程序
	logger.Info("启动 ClashRuleSync...")

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动程序
	go func() {
		if err := s.Run(); err != nil {
			logger.Fatalf("运行程序失败: %v", err)
		}
	}()

	// 等待终止信号
	<-sigChan
	logger.Info("接收到终止信号，正在优雅关闭...")

	// 尝试停止服务
	s.Stop()
}
