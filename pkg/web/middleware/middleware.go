package middleware

import (
	"net/http"
	"time"

	"github.com/shuakami/clashrule-sync/pkg/logger"
)

// LoggingMiddleware 记录每个HTTP请求的日志
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 调用下一个处理器
		next.ServeHTTP(w, r)

		// 记录请求完成情况
		logger.Debugf(
			"请求 %s %s 已完成，耗时 %v",
			r.Method,
			r.URL.Path,
			time.Since(start),
		)
	})
}

// RecoveryMiddleware 从panic中恢复并返回500错误
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("处理请求时发生panic: %v", err)
				http.Error(w, "内部服务器错误", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// CacheControlMiddleware 添加缓存控制头
func CacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 静态文件可以缓存，但API请求不缓存
		if r.URL.Path == "/" || r.URL.Path == "/setup" || r.URL.Path == "/logs" || r.URL.Path == "/index.html" || r.URL.Path == "/setup.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if isStaticFile(r.URL.Path) {
			// 静态资源可以缓存一小时
			w.Header().Set("Cache-Control", "public, max-age=3600")
		} else if isAPIPath(r.URL.Path) {
			// API不缓存
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		next.ServeHTTP(w, r)
	})
}

// isStaticFile 判断是否是静态文件
func isStaticFile(path string) bool {
	staticPrefixes := []string{
		"/static/",
		"/assets/",
		"/css/",
		"/js/",
		"/img/",
		"/favicon.ico",
	}

	for _, prefix := range staticPrefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// isAPIPath 判断是否是API路径
func isAPIPath(path string) bool {
	return len(path) >= 4 && path[:4] == "/api"
} 