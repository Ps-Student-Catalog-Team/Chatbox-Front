# 新消息提醒功能文档

## 功能概述

已为 Chatbox 聊天应用添加完整的新消息提醒系统，包括：

### 🔔 提醒类型

1. **桌面通知** (Desktop Notifications)
   - 在系统通知中显示新消息
   - 支持点击通知快速跳转到聊天窗口
   - 自动关闭（4秒后）

2. **提示音** (Sound Notification)
   - 新消息到达时播放蜂鸣音
   - 使用 Web Audio API 生成（无需额外音频文件）

3. **未读徽章** (Unread Badge)
   - 侧边栏显示红色徽章
   - 标记未读消息数量（99+显示上限）
   - 动画弹出效果

4. **标题提醒** (Title Blink)
   - 浏览器标签页标题闪烁
   - 格式：`🔔 (X) 新消息`
   - 用户打开聊天后自动停止闪烁

## 核心文件

### 新增文件
- **`notification.js`** - 通知管理核心模块

### 修改文件
- **`index.html`** - 集成通知系统、添加设置UI
- **`chatHistory.js`** - 在消息路由中触发通知
- **`style-desktop.css`** - 徽章样式（桌面版）
- **`style-mobile.css`** - 徽章样式（移动版）

## 使用方式

### 1. 打开通知设置

点击聊天窗口右上角的 **⋮** 菜单，选择 **"提醒设置"**

![菜单位置](menu.png)

### 2. 配置各项提醒

在弹出的设置面板中切换各项功能：

- ✅ **桌面通知** - 首次启用会请求浏览器权限
- ✅ **提示音** - 新消息播放提示音
- ✅ **未读徽章** - 侧边栏显示未读计数
- ✅ **标题提醒** - 标签页标题闪烁

### 3. 权限授予

首次启用桌面通知时，浏览器会显示权限申请对话框：

```
此网站想向您发送通知
[允许] [不允许]
```

点击 **"允许"** 才能接收桌面通知。

### 4. 通知行为

当有新消息到达时：
- 🔊 播放提示音
- 🔔 显示系统桌面通知（如果已授权）
- 🏷️ 侧边栏显示未读红色徽章
- 📌 浏览器标题开始闪烁
- 🖱️ 点击通知可快速跳转到该聊天

### 5. 清除通知

打开对应的聊天窗口后：
- 自动清除该聊天的未读计数
- 自动停止标题闪烁（如果没有其他未读消息）
- 浏览器获得焦点时，也会清除当前聊天的未读

## API 接口

### NotificationManager 对象

提醒系统通过全局 `NotificationManager` 对象控制，提供以下接口：

#### 初始化

```javascript
// 初始化并请求通知权限
await NotificationManager.init();

// 加载本地保存的设置
NotificationManager.loadSettings();
```

#### 消息处理

```javascript
// 处理新消息（自动触发）
NotificationManager.handleNewMessage(messageObject);

// 打开聊天窗口时重置未读
NotificationManager.onChatWindowOpened(type, id);
```

#### 手动控制

```javascript
// 获取某个聊天的未读计数
const count = NotificationManager.getUnreadCount('private', 'username');

// 增加未读计数
NotificationManager.incrementUnreadCount('group', '123');

// 重置特定聊天的未读计数
NotificationManager.resetUnreadCount('private', 'username');

// 清空所有未读
NotificationManager.clearAllUnread();

// 手动播放提示音
NotificationManager.playNotificationSound();

// 显示桌面通知
NotificationManager.showDesktopNotification('标题', {
    body: '消息内容',
    tag: 'unique-tag'
});
```

#### 设置管理

```javascript
// 获取当前配置
const config = NotificationManager.getConfig();

// 保存设置（自动持久化到本地存储）
NotificationManager.saveSettings({
    enableDesktopNotification: true,
    enableSound: false,
    enableBadge: true,
    enableTitleBlink: true
});
```

## 配置项

在 `notification.js` 中的 `CONFIG` 对象定义：

```javascript
const CONFIG = {
    enableDesktopNotification: true,  // 桌面通知
    enableSound: true,                // 提示音
    enableBadge: true,                // 未读徽章
    enableTitleBlink: true,           // 标题闪烁
    soundUrl: './notification.mp3',   // 音频文件（可选）
};
```

