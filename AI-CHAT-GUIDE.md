# Chatbox 聊天机器人集成指南

## 📚 概述

本指南说明如何在 Chatbox-Front 项目中使用集成的 **Gemini AI 聊天机器人**功能。

---

## ✨ 新增功能

### 1. **AI 助手集成**
- 在聊天列表中添加了 "🤖 Gemini 助手" 项
- 支持与 Gemini AI 进行自然对话
- 完全集成到现有的聊天界面中

### 2. **核心特性**
- ✅ 无缝集成 - 与现有聊天功能完全兼容
- ✅ 实时对话 - 支持即时消息传递
- ✅ 消息缓存 - 自动保存对话记录
- ✅ 快速访问 - 从侧边栏直接访问
- ✅ 菜单选项 - 专用的清空记录功能

---

## 🚀 快速开始

### 步骤 1：启动应用

按照原有 Chatbox 的启动方式：

```bash
go run main.go
```

访问 `http://127.0.0.1:40001/index.html`

### 步骤 2：登录账户

输入账户名称和密码登录（或注册新账户）。

### 步骤 3：访问 AI 聊天

在左侧聊天列表中，你会看到：
- **公共聊天室** - 与其他用户聊天
- **🤖 Gemini 助手** - 与 AI 助手聊天
- **其他私聊/群聊** - 与好友聊天

点击 "Gemini 助手" 即可开始与 AI 对话。

---

## 💬 使用示例

### 示例 1：提问

**用户**: "Gemini，如何学好编程？"

**AI 回应**: "学好编程的关键是：1️⃣ 动手写代码，2️⃣ 理解基础概念，3️⃣ 多看好代码，4️⃣ 持续练习。推荐从 Python 或 JavaScript 开始。"

### 示例 2：代码帮助

**用户**: "帮我写个 JavaScript 的快速排序算法"

**AI 回应**: 
```javascript
function quickSort(arr) {
    if (arr.length <= 1) return arr;
    const pivot = arr[0];
    const left = arr.slice(1).filter(x => x < pivot);
    const right = arr.slice(1).filter(x => x >= pivot);
    return [...quickSort(left), pivot, ...quickSort(right)];
}
```

---

## 🔧 配置说明

### 修改 Gemini API Key

