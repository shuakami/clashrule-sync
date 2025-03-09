package process

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"syscall"

	"github.com/shirou/gopsutil/process"
	"github.com/shuakami/clashrule-sync/pkg/utils"
)

// 默认API配置
const DefaultAPIURL = "http://127.0.0.1:9090"

// ClashProcessNames 定义了Clash可能的进程名称
var ClashProcessNames = []string{
	"clash", "clash.exe",
	"clash-windows", "clash-windows.exe",
	"clash-win64", "clash-win64.exe",
	"Clash for Windows", "Clash for Windows.exe",
	"Clash.Meta", "Clash.Meta.exe",
	"clash-verge", "clash-verge.exe",
}

// ProcessMonitor 用于监控Clash进程状态
type ProcessMonitor struct {
	isRunning     bool
	onClashStart  func()
	onClashStop   func()
	checkInterval time.Duration
	stopChan      chan struct{}
	mutex         sync.RWMutex
	// HTTP客户端用于API连接测试
	client *http.Client
}

// NewProcessMonitor 创建一个新的进程监控器
func NewProcessMonitor(checkInterval time.Duration, onStart func(), onStop func()) *ProcessMonitor {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return &ProcessMonitor{
		checkInterval: checkInterval,
		onClashStart:  onStart,
		onClashStop:   onStop,
		stopChan:      make(chan struct{}),
		client:        client,
	}
}

// Start 开始监控Clash进程
func (pm *ProcessMonitor) Start() {
	log.Println("开始监控Clash进程...")
	go func() {
		ticker := time.NewTicker(pm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.mutex.RLock()
				wasRunning := pm.isRunning
				pm.mutex.RUnlock()

				isRunning := pm.checkClashProcess()

				if isRunning && !wasRunning {
					log.Println("检测到Clash已启动")
					pm.setRunningState(true)
					if pm.onClashStart != nil {
						pm.onClashStart()
					}
				} else if !isRunning && wasRunning {
					log.Println("检测到Clash已停止")
					pm.setRunningState(false)
					if pm.onClashStop != nil {
						pm.onClashStop()
					}
				}
			case <-pm.stopChan:
				log.Println("停止监控Clash进程")
				return
			}
		}
	}()
}

// Stop 停止监控
func (pm *ProcessMonitor) Stop() {
	select {
	case <-pm.stopChan:
		// 通道已关闭，不做操作
		return
	default:
		close(pm.stopChan)
	}
}

// IsClashRunning 返回Clash是否正在运行
func (pm *ProcessMonitor) IsClashRunning() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.isRunning
}

// setRunningState 设置运行状态（线程安全）
func (pm *ProcessMonitor) setRunningState(running bool) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.isRunning = running
}

// checkClashProcess 检查Clash进程是否在运行
func (pm *ProcessMonitor) checkClashProcess() bool {
	processes, err := process.Processes()
	if err != nil {
		log.Printf("获取进程列表出错: %v", err)
		return false
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		for _, clashName := range ClashProcessNames {
			if name == clashName {
				// 检测到Clash进程
				return true
			}
		}
	}

	// 如果没有找到进程，尝试通过API连接测试
	return CheckAPIConnection(DefaultAPIURL)
}

// CheckAPIConnection 检查是否可以连接到Clash API
func CheckAPIConnection(apiURL string, secret ...string) bool {
	testURL := apiURL + "/version"

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return false
	}

	// 如果提供了密钥，添加到请求头
	if len(secret) > 0 && secret[0] != "" {
		req.Header.Set("Authorization", "Bearer "+secret[0])
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("连接Clash API失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("Clash API返回非200状态码: %d", resp.StatusCode)
		return false
	}

	// 读取响应内容
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应失败: %v", err)
		return false
	}

	log.Println("成功连接到Clash API")
	return true
}

// CheckClashRunning 检查Clash进程是否在运行，返回布尔值和可能的错误
// 用于一次性检查，不参与监控流程
func CheckClashRunning() (bool, error) {
	processes, err := process.Processes()
	if err != nil {
		return false, fmt.Errorf("获取进程列表失败: %v", err)
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		for _, clashName := range ClashProcessNames {
			if name == clashName {
				log.Printf("检测到Clash进程: %s (PID: %d)", name, p.Pid)
				return true, nil
			}
		}
	}

	return false, nil
}

// 添加全局变量来缓存成功的Clash启动路径
var (
	lastSuccessfulClashPath string
	lastSuccessfulClashArgs []string
	clashPathMutex          sync.Mutex
)

