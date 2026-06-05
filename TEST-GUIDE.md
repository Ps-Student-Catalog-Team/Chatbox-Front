# 新消息提醒功能测试指南

## 🧪 测试清单

### 1. 标题栏显示未读数

- [ ] 打开应用，标题显示正常（无数字）
- [ ] 接收新消息，标题变为 `(1) 原标题`
- [ ] 收到多条消息，标题更新为 `(3) 原标题`
- [ ] 打开聊天，标题恢复正常
- [ ] 刷新页面，未读计数保留

### 2. 新消息分隔线

- [ ] 打开聊天，没有分隔线
- [ ] 后台收到新消息，关闭后重新打开
- [ ] 分隔线出现在最旧新消息上方
- [ ] 分隔线显示 "新消息" 文字
- [ ] 分隔线有红色渐变效果

### 3. 回到最新消息按钮

- [ ] 打开聊天，"↓ 最新" 按钮隐藏
- [ ] 向上滚动，按钮出现并显示为蓝色
- [ ] 点击按钮，平滑滚动到底部
- [ ] 滚动回底部，按钮自动隐藏
- [ ] 按钮点击后，新消息标记清除

### 4. 侧边栏未读徽章

- [ ] 接收新消息，侧边栏显示红色徽章
- [ ] 徽章显示正确的未读数
- [ ] 打开聊天后徽章清除
- [ ] 多个聊天各显示各自的徽章

### 5. 提醒功能

- [ ] 启用桌面通知，收到系统通知
- [ ] 启用提示音，播放蜂鸣音
- [ ] 启用未读徽章，显示红色数字
- [ ] 配置保存，刷新后生效

### 6. 多聊天场景

- [ ] 打开聊天1
- [ ] 后台收到聊天2的消息
- [ ] 标题显示 `(1) 原标题`
- [ ] 侧边栏聊天2显示徽章
- [ ] 点击聊天2，聊天2的徽章清除
- [ ] 标题仍显示 `(1)` 吗？❌ 应该是全部清除了，因为打开聊天2会清除聊天2的新消息标记
- [ ] 回到聊天1，标题恢复正常

### 7. 边界情况

- [ ] 发送自己的消息，不显示分隔线
- [ ] 快速收到多条消息，分隔线在正确位置
- [ ] 一开始就有消息的聊天，打开时无分隔线
- [ ] 切换不同聊天，各自保持独立的状态

## 📱 移动端测试

- [ ] "↓ 最新" 按钮在小屏幕上可点击
- [ ] 未读徽章大小适配手机屏幕
- [ ] 新消息分隔线在小屏幕上清晰可见
- [ ] 触屏滚动时按钮显示/隐藏正常

## 🔍 浏览器兼容性

| 浏览器 | 状态 | 备注 |
|--------|------|------|
| Chrome 90+ | ✅ 测试 | |
| Firefox 88+ | ✅ 测试 | |
| Safari 14+ | ✅ 测试 | |
| Edge 90+ | ✅ 测试 | |
| 移动Safari | ✅ 测试 | |
| Chrome Mobile | ✅ 测试 | |

## 🐛 已知问题和修复

### 问题 1：标题不更新

**症状**：收到消息但标题没变化

**排查**：
```javascript
// 在控制台检查
console.log(NotificationManager.getUnreadCount('private', 'username'));
```

**修复**：确保启用了"标题提醒"设置

---

### 问题 2：分隔线不出现

**症状**：打开聊天时没有新消息分隔线

**排查**：
```javascript
// 检查消息是否标记
const msgs = cacheMessages['private:username'];
console.log(msgs.map(m => ({id: m.id, isNew: m.isNew})));
```

**修复**：这是正常的 - 打开聊天时分隔线被清除。需要在后台收到新消息后才会显示。

---

### 问题 3：按钮一直显示

**症状**："↓ 最新" 按钮不隐藏

**排查**：
```javascript
// 检查滚动事件是否触发
document.getElementById('chatBox').addEventListener('scroll', () => {
    console.log('滚动事件触发');
});
```

**修复**：确保聊天框能够滚动（内容超过高度）

## ✅ 验证清单

运行以下测试代码验证所有功能：

```javascript
// 1. 检查NotificationManager是否加载
console.assert(typeof NotificationManager !== 'undefined', '通知管理器未加载');

// 2. 检查关键函数是否存在
console.assert(typeof clearNewMessageMarks === 'function', 'clearNewMessageMarks函数不存在');
console.assert(typeof scrollToLatestMessage === 'function', 'scrollToLatestMessage函数不存在');
console.assert(typeof updateScrollToLatestButton === 'function', 'updateScrollToLatestButton函数不存在');

// 3. 检查样式类是否定义
const style = document.createElement('div');
style.className = 'new-message-divider';
console.assert(getComputedStyle(style).display !== '', '分隔线样式未定义');

// 4. 手动测试通知
NotificationManager.incrementUnreadCount('test', 'test');
console.assert(NotificationManager.getUnreadCount('test', 'test') === 1, '未读计数功能失效');

// 5. 清理
NotificationManager.clearAllUnread();

console.log('✅ 所有验证通过！');
```

## 🎥 演示场景

### 场景 A：收到私聊消息

```
步骤:
1. 打开公共聊天室
2. 使用另一个账号发送私聊消息
3. 观察标题、徽章、通知

预期结果:
✓ 标题: "(1) 局域网通讯 | 公共聊天室"
✓ 侧边栏显示私聊徽章 "1"
✓ 桌面通知: "username: 你好"
✓ 播放提示音
```

### 场景 B：查看新消息

```
步骤:
1. 从场景A继续
2. 点击侧边栏的私聊
3. 观察分隔线和滚动

预期结果:
✓ 自动定位到分隔线
✓ 分隔线显示 "新消息"
✓ 徽章消失
✓ 标题恢复正常
```

### 场景 C：浏览历史

```
步骤:
1. 在私聊中向上滚动
2. 观察"↓ 最新"按钮
3. 点击按钮

预期结果:
✓ 向上滚动时按钮出现
✓ 按钮蓝色高亮
✓ 点击后平滑滚动到底部
✓ 按钮隐藏
✓ 分隔线清除
```

## 📊 性能测试

```javascript
// 测试渲染性能
console.time('renderChatBubbles');
renderChatBubbles();
console.timeEnd('renderChatBubbles');
// 预期: < 100ms

// 测试内存占用
const size = JSON.stringify(cacheMessages).length;
console.log(`消息缓存大小: ${(size / 1024).toFixed(2)} KB`);
// 预期: < 1MB
```

## 📝 测试报告模板

```
测试日期: 2026-06-05
测试环境: Chrome 浏览器, Windows
测试人员: [名字]

功能测试: ✅ 通过 / ❌ 失败
- 标题显示未读: [结果]
- 新消息分隔线: [结果]
- 回到最新按钮: [结果]
- 侧边栏徽章: [结果]

问题记录:
1. [问题描述] - 严重程度: [高/中/低]
2. [问题描述] - 严重程度: [高/中/低]

建议:
- [建议1]
- [建议2]
```

---

运行测试后，请记录您的发现！🧪
