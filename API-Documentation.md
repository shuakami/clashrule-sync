# ClashRuleSync API 文档

ClashRuleSync 提供了一套轻量易用的 HTTP API，专门用来管理规则、同步配置，以及实时监控系统运行情况。以下是 API 的具体使用说明，涵盖了详细请求与响应示例，并提供了易懂的调用方式，帮助你快速上手。

---

## 🔑 认证与安全说明

ClashRuleSync API 默认无需认证即可调用，但为了安全考虑，所有接口仅监听在本地（`localhost`），无法被外网直接访问，确保了本地使用的安全性。

---

## 📡 API 接口一览

API 主要分为四类：状态监控、配置管理、规则管理、日志管理，以及测试工具。

---

### 一、系统状态监控 API

#### ▶ 获取当前系统状态  
- **请求方式：** `GET`
- **接口地址：** `/api/status`

该接口用于获取系统当前运行状态，包括 Clash 连接情况、规则更新记录和自动启动设置等。

**响应示例：**
```json
{
  "status": "connected",
  "status_message": "Clash 正在运行，API 连接正常。",
  "clash_running": true,
  "process_detected": true,
  "api_connected": true,
  "last_update_time": "2024-01-20T15:04:05Z",
  "next_update_time": "2024-01-21T03:04:05Z",
  "update_history": [
    {
      "time": "2024-01-20T15:04:05Z",
      "success": true,
      "message": "更新成功"
    }
  ],
  "auto_start_enabled": true,
  "system_auto_start_enabled": false
}
```

#### ▶ 手动触发规则更新  
- **请求方式：** `POST`
- **接口地址：** `/api/update`

立即触发一次规则更新。

**响应示例：**
```json
{
  "status": "ok",
  "success": true,
  "message": "规则更新成功"
}
```

---

### 二、配置管理 API

#### ▶ 获取当前配置  
- **请求方式：** `GET`
- **接口地址：** `/api/config`

**响应示例：**
```json
{
  "clash_api_url": "http://127.0.0.1:9090",
  "clash_api_secret": "",
  "clash_config_path": "C:/Users/username/.config/clash/config.yaml",
  "update_interval": 12,
  "auto_start_enabled": true,
  "system_auto_start_enabled": false,
  "web_port": 8899,
  "rule_providers": [
    {
      "name": "direct",
      "type": "http",
      "behavior": "domain",
      "url": "https://example.com/direct.txt",
      "path": "./ruleset/direct.yaml",
      "interval": 86400,
      "enabled": true
    }
  ]
}
```

#### ▶ 更新配置  
- **请求方式：** `POST`
- **接口地址：** `/api/config`

更新 ClashRuleSync 配置，例如调整 API 地址或更新间隔等。

**请求体示例：**
```json
{
  "clash_api_url": "http://127.0.0.1:9090",
  "clash_api_secret": "",
  "clash_config_path": "C:/Users/username/.config/clash/config.yaml",
  "update_interval": 24,
  "auto_start_enabled": true,
  "system_auto_start_enabled": false
}
```

**响应示例：**
```json
{
  "status": "ok",
  "message": "配置更新成功"
}
```

#### ▶ 开启/关闭自动启动  
- **请求方式：** `POST`
- **接口地址：** `/api/toggle-autostart`

**请求体示例：**
```json
{
  "enabled": true
}
```

**响应示例：**
```json
{
  "status": "ok",
  "message": "自动启动设置已更新"
}
```

---

### 三、规则管理 API

规则管理 API 支持对规则来源进行增删改查，并可直接同步到 Clash。

#### ▶ 获取所有规则  
- **请求方式：** `GET`
- **接口地址：** `/api/rules`

**响应示例：**
```json
{
  "status": "ok",
  "rule_providers": [ /* 规则列表 */ ]
}
```

#### ▶ 添加新规则  
- **请求方式：** `POST`
- **接口地址：** `/api/rules/add`

**请求体示例：**
```json
{
  "rule": {
    "name": "direct",
    "type": "http",
    "behavior": "domain",
    "url": "https://example.com/direct.txt",
    "path": "./ruleset/direct.yaml",
    "interval": 86400
  }
}
```

#### ▶ 编辑已有规则  
- **请求方式：** `POST`
- **接口地址：** `/api/rules/edit`

**请求体示例：**
```json
{
  "index": 0,
  "rule": {
    "name": "direct",
    "type": "http",
    "behavior": "domain",
    "url": "https://example.com/new-direct.txt",
    "path": "./ruleset/direct.yaml",
    "interval": 86400
  }
}
```

#### ▶ 删除规则  
- **请求方式：** `POST`
- **接口地址：** `/api/rules/delete`

**请求体示例：**
```json
{
  "index": 0
}
```

#### ▶ 同步绕过规则到 Clash  
- **请求方式：** `POST`
- **接口地址：** `/api/sync-bypass`

**响应示例：**
```json
{
  "status": "ok",
  "message": "同步成功"
}
```

---

### 四、日志管理 API

#### ▶ 获取系统日志  
- **请求方式：** `GET`
- **接口地址：** `/api/logs?file=app.log&lines=500`

用于获取最近的日志内容，支持自定义行数。

---

### 四、常用测试 API

#### ▶ 测试与 Clash API 的连接  
- **请求方式：** `POST`
- **接口地址：** `/api/test-connection`

**请求体示例：**
```json
{
  "clash_api_url": "http://127.0.0.1:9090",
  "clash_api_secret": ""
}
```