// 保存最后一次成功的启动路径
func saveSuccessfulPath(path string, args []string) {
	clashPathMutex.Lock()
	defer clashPathMutex.Unlock()

	lastSuccessfulClashPath = path
	lastSuccessfulClashArgs = make([]string, len(args))
	copy(lastSuccessfulClashArgs, args)

	// 将信息写入临时文件，以便下次启动时恢复
	savePathInfoToFile(path, args)
}

// 从文件读取上次成功的路径
func loadSuccessfulPathFromFile() (string, []string) {
	configDir := utils.GetConfigDir()
	pathFile := filepath.Join(configDir, "clash_path.json")

	if !utils.FileExists(pathFile) {
		return "", nil
	}

	data, err := os.ReadFile(pathFile)
	if err != nil {
		return "", nil
	}

	var info struct {
		Path string   `json:"path"`
		Args []string `json:"args"`
	}

	err = json.Unmarshal(data, &info)
	if err != nil {
		return "", nil
	}

	return info.Path, info.Args
}

// 保存路径信息到文件
func savePathInfoToFile(path string, args []string) {
	if path == "" {
		return
	}

	configDir := utils.GetConfigDir()
	err := utils.EnsureDirExists(configDir)
	if err != nil {
		return
	}

	pathFile := filepath.Join(configDir, "clash_path.json")

	info := struct {
		Path string   `json:"path"`
		Args []string `json:"args"`
	}{
		Path: path,
		Args: args,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return
	}

	_ = os.WriteFile(pathFile, data, 0644)
}

// collectClashProcesses 收集当前运行的Clash进程信息
func collectClashProcesses(processes []*process.Process, cachedPath string, cachedArgs []string) []ProcessInfo {
	var processesToRestart []ProcessInfo
	
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		for _, clashProcessName := range ClashProcessNames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(clashProcessName)) {
				// 尝试获取进程的完整路径
				exePath, err := p.Exe()
				if err != nil {
					log.Printf("无法获取进程 %s (PID: %d) 的路径: %v", name, p.Pid, err)
					// 如果无法获取路径，使用缓存的路径
					if cachedPath != "" {
						exePath = cachedPath
					}
				}

				if exePath != "" {
					// 获取启动命令行
					cmdline, err := p.Cmdline()
					var args []string
					if err != nil {
						log.Printf("无法获取命令行参数: %v", err)
						// 如果无法获取参数，使用缓存的参数
						if cachedArgs != nil {
							args = cachedArgs
						} else {
							args = []string{exePath}
						}
					} else {
						args = parseCommandLine(cmdline)
					}

					// 记录进程信息
					procInfo := ProcessInfo{
						path: exePath,
						name: name,
						pid:  p.Pid,
						cmd:  cmdline,
						args: args,
					}

					log.Printf("记录Clash进程信息: %s (PID: %d) 路径: %s", name, p.Pid, exePath)
					processesToRestart = append(processesToRestart, procInfo)
				}

				// 尝试结束进程
				log.Printf("正在强制结束Clash进程: %s (PID: %d)", name, p.Pid)
				_ = p.Kill() // 忽略可能的权限错误，继续尝试其他进程
			}
		}
	}
	
	return processesToRestart
}

// tryRestartFromCache 尝试从缓存路径启动Clash
func tryRestartFromCache(cachedPath string, cachedArgs []string) bool {
	if cachedPath == "" || !utils.FileExists(cachedPath) {
		return false
	}
	
	log.Printf("尝试使用缓存的路径重启Clash: %s", cachedPath)
	
	var cmd *exec.Cmd
	if len(cachedArgs) > 0 {
		cmd = exec.Command(cachedArgs[0], cachedArgs[1:]...)
	} else {
		cmd = exec.Command(cachedPath)
	}
	
	// 将进程设置为后台运行
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	
	err := cmd.Start()
	if err != nil {
		log.Printf("使用缓存路径启动Clash失败: %v", err)
		return false
	}
	
	log.Printf("成功使用缓存路径启动Clash: %s", cachedPath)
	return true
}

