package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shuakami/clashrule-sync/pkg/config"
	"github.com/shuakami/clashrule-sync/pkg/logger"
	"github.com/shuakami/clashrule-sync/pkg/web/common"
)

// LogHandler 处理日志相关请求
type LogHandler struct {
	Config *config.Config
}

// NewLogHandler 创建日志处理器
func NewLogHandler(cfg *config.Config) *LogHandler {
	return &LogHandler{
		Config: cfg,
	}
}

// LogContent 表示日志内容结构
type LogContent struct {
	Content   string   `json:"content"`
	FilePath  string   `json:"file_path"`
	LogFiles  []string `json:"log_files"`
	LogConfig struct {
		LogLevel   string `json:"log_level"`
		MaxSize    int    `json:"max_size"`
		MaxBackups int    `json:"max_backups"`
		MaxAge     int    `json:"max_age"`
		Compress   bool   `json:"compress"`
	} `json:"log_config"`
}

// LogConfigRequest 表示更新日志配置的请求
type LogConfigRequest struct {
	LogLevel   string `json:"log_level"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
}

// HandleGetLog 获取日志内容
func (h *LogHandler) HandleGetLog(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	query := r.URL.Query()
	fileName := query.Get("file")
	lines, _ := strconv.Atoi(query.Get("lines"))
	if lines <= 0 {
		lines = 500 // 默认显示500行
	}

	// 读取当前日志文件路径
	logFilePath := logger.GetLogFilePath()
	if fileName != "" {
		// 仅允许访问日志目录下的日志文件
		logDir := filepath.Dir(logFilePath)
		logFilePath = filepath.Join(logDir, fileName)
	}

	// 检查文件是否存在
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		common.SendErrorResponse(w, http.StatusNotFound, "日志文件不存在", nil)
		return
	}

	// 读取日志文件
	content, err := readLastLines(logFilePath, lines)
	if err != nil {
		common.SendErrorResponse(w, http.StatusInternalServerError, "读取日志失败", err)
		return
	}

	// 获取所有日志文件
	logDir := filepath.Dir(logFilePath)
	files, err := listLogFiles(logDir)
	if err != nil {
		logger.Warnf("读取日志目录失败: %v", err)
		files = []string{}
	}

	// 构造响应
	response := LogContent{
		Content:  content,
		FilePath: logFilePath,
		LogFiles: files,
	}

	// 添加日志配置信息
	response.LogConfig.LogLevel = h.Config.LogConfig.LogLevel
	response.LogConfig.MaxSize = h.Config.LogConfig.MaxSize
	response.LogConfig.MaxBackups = h.Config.LogConfig.MaxBackups
	response.LogConfig.MaxAge = h.Config.LogConfig.MaxAge
	response.LogConfig.Compress = h.Config.LogConfig.Compress

	common.SendJSONResponse(w, response)
}

// HandleSetLogConfig 更新日志配置
func (h *LogHandler) HandleSetLogConfig(w http.ResponseWriter, r *http.Request) {
	// 解析请求
	var req LogConfigRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		common.SendErrorResponse(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 更新配置
	h.Config.LogConfig.LogLevel = req.LogLevel
	h.Config.LogConfig.MaxSize = req.MaxSize
	h.Config.LogConfig.MaxBackups = req.MaxBackups
	h.Config.LogConfig.MaxAge = req.MaxAge
	h.Config.LogConfig.Compress = req.Compress

	// 保存配置
	err = h.Config.SaveConfig()
	if err != nil {
		common.SendErrorResponse(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}

	// 应用日志配置
	logConfig := &logger.LogConfig{
		LogLevel:   req.LogLevel,
		MaxSize:    req.MaxSize,
		MaxBackups: req.MaxBackups,
		MaxAge:     req.MaxAge,
		Compress:   req.Compress,
	}
	logger.ApplyConfig(logConfig)

	common.SendSuccessResponse(w, "日志配置已更新", nil)
}

// HandleCleanLogs 清理过期日志
func (h *LogHandler) HandleCleanLogs(w http.ResponseWriter, r *http.Request) {
	// 执行日志清理
	logger.CleanOldLogs()
	common.SendSuccessResponse(w, "日志清理成功", nil)
}

// 读取文件最后N行
func readLastLines(filePath string, n int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	// 如果文件为空，直接返回
	if stat.Size() == 0 {
		return "", nil
	}

	// 创建一个足够大的缓冲区
	bufferSize := int64(1024 * 1024) // 1MB
	if stat.Size() < bufferSize {
		bufferSize = stat.Size()
	}

	// 从文件末尾开始读取
	offset := stat.Size() - bufferSize
	if offset < 0 {
		offset = 0
		bufferSize = stat.Size()
	}

	// 读取数据
	buffer := make([]byte, bufferSize)
	_, err = file.ReadAt(buffer, offset)
	if err != nil && err != io.EOF {
		return "", err
	}

	// 将字节转换为字符串
	content := string(buffer)

	// 按行分割
	lines := strings.Split(content, "\n")

	// 如果不是从文件开始读取，且第一行不完整，则删除
	if offset > 0 && len(lines) > 0 {
		lines = lines[1:]
	}

	// 如果行数超过要求，只保留最后n行
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// 重新组合为字符串
	return strings.Join(lines, "\n"), nil
}

// 列出日志目录中的所有日志文件
func listLogFiles(logDir string) ([]string, error) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		// 只添加.log和.gz后缀的文件
		name := file.Name()
		ext := filepath.Ext(name)
		if ext == ".log" || ext == ".gz" {
			result = append(result, name)
		}
	}

	return result, nil
} 