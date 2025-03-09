package logger

// LogConfig 日志配置
type LogConfig struct {
	// LogLevel 日志级别：debug, info, warn, error, fatal, panic
	LogLevel string `json:"log_level"`
	
	// MaxSize 单个日志文件的最大大小，单位MB
	MaxSize int `json:"max_size"`
	
	// MaxBackups 保留的旧日志文件的最大数量
	MaxBackups int `json:"max_backups"`
	
	// MaxAge 保留的旧日志文件的最大天数
	MaxAge int `json:"max_age"`
	
	// Compress 是否压缩旧日志文件
	Compress bool `json:"compress"`
}

// NewDefaultLogConfig 创建默认日志配置
func NewDefaultLogConfig() *LogConfig {
	return &LogConfig{
		LogLevel:   "info",
		MaxSize:    10, // MB
		MaxBackups: 5,  // 文件个数
		MaxAge:     30, // 天
		Compress:   true,
	}
}

// ApplyConfig 应用日志配置
func ApplyConfig(config *LogConfig) {
	if config == nil {
		return
	}
	
	// 更新全局变量
	maxSize = config.MaxSize
	maxBackups = config.MaxBackups
	maxAge = config.MaxAge
	compress = config.Compress
	
	// 设置日志级别
	SetLogLevel(config.LogLevel)
	
	// 记录配置应用信息
	Infof("应用日志配置: 级别=%s, 大小=%dMB, 备份数=%d, 保留时间=%d天, 压缩=%v",
		config.LogLevel, config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress)
} 