编辑 [ai-chat.js](ai-chat.js#L10)：

```javascript
const GEMINI_CONFIG = {
    API_KEY: "你的 API_KEY 地址", // 修改这里
    API_ENDPOINT: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
    MODEL: "gemini-2.5-flash",
    SYSTEM_PROMPT: "你是一个神人ai..." // 可选：修改 AI 性格
};
```

### 修改 AI 性格

在同一文件的 `SYSTEM_PROMPT` 中修改 AI 的行为指示：

```javascript
SYSTEM_PROMPT: "你是一个专业的编程助手，使用严谨的语言..."
```

---

## 📋 功能清单

### 消息界面
- [x] 收发消息
- [x] 时间戳显示
- [x] 消息缓存
- [x] 自动滚动到最新

### 菜单功能
- [x] 🗑️ 清空对话记录
- [x] ℹ️ 关于 Gemini
- [x] 🔔 提醒设置（通用）

### 兼容性
- [x] 桌面端
- [x] 移动端适配
- [x] 响应式设计

---

## ⚙️ 技术架构

### 文件结构

```
Chatbox-Front/
├── index.html           # 主 HTML 文件（已修改）
├── ai-chat.js          # 🆕 AI 聊天模块（核心）
├── chatHistory.js      # 消息历史管理
├── notification.js     # 通知管理
├── style-desktop.css   # 桌面样式
├── style-mobile.css    # 移动样式
└── ... 其他文件
```

### 关键模块

#### ai-chat.js
```javascript
// 全局配置
GEMINI_CONFIG = {
    API_KEY, API_ENDPOINT, MODEL, SYSTEM_PROMPT
}

// 主要函数
initializeAIChat()          // 初始化 AI 模块
addAIChatToList()          // 添加 AI 项到列表
switchToAIChat()           // 切换到 AI 聊天
sendMessageToAI(message)   // 发送消息
callGeminiAPI(message)     // 调用 API
clearAIChatHistory()       // 清空记录
```

---

## 🔐 隐私 & 安全

### ⚠️ 重要提示

1. **API Key 安全**
   - ❌ 不要将 API Key 提交到公开仓库
   - ❌ 生产环境应使用后端代理
   - ✅ 使用环境变量管理敏感信息

2. **消息隐私**
   - 对话记录存储在浏览器本地缓存中
   - 消息将发送到 Gemini 服务器处理
   - 建议不要发送敏感个人信息

---

## 🐛 故障排除

### 问题 1：AI 无法连接

**症状**: "数据链路异常"

**解决方案**:
1. 检查网络连接
2. 验证 API Key 是否正确
3. 检查浏览器控制台错误

```javascript
// 在浏览器控制台运行
console.log(GEMINI_CONFIG.API_KEY);
```

### 问题 2：消息发送但无回复

**症状**: AI 一直不回复

**解决方案**:
1. 检查 API 配额是否用尽
2. 查看浏览器网络选项卡
3. 尝试发送简单的消息

### 问题 3：消息格式混乱

**症状**: 消息显示不正常

**解决方案**:
1. 清空浏览器缓存
2. 运行 `clearAIChatHistory()`
3. 刷新页面

---

## 📚 API 文档

### sendMessageToAI(userMessage)

发送消息给 AI。

**参数**:
- `userMessage` (string): 用户输入的消息

**返回**: void

**示例**:
```javascript
sendMessageToAI("你好，Gemini！");
```

### callGeminiAPI(userMessage)

直接调用 Gemini API（低级接口）。

**参数**:
- `userMessage` (string): 消息内容

**返回**: Promise<string> - AI 的回复

**示例**:
```javascript
const response = await callGeminiAPI("写一个 Hello World 程序");
console.log(response);
```

### clearAIChatHistory()

清空所有 AI 对话记录。

**返回**: void

**示例**:
```javascript
clearAIChatHistory();
```

---

## 🔄 集成说明

### 修改的文件

#### index.html
- ✅ 添加了 `ai-chat.js` 脚本引入
- ✅ 修改了 `sendMessage()` 函数支持 AI 聊天
- ✅ 添加了 AI 菜单项 `aiMenuItems`
- ✅ 修改了 `toggleChatMenu()` 支持 AI 类型
- ✅ 修改了 `renderSidebarList()` 添加 AI 项
- ✅ 修改了 `selectActiveChat()` 支持 AI 类型

#### ai-chat.js（新增）
- 完整的 Gemini API 集成
- 消息缓存管理
- UI 事件处理
- 初始化逻辑

---

## 📈 性能考虑

### 浏览器性能
- 消息缓存可能占用内存
- 建议定期清空旧对话
- 大量消息时性能略有下降

### API 配额
- 免费配额通常有限制
- 监控 API 使用情况
- 可配置请求频率限制

---

## 🎓 高级用法

### 自定义 AI 性格

编辑 `SYSTEM_PROMPT`：

```javascript
// 专业顾问风格
SYSTEM_PROMPT: "你是一位经验丰富的技术顾问..."

// 友好助手风格
SYSTEM_PROMPT: "你是一位热心的朋友，乐于帮助..."

// 幽默风格
SYSTEM_PROMPT: "你是一位爱开玩笑的编程达人..."
```

### 扩展功能

如需添加更多功能（如文件上传、代码执行等），可在 `ai-chat.js` 中扩展：

```javascript
// 示例：添加文件支持
async function sendFileToAI(file) {
    // 实现文件处理逻辑
}
```

---

## 📞 支持 & 反馈

如有问题或建议，请：
1. 检查本文档的故障排除部分
2. 查看浏览器控制台错误
3. 参考 Gemini API 官方文档

---

## 📄 许可证

本模块继承 Chatbox 项目的许可证。

---

## ✅ 检查清单

集成完成后，请确认：

- [ ] `ai-chat.js` 文件已添加到项目
- [ ] `index.html` 已修改并包含 AI 脚本引入
- [ ] Gemini API Key 已配置
- [ ] 可以访问 AI 助手菜单项
- [ ] 可以发送消息并获得回复
- [ ] 消息缓存正常工作
- [ ] 可以清空对话记录

---

**版本**: 1.0.0  
**最后更新**: 2026-06-06  
**作者**: GitHub Copilot AI Assistant
