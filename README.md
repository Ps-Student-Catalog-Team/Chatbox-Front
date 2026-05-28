# 🌐 Chatbox-Front | 局域网即时通讯系统

`Chatbox-Front` 是一款专为局域网环境打造的轻量级、高性能即时通讯（IM）系统。项目采用 **Go 语言（WebSocket）** 驱动高并发后端，前端具备完备的设备自适应能力，支持私聊、加密群聊及全功能管理后台，非常适合校园网、企业内网等限制外网环境下的部署与技术研究。

> 💡 **提示**：本项目亦配有独立的进阶管理端软件：[Chatbox-background](https://github.com/stormsnow2233/Chatbox-background)（需配合本后端核心协同运行）。

---

## ✨ 核心特性

- **🚀 极速响应**：基于 Go + WebSocket 架构，内存占用低，连接稳健，专为内网高并发设计。
- **📱 全端自适应**：前端深度适配 Desktop 与 Mobile 双端视觉布局，支持流畅的高级微交互与毛玻璃（Blur）视觉特效。
- **🛡 安全隐私**：支持完整的用户鉴权、好友系统及独立加密的内部群聊功能。
- **⚙ 内置控制台**：免去繁琐配置，内置独立管理后台，支持公告广播、用户及消息一键管理。

---

## 🚀 快速开始

### 1. 后端部署
确保本地已安装 Go 环境，在项目根目录下执行：
```bash
# 下载依赖并运行后端
go run main.go
```
# 后端服务启动说明

- 默认端口：`:40001`
- 启动后自动在根目录创建本地数据库：`chat.db`

---

## 访问方式

| 类型 | 地址 | 说明 |
|------|------|------|
| 用户聊天客户端 | `http://127.0.0.1:40001/index.html` | 主聊天界面 |
| 内置管理控制台 | `http://127.0.0.1:40001/admin.html` | 管理后台 |
| 忘记密码 | `http://127.0.0.1:40001/forgot-passwprd.html` | 忘记密码修改 |

> ⚠️ 部署于服务器或校园网时，请将 `127.0.0.1` 替换为你的实际内网 IP

### 默认管理密钥 / 密码
*admin666*

---

## 项目架构与核心文件

| 文件名 | 类型 | 核心职能 |
|--------|------|----------|
| `main.go` | Backend Core | 系统中枢，集成 SQLite、RESTful API、WebSocket、鉴权、路由、群组管理、管理员指令 |
| `index.html` | Main UI | 聊天室主界面，支持多端自适应、消息历史缓存、在线人数统计、WebSocket 通信 |
| `admin.html` | Admin Panel | 内置管理后台，暗色科技风登录，支持用户/消息审查、删除、全局广播、全局禁言 |
| `forgot-password.html` | Account Tool | 账户重置面板，通过后端 API 重置密码 |

---

## 技术文档

| 文档 | 说明 |
|------|------|
| [QUICK-START.md](QUICK-START.md) | 5分钟完成上线 |
| [UPDATE-NOTES.md](UPDATE-NOTES.md) | API 路由设计与 WebSocket 信令协议 |
| [README-mobile-adapt.md](README-mobile-adapt.md) | 多端视口适配与动画实现 |
| [LATEST-UPDATES.md](LATEST-UPDATES.md) | 版本迭代历史与功能演进 |
| [IMPLEMENTATION-CHECKLIST.md](IMPLEMENTATION-CHECKLIST.md) | 核心功能清单与未完事项 |

---

## 路线图 (Roadmap)

- [x] 基于 Go + SQLite 的轻量级高并发后端
- [x] 支持多群聊切换及独立会话缓存
- [x] 响应式前端布局

---

## 参与贡献

欢迎提交 Issue 或 Pull Request，共同完善这个有趣的内网通讯工具。
