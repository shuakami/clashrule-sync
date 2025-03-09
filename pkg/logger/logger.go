package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
)

var (
	// Log 全局日志实例
	Log *logrus.Logger

	// 日志轮换设置
	maxSize    = 10 // MB
	maxBackups = 5  // 文件个数
	maxAge     = 30 // 天
	compress   = true

	// 日志文件路径
	logFilePath string
)

// 获取配置目录路径
func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("获取用户主目录失败: %v，使用当前目录\n", err)
		return ".config/clashrule-sync"
	}
	return filepath.Join(homeDir, ".config", "clashrule-sync")
}

// 确保目录存在
func ensureDirExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

// 初始化日志系统
func init() {
	Log = logrus.New()
	
	// 设置日志格式
	Log.SetFormatter(&logrus.TextFormatter{
		ForceColors:               true,
		DisableColors:             false,
		ForceQuote:                false,
		DisableQuote:              false,
		EnvironmentOverrideColors: false,
		DisableTimestamp:          false,
		FullTimestamp:             true,
		TimestampFormat:           "2006-01-02 15:04:05",
		DisableSorting:            false,
		SortingFunc:               nil,
		DisableLevelTruncation:    false,
		PadLevelText:              true,
		QuoteEmptyFields:          false,
		FieldMap:                  nil,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			// 获取相对路径
			repopath := fmt.Sprintf("%s/src/github.com/shuakami/clashrule-sync/", os.Getenv("GOPATH"))
			filename := strings.Replace(f.File, repopath, "", -1)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})

	// 设置日志级别
	Log.SetLevel(logrus.InfoLevel)

	// 设置调用者信息
	Log.SetReportCaller(true)

	// 创建日志目录
	configDir := getConfigDir()
	logDir := filepath.Join(configDir, "logs")
	if err := ensureDirExists(logDir); err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
		return
	}

	// 设置日志文件路径
	logFilePath = filepath.Join(logDir, "clashrule-sync.log")

	// 设置日志输出
	Log.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    maxSize,    // MB
		MaxBackups: maxBackups, // 保留旧文件的最大个数
		MaxAge:     maxAge,     // 保留旧文件的最大天数
		Compress:   compress,   // 是否压缩
		LocalTime:  true,
	}))
}

// Debug 输出调试日志
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

// Debugf 输出格式化的调试日志
func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

// Info 输出信息日志
func Info(args ...interface{}) {
	Log.Info(args...)
}

// Infof 输出格式化的信息日志
func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

// Println 输出信息日志（同Info）
func Println(args ...interface{}) {
	Log.Info(args...)
}

// Printf 输出格式化的信息日志（同Infof）
func Printf(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

// Warn 输出警告日志
func Warn(args ...interface{}) {
	Log.Warn(args...)
}

// Warnf 输出格式化的警告日志
func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

// Error 输出错误日志
func Error(args ...interface{}) {
	Log.Error(args...)
}

// Errorf 输出格式化的错误日志
func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

// Fatal 输出致命错误日志并退出程序
func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

// Fatalf 输出格式化的致命错误日志并退出程序
func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

// Panic 输出Panic日志并抛出panic
func Panic(args ...interface{}) {
	Log.Panic(args...)
}

// Panicf 输出格式化的Panic日志并抛出panic
func Panicf(format string, args ...interface{}) {
	Log.Panicf(format, args...)
}

// CleanOldLogs 清理旧日志文件
func CleanOldLogs() {
	if logFilePath == "" {
		return
	}

	dir := filepath.Dir(logFilePath)
	files, err := os.ReadDir(dir)
	if err != nil {
		Error("读取日志目录失败:", err)
		return
	}

	// 获取日志文件名前缀
	base := filepath.Base(logFilePath)
	prefix := strings.TrimSuffix(base, filepath.Ext(base))

	// 过期时间
	cutoffTime := time.Now().AddDate(0, 0, -maxAge)

	// 循环检查日志文件
	for _, file := range files {
		// 跳过目录
		if file.IsDir() {
			continue
		}

		// 检查是否是备份的日志文件
		if !strings.HasPrefix(file.Name(), prefix) || (!strings.HasSuffix(file.Name(), ".log") && !strings.HasSuffix(file.Name(), ".gz")) {
			continue
		}

		// 获取文件信息
		fileInfo, err := file.Info()
		if err != nil {
			Warnf("获取文件信息失败: %s, %v", file.Name(), err)
			continue
		}

		// 检查文件修改时间
		if fileInfo.ModTime().Before(cutoffTime) {
			// 删除过期的日志文件
			fullPath := filepath.Join(dir, file.Name())
			if err := os.Remove(fullPath); err != nil {
				Warnf("删除过期日志文件失败: %s, %v", fullPath, err)
			} else {
				Infof("已删除过期日志文件: %s", fullPath)
			}
		}
	}
}

// GetLogDir 获取日志目录路径
func GetLogDir() string {
	if logFilePath == "" {
		return ""
	}
	return filepath.Dir(logFilePath)
}

// GetLogFilePath 获取当前日志文件路径
func GetLogFilePath() string {
	return logFilePath
}

// SetLogLevel 设置日志级别
func SetLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warn":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	case "fatal":
		Log.SetLevel(logrus.FatalLevel)
	case "panic":
		Log.SetLevel(logrus.PanicLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
} 