## 本地存储

所有设置都保存在浏览器本地存储中，键名为 `notification_settings`：

```javascript
// 示例存储内容
{
    "enableDesktopNotification": true,
    "enableSound": true,
    "enableBadge": true,
    "enableTitleBlink": true
}
```

用户的设置会在刷新页面后保留。

## 浏览器兼容性

| 功能 | Chrome | Firefox | Safari | Edge |
|------|--------|---------|--------|------|
| 桌面通知 | ✅ 80+ | ✅ 110+ | ✅ 13+ | ✅ 80+ |
| 提示音 (Web Audio API) | ✅ | ✅ | ✅ | ✅ |
| 未读徽章 | ✅ | ✅ | ✅ | ✅ |
| 标题闪烁 | ✅ | ✅ | ✅ | ✅ |

## 场景示例

### 场景 1：后台接收消息

1. 用户在浏览器中打开聊天应用
2. 用户切换到其他标签页/应用
3. 收到新私聊消息
4. **效果**：
   - 🔊 播放提示音
   - 🔔 显示桌面通知："username: 你好！"
   - 🏷️ 侧边栏显示红色徽章 "1"
   - 📌 标题变为 "🔔 (1) 新消息" 并闪烁

### 场景 2：群聊提醒

1. 用户正在查看公共聊天室
2. 收到群聊消息
3. **效果**：
   - 群聊显示红色徽章
   - 如果用户不在该群聊窗口，则显示桌面通知
   - 打开该群聊后自动清除徽章

### 场景 3：焦点恢复

1. 用户离开浏览器一段时间
2. 收到多条消息，有多个聊天显示未读
3. 用户返回浏览器并点击其中一个聊天
4. **效果**：
   - 点击的聊天未读清除
   - 其他聊天的未读保留
   - 如果还有未读，标题继续闪烁

## 常见问题

### Q1：为什么没有收到桌面通知？

**A：** 可能原因：
1. 未授予浏览器通知权限
   - 解决：在设置中重新启用桌面通知，授予权限
2. 浏览器标签页被停用
   - 解决：点击浏览器标签页使其获得焦点
3. 系统通知被禁用
   - 解决：检查操作系统的通知设置

### Q2：提示音不播放怎么办？

**A：** 
1. 检查浏览器音量设置
2. 检查系统音量
3. 尝试禁用后重新启用提示音设置
4. 在浏览器设置中确保允许音频播放

### Q3：如何完全禁用所有提醒？

**A：** 在设置面板中全部关闭，或使用开发者工具：

```javascript
NotificationManager.saveSettings({
    enableDesktopNotification: false,
    enableSound: false,
    enableBadge: false,
    enableTitleBlink: false
});
```

### Q4：如何清除本地存储的设置？

**A：** 使用浏览器开发者工具：

```javascript
// 清除设置
localStorage.removeItem('notification_settings');

// 刷新页面后使用默认配置
```

## 调试模式

在浏览器控制台中查看调试日志：

```javascript
// 查看当前配置
console.log(NotificationManager.getConfig());

// 手动测试通知
NotificationManager.playNotificationSound();
NotificationManager.showDesktopNotification('测试标题', {
    body: '这是一条测试通知'
});

// 手动增加未读
NotificationManager.incrementUnreadCount('private', 'testuser');
```

## 性能优化

- 通知系统采用模块模式，避免全局污染
- 未读计数存储在内存中，高效查询
- 徽章更新通过 querySelector 批量处理
- 定时器使用 clearInterval 及时清理

## 安全考虑

- 不存储任何敏感信息在本地存储
- 仅存储用户的偏好设置
- 通知内容不包含隐私数据（消息预览可选）
- 符合浏览器的通知 API 安全策略

## 未来改进

- [ ] 支持自定义通知音效
- [ ] 支持按聊天类型设置不同提醒级别
- [ ] 支持勿扰模式时间段设置
- [ ] 支持消息内容预览选项
- [ ] 支持静音特定用户
- [ ] 集成 Service Worker 支持后台通知
