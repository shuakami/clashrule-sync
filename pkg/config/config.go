package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shuakami/clashrule-sync/pkg/utils"
)

// Config 存储程序配置
type Config struct {
	// 基本配置
	ClashAPIURL          string        `json:"clash_api_url"`
	ClashAPIPort         int           `json:"clash_api_port"`
	ClashAPISecret       string        `json:"clash_api_secret"`
	ClashConfigPath      string        `json:"clash_config_path"`
	WebPort              int           `json:"web_port"`
	UpdateInterval       time.Duration `json:"update_interval"`
	FirstRun             bool          `json:"first_run"`
	LastUpdateTime       time.Time     `json:"last_update_time"`
	AutoStartEnabled     bool          `json:"auto_start_enabled"`
	SystemAutoStartEnabled bool        `json:"system_auto_start_enabled"`

	// 日志配置
	LogConfig struct {
		LogLevel   string `json:"log_level"`   // 日志级别：debug, info, warn, error, fatal, panic
		MaxSize    int    `json:"max_size"`    // 单个日志文件的最大大小，单位MB
		MaxBackups int    `json:"max_backups"` // 保留的旧日志文件的最大数量
		MaxAge     int    `json:"max_age"`     // 保留的旧日志文件的最大天数
		Compress   bool   `json:"compress"`    // 是否压缩旧日志文件
	} `json:"log_config"`

	// 规则源配置
	RuleProviders []RuleProvider `json:"rule_providers"`
	
	// 配置文件路径缓存
	configPath string
	// 互斥锁，防止并发写入
	mutex sync.RWMutex
}

// RuleProvider 定义规则提供者
type RuleProvider struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Type     string `json:"type"` // domain, ipcidr 等
	Behavior string `json:"behavior"`
	Path     string `json:"path"`
	Enabled  bool   `json:"enabled"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	cfg := &Config{
		ClashAPIURL:          "http://127.0.0.1:9090",
		ClashAPIPort:         9090,
		ClashAPISecret:       "",
		ClashConfigPath:      "",
		WebPort:              8899,
		UpdateInterval:       12 * time.Hour,
		FirstRun:             true,
		LastUpdateTime:       time.Time{},
		AutoStartEnabled:     true,
		SystemAutoStartEnabled: true,
		RuleProviders: []RuleProvider{},
	}
	
	// 设置默认日志配置
	cfg.LogConfig.LogLevel = "info"
	cfg.LogConfig.MaxSize = 10    // MB
	cfg.LogConfig.MaxBackups = 5  // 文件个数
	cfg.LogConfig.MaxAge = 30     // 天
	cfg.LogConfig.Compress = true
	
	return cfg
}

// LoadConfig 从文件加载配置
func LoadConfig(filepath string) (*Config, error) {
	// 如果文件不存在，创建默认配置
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Println("配置文件不存在，创建默认配置")
		config := DefaultConfig()
		config.configPath = filepath
		err = config.SaveConfig()
		if err != nil {
			return nil, fmt.Errorf("保存默认配置失败: %v", err)
		}
		return config, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析配置
	config := &Config{}
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}
	
	// 保存配置文件路径
	config.configPath = filepath

	// 检查并修正 UpdateInterval
	if config.UpdateInterval <= 0 {
		log.Println("检测到无效的更新间隔，使用默认值（12小时）")
		config.UpdateInterval = 12 * time.Hour
	}

	return config, nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	filePath := c.configPath
	if filePath == "" {
		filePath = GetConfigPath()
		c.configPath = filePath
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := utils.EnsureDirExists(dir); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 序列化配置
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	// 写入文件
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// UpdateLastUpdateTime 更新最后一次规则更新时间
func (c *Config) UpdateLastUpdateTime() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.LastUpdateTime = time.Now()
}

// GetConfigPath 返回配置文件路径
func GetConfigPath() string {
	configDir := utils.GetConfigDir()
	return filepath.Join(configDir, "config.json")
}

// GetRuleProvider 通过名称获取规则提供者
func (c *Config) GetRuleProvider(name string) *RuleProvider {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	for i := range c.RuleProviders {
		if c.RuleProviders[i].Name == name {
			return &c.RuleProviders[i]
		}
	}
	return nil
} 