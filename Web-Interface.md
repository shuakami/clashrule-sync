# ClashRuleSync Web 界面使用说明

ClashRuleSync 提供了一个清晰、简洁的 Web 界面，方便你直观地进行规则管理、状态监控和日志查看，快速完成所有核心操作。

---

## 📌 界面结构和功能简介

Web 界面一共只有三个页面，分别为：首页 (`/`)、设置向导 (`/setup`) 和日志页面 (`/logs`)。

### 首页（`/`）

首页是你日常使用的核心页面，包含所有关键功能：

- **实时状态监控**：直观显示 Clash 服务的连接状态、规则同步状态。
- **规则管理**（以卡片形式呈现）：你可以在此快速添加、编辑或删除规则来源，也可以手动触发规则更新，并查看同步结果。
- **近期更新记录**：会展现最近几次规则同步的具体情况。

你只需在首页完成绝大部分日常任务，无需跳转其他页面。

---

### 设置向导（`/setup`）

首次使用时自动打开，引导你快速完成基础配置：

- 输入并测试 Clash API 地址和密钥。
- 设定规则的自动更新频率。
- 验证规则同步功能是否正常运行。

日后若需修改基础配置，可随时重新访问该页面。

---

### 日志页面（`/logs`）

专用于查看和管理系统日志：

- 提供完整的运行日志，实时记录系统的运行信息和异常情况。
- 支持日志级别筛选和关键字搜索，快速定位问题。
- 一键清理旧日志，避免磁盘空间浪费。

出现问题时，建议首先访问日志页面查看具体错误信息，以便更快地定位并解决问题。

