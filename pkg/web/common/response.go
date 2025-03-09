package common

import (
	"encoding/json"
	"net/http"

	"github.com/shuakami/clashrule-sync/pkg/logger"
)

// 统一的成功响应格式
type SuccessResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// 统一的错误响应格式
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// SendJSONResponse 发送JSON响应
func SendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("编码JSON响应失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// SendSuccessResponse 发送成功的JSON响应
func SendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	resp := SuccessResponse{
		Status:  "ok",
		Message: message,
		Data:    data,
	}
	SendJSONResponse(w, resp)
}

// SendErrorResponse 发送错误的JSON响应
func SendErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	resp := ErrorResponse{
		Status:  "error",
		Message: message,
	}
	
	if err != nil {
		resp.Error = err.Error()
		logger.Errorf("错误: %s - %v", message, err)
	} else {
		logger.Infof("错误: %s", message)
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		logger.Errorf("编码错误响应失败: %v", encodeErr)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// SendMethodNotAllowed 发送方法不允许的响应
func SendMethodNotAllowed(w http.ResponseWriter) {
	SendErrorResponse(w, http.StatusMethodNotAllowed, "方法不允许", nil)
}

// SendBadRequest 发送无效请求的响应
func SendBadRequest(w http.ResponseWriter, message string, err error) {
	SendErrorResponse(w, http.StatusBadRequest, message, err)
}

// SendInternalError 发送服务器内部错误的响应
func SendInternalError(w http.ResponseWriter, message string, err error) {
	SendErrorResponse(w, http.StatusInternalServerError, message, err)
}

// RequirePostMethod 确保请求方法为POST，否则发送错误响应
func RequirePostMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		SendMethodNotAllowed(w)
		return false
	}
	return true
}

// RequireGetMethod 确保请求方法为GET，否则发送错误响应
func RequireGetMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		SendMethodNotAllowed(w)
		return false
	}
	return true
}

// ParseJSON 解析请求体中的JSON数据
func ParseJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		SendBadRequest(w, "无效的请求体", err)
		return false
	}
	return true
} 