// RestartClash 安全地结束Clash进程并重新启动它
func RestartClash() error {
	// 记录启动时间，用于计算整个过程耗时
	startTime := time.Now()

	// 记录进程信息，用于重启
	type ProcessInfo struct {
		path string
		name string
		pid  int32
		cmd  string
		args []string
	}
	var processesToRestart []ProcessInfo

	// 先加载上次成功的路径
	cachedPath, cachedArgs := loadSuccessfulPathFromFile()
	if cachedPath != "" && utils.FileExists(cachedPath) {
		log.Printf("已加载缓存的Clash路径: %s", cachedPath)
	}

	// 标记是否需要使用缓存
	useCache := false

	// 获取所有进程
	processes, err := process.Processes()
	if err != nil {
		log.Printf("获取进程列表失败: %v, 尝试使用缓存路径", err)
		// 如果无法获取进程，但有缓存路径，标记使用缓存
		useCache = (cachedPath != "")
	} else {
		// 找到所有Clash相关进程并记录信息
		processesToRestart = collectClashProcesses(processes, cachedPath, cachedArgs)
	}

	// 等待所有Clash进程完全结束
	time.Sleep(1 * time.Second)
	
	// 如果没有找到进程，但有缓存路径，也尝试重启
	if len(processesToRestart) == 0 && cachedPath != "" {
		useCache = true
	}

	// 检查是否有进程信息可重启
	if len(processesToRestart) > 0 {
		// 有进程要重启，使用最后找到的进程信息
		procInfo := processesToRestart[len(processesToRestart)-1] // 使用最后找到的进程

		log.Printf("尝试重启Clash: %s", procInfo.path)
		
		// 保存成功的路径信息到文件
		saveSuccessfulPath(procInfo.path, procInfo.args)
		
		// 启动进程
		var cmd *exec.Cmd
		if len(procInfo.args) > 0 {
			cmd = exec.Command(procInfo.args[0], procInfo.args[1:]...)
		} else {
			cmd = exec.Command(procInfo.path)
		}
		
		// 将进程设置为后台运行
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: 0x08000000, // CREATE_NO_WINDOW
		}
		
		err = cmd.Start()
		if err != nil {
			log.Printf("启动Clash失败: %v", err)
			// 启动失败，尝试使用其他方法
			if cachedPath != "" && cachedPath != procInfo.path {
				log.Printf("尝试使用缓存路径启动: %s", cachedPath)
				if tryRestartFromCache(cachedPath, cachedArgs) {
					log.Printf("使用缓存路径启动成功")
				} else {
					// 尝试使用启动器
					err = startClashThroughLauncher()
					if err != nil {
						log.Printf("通过启动器启动失败: %v", err)
					}
				}
			} else {
				// 尝试使用启动器
				err = startClashThroughLauncher()
				if err != nil {
					log.Printf("通过启动器启动失败: %v", err)
				}
			}
		} else {
			log.Printf("成功重启Clash: %s", procInfo.path)
		}
	} else if useCache {
		// 没有找到进程，但有缓存路径，尝试用缓存启动
		if tryRestartFromCache(cachedPath, cachedArgs) {
			log.Printf("使用缓存路径启动成功")
		} else {
			// 尝试使用启动器
			err = startClashThroughLauncher()
			if err != nil {
				log.Printf("通过启动器启动失败: %v", err)
			}
		}
	} else {
		// 没有找到进程，也没有缓存路径，尝试查找可能的Clash可执行文件
		log.Println("未找到运行中的Clash进程，尝试查找Clash可执行文件...")
		
		// 尝试使用启动器
		err = startClashThroughLauncher()
		if err != nil {
			log.Printf("通过启动器启动失败: %v", err)
		}
	}

	// 等待Clash启动并检查是否成功
	success := false
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		if verifyClashRunning() {
			success = true
			break
		}
	}

	// 计算耗时
	elapsed := time.Since(startTime)
	if success {
		log.Printf("Clash重启成功，耗时: %.2f秒", elapsed.Seconds())
		return nil
	} else {
		log.Printf("Clash可能启动失败，耗时: %.2f秒", elapsed.Seconds())
		return fmt.Errorf("未能确认Clash成功启动")
	}
}

// 验证Clash是否确实在运行
func verifyClashRunning() bool {
	// 获取进程列表
	processes, err := process.Processes()
	if err != nil {
		log.Printf("验证时获取进程列表失败: %v", err)
		return false
	}

	// 检查是否有Clash进程
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		for _, clashName := range ClashProcessNames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(clashName)) {
				cpuPercent, _ := p.CPUPercent()
				memPercent, _ := p.MemoryPercent()
				log.Printf("验证成功: 找到 %s (PID: %d, CPU: %.1f%%, 内存: %.1f%%)",
					name, p.Pid, cpuPercent, memPercent)
				return true
			}
		}
	}

	// 尝试连接Clash API作为额外验证
	return CheckAPIConnection("http://127.0.0.1:9090")
}

