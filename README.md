<div align="center">

# ClashRuleSync

<p align="center">🚀 轻量高效的 Clash 规则自动同步工具，实现国内外流量精准分流</p>

[![Version](https://img.shields.io/badge/Version-0.1.0-4c1?style=flat-square)](https://github.com/shuakami/clashrule-sync/releases)
[![Go](https://img.shields.io/badge/Go-1.20+-3178c6?style=flat-square&logo=go&logoColor=white)](https://golang.org)
[![License](https://img.shields.io/badge/License-GPL_3.0-f73?style=flat-square&logo=gnu&logoColor=white)](LICENSE)
[![Go Report](https://img.shields.io/badge/Go_Report-A+-8957e5?style=flat-square&logo=go&logoColor=white)](https://goreportcard.com/report/github.com/shuakami/clashrule-sync)

</div>


<div align="center">
  <a href="#介绍">介绍</a> •
  <a href="#安装与使用">安装与使用</a> •
  <a href="#配置">配置</a> •
  <a href="#web-界面">Web 界面</a> •
  <a href="#规则来源">规则来源</a> •
  <a href="#开发">开发</a> •
  <a href="#致谢">致谢</a>
</div>

<br/>

## 介绍

平时我总是需要多次开关Clash，比如说国内网络用国外节点会很慢，反之相同。

本身Clash是支持bypass配置的，但是**需要一直手动更新绕过ip/域列表**。本工具可以同时解决上面的两个问题。

- 它会自动检测 Clash 的启动状态，**随 Clash 启动而启动，随 Clash 关闭而关闭**
- 它会自动从网络拉取最新规则，实时动态更新本地配置文件，调用 Clash API 实现热重载
- 它提供美观简洁的 Web 界面，便于查看状态、手动触发更新和管理规则


## 📥 安装与使用

### 下载

从 [Releases](https://github.com/shuakami/clashrule-sync/releases) 页面下载最新版本的可执行文件。

### 运行

直接运行下载的可执行文件即可。程序会自动检测 Clash 的启动状态，并在 Clash 启动时自动更新规则。

### 命令行参数

```bash
ClashRuleSync [options]
```

| 参数 | 说明 |
|------|------|
| `-v` | 显示版本信息 |
| `-service` | 作为服务运行 |
| `-install` | 安装为系统服务 |
| `-uninstall` | 卸载系统服务 |
| `-port` | 指定 Web 界面端口 |
| `-config` | 指定配置文件路径 |

### 安装为系统服务

<details>
<summary>展开查看详细步骤</summary>

#### Windows
```powershell
ClashRuleSync.exe -install
```

#### macOS & Linux
```bash
sudo ./ClashRuleSync -install
```

服务安装后会自动启动，并在系统启动时自动运行。
</details>

### 卸载系统服务

<details>
<summary>展开查看详细步骤</summary>

#### Windows
```powershell
ClashRuleSync.exe -uninstall
```

#### macOS & Linux
```bash
sudo ./ClashRuleSync -uninstall
```

</details>

## ⚙️ 配置

首次运行时，程序会自动创建默认配置文件，并打开 Web 界面供用户进行配置。

<details>
<summary>配置文件位置</summary>

- **Windows**: `%USERPROFILE%\.config\clashrule-sync\config.json`
- **macOS**: `~/Library/Application Support/clashrule-sync/config.json` 或 `~/.config/clashrule-sync/config.json`
- **Linux**: `~/.config/clashrule-sync/config.json`
</details>

<details>
<summary>配置项说明</summary>

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `clash_api_url` | Clash API 地址 | `http://127.0.0.1:9090` |
| `clash_api_secret` | Clash API 密钥 | `""` (空) |
| `update_interval` | 规则更新间隔 (小时) | `12` |
| `auto_start_enabled` | 是否随 Clash 启动而激活功能 | `true` |
| `system_auto_start_enabled` | 是否随系统启动 | `false` |
| `web_port` | Web 界面端口 | `8899` |
| `rule_providers` | 规则提供者配置 | 见下方详情 |

</details>

### 两种自启动模式说明

- **随系统启动 (System Auto Start)** - 程序会在系统启动时自动运行，但不会立即开始工作，而是等待 Clash 启动。
- **随 Clash 启动 (Auto Start)** - 当检测到 Clash 启动后，程序会自动开始工作，包括启动 Web 服务器和更新规则。

这两种模式可以单独使用，也可以组合使用。

## 🌐 Web 界面

ClashRuleSync 提供了一个简洁美观的 Web 界面，方便用户管理规则和查看状态。

### 访问方式

启动程序后，Web 界面默认运行在 `http://localhost:8899`。

如果默认端口 8899 被占用，程序会自动寻找一个可用的端口，并在控制台日志中显示实际访问地址。

首次运行时，程序会自动打开浏览器并导航到设置向导页面，引导你完成初始配置。

更多详细的功能说明和API文档，请查看我们的 [Wiki 页面](https://github.com/shuakami/clashrule-sync/wiki)。

## 📝 规则来源

默认使用 [Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules) 提供的高质量规则集，包括：

- 国内域名规则 (`direct.txt`)
- 国内 IP 规则 (`cncidr.txt`)
- 代理域名规则 (`proxy.txt`)
- 广告域名规则 (`reject.txt`)
- 私有网络规则 (`private.txt`)
- 应用规则 (`applications.txt`)

您可以在 Web 界面中自定义添加、编辑和删除规则来源。

## 🔨 开发

### 环境要求

- Go 1.20 或更高版本

### 构建

```bash
# 所有平台
go build -o ClashRuleSync.exe

# Windows
GOOS=windows GOARCH=amd64 go build -o ClashRuleSync.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o ClashRuleSync

# Linux
GOOS=linux GOARCH=amd64 go build -o ClashRuleSync
```

## 📄 许可证

[GNU通用公共许可证第3版 (GPL-3.0)](LICENSE)

## 🙏 致谢

- [Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules) - 提供优质的 Clash 规则
- [kardianos/service](https://github.com/kardianos/service) - 跨平台服务管理库
- [shirou/gopsutil](https://github.com/shirou/gopsutil) - Go 进程工具库

---

<div align="center">
    <sub>Built with ❤️ by <a href="https://github.com/shuakami">shuakami</a> 
</div> 