// findAllClashExecutables 查找系统中所有可能的Clash可执行文件
func findAllClashExecutables() ([]string, error) {
	var results []string

	// 常见的Clash安装位置
	searchPaths := []string{
		os.ExpandEnv("%LOCALAPPDATA%\\Programs\\Clash for Windows"),
		os.ExpandEnv("%PROGRAMFILES%\\Clash for Windows"),
		os.ExpandEnv("%PROGRAMFILES(X86)%\\Clash for Windows"),
		os.ExpandEnv("%APPDATA%\\Clash for Windows"),
		filepath.Join(utils.GetHomeDir(), ".config", "clash"),
	}

	// 搜索这些位置
	for _, basePath := range searchPaths {
		if !utils.FileExists(basePath) {
			continue
		}

		// 尝试找到可执行文件
		for _, name := range []string{
			"Clash for Windows.exe",
			"clash-win64.exe",
			"clash.exe",
			"Clash.Meta.exe",
			"clash-verge.exe",
		} {
			path := filepath.Join(basePath, name)
			if utils.FileExists(path) {
				results = append(results, path)
			}
		}
	}

	return results, nil
}

// parseCommandLine 将命令行字符串解析为参数数组
func parseCommandLine(cmdline string) []string {
	if cmdline == "" {
		return nil
	}

	// 对于Windows，处理引号和转义
	if utils.IsWindows() {
		// 简单分割，忽略引号内的空格
		var args []string
		var arg string
		inQuote := false

		for _, r := range cmdline {
			switch r {
			case '"':
				inQuote = !inQuote
			case ' ':
				if !inQuote {
					if arg != "" {
						args = append(args, arg)
						arg = ""
					}
				} else {
					arg += string(r)
				}
			default:
				arg += string(r)
			}
		}

		if arg != "" {
			args = append(args, arg)
		}

		// 如果首个参数是带有完整路径的可执行文件，只取文件名
		if len(args) > 0 {
			args[0] = filepath.Base(args[0])
		}

		return args
	}

	// 对于其他系统，简单按空格分割
	return strings.Fields(cmdline)
}

// findClashExecutable 尝试查找Clash可执行文件的路径
func findClashExecutable(processName string) (string, error) {
	// 在Windows上尝试查找Clash for Windows的安装位置
	if utils.IsWindows() {
		// 常见的安装位置
		commonPaths := []string{
			os.ExpandEnv("%LOCALAPPDATA%\\Programs\\Clash for Windows"),
			os.ExpandEnv("%PROGRAMFILES%\\Clash for Windows"),
			os.ExpandEnv("%PROGRAMFILES(X86)%\\Clash for Windows"),
		}

		// 检查这些位置
		for _, path := range commonPaths {
			exePath := filepath.Join(path, processName)
			if utils.FileExists(exePath) {
				return exePath, nil
			}
		}
	}

	// 如果找不到，返回错误
	return "", fmt.Errorf("无法找到Clash可执行文件")
}

// startClashThroughLauncher 尝试通过Windows开始菜单或快捷方式启动Clash
func startClashThroughLauncher() error {
	// 提前定义多种常见的启动路径，增加成功率
	startMethods := []struct {
		name     string
		launcher func() *exec.Cmd
	}{
		{
			name: "应用程序启动器",
			launcher: func() *exec.Cmd {
				return utils.CreateHiddenWindowsProcess("explorer.exe", "shell:AppsFolder\\Clash_for_Windows.Clash")
			},
		},
		{
			name: "开始菜单快捷方式",
			launcher: func() *exec.Cmd {
				path := os.ExpandEnv("%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Clash for Windows\\Clash for Windows.lnk")
				return utils.CreateHiddenWindowsProcess("cmd.exe", "/c", "start", "", path)
			},
		},
		{
			name: "桌面快捷方式",
			launcher: func() *exec.Cmd {
				path := filepath.Join(utils.GetHomeDir(), "Desktop", "Clash for Windows.lnk")
				return utils.CreateHiddenWindowsProcess("cmd.exe", "/c", "start", "", path)
			},
		},
		{
			name: "运行命令",
			launcher: func() *exec.Cmd {
				return utils.CreateHiddenWindowsProcess("cmd.exe", "/c", "start", "Clash for Windows")
			},
		},
	}

	// 尝试所有方法
	for _, method := range startMethods {
		cmd := method.launcher()
		err := cmd.Start()
		if err == nil {
			log.Printf("通过%s启动Clash成功", method.name)
			return nil
		}
	}

	return fmt.Errorf("所有已知启动方法均失败")
}
