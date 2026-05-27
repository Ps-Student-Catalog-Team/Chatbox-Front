# 聊天应用更新说明

## 🔧 前端改进内容

### 1. **私聊记录自动加载** ✨
**问题**: 重新打开网页后，私聊记录消失，需要手动刷新

**解决方案**:
- 页面登录后，自动加载所有好友的私聊历史记录
- 自动加载所有群聊的历史记录
- 所有记录缓存到本地，提高加载速度

**改进代码**:
```javascript
// 新增函数：登录后自动加载所有私聊
loadAllPrivateChatHistory()

// 当收到 sync_data 消息时自动触发
case "sync_data":
    globalFriends = data.friends || [];
    globalGroups = data.groups || [];
    renderSidebarList();
    loadAllPrivateChatHistory();  // ← 关键！
    break;
```

### 2. **群聊管理功能** 👥
新增群聊管理界面，群主可以：
- ✅ **修改群名** - 更改群聊名称
- ✅ **发布公告** - 发送重要通知给所有成员
- ✅ **查看成员** - 显示群聊成员列表及其身份
- ✅ **解散群聊** - 群主专属，将所有成员移出
- ✅ **退出群聊** - 普通成员可选择退出

**UI变化**:
- 聊天窗口标题栏新增"⚙️ 设置"按钮（仅在群聊时显示）
- 点击后弹出群聊管理模态框
- 群主看到管理选项，普通成员只能退出

---

## 📡 后端需要实现的接口

### 1. **获取群成员** (已使用)
```
GET /api/group/{groupId}/members

返回格式:
{
  "members": [
    {
      "username": "user1",
      "is_owner": true,
      "joined_at": 1234567890
    },
    {
      "username": "user2",
      "is_owner": false,
      "joined_at": 1234567891
    }
  ]
}
```

### 2. **WebSocket - 重命名群聊** (需要实现)
```
客户端发送:
{
  "action": "rename_group",
  "group_id": "123",
  "new_name": "新群名"
}

服务器响应成功:
{
  "type": "rename_group_ok",
  "group_id": "123",
  "new_name": "新群名"
}

服务器响应失败:
{
  "type": "rename_group_err",
  "content": "错误原因"
}
```

### 3. **WebSocket - 发布公告** (需要实现)
```
客户端发送:
{
  "action": "publish_announcement",
  "group_id": "123",
  "announcement": "这是一条公告"
}

服务器响应成功:
{
  "type": "publish_announcement_ok",
  "group_id": "123"
}

服务器响应失败:
{
  "type": "publish_announcement_err",
  "content": "错误原因"
}

可选: 将公告作为系统消息发送给所有成员
{
  "type": "msg",
  "sender": "system",
  "target_type": "group",
  "target_id": "123",
  "content": "[公告] 这是一条公告",
  "timestamp": 1234567890
}
```

### 4. **WebSocket - 解散群聊** (需要实现)
```
客户端发送:
{
  "action": "disband_group",
  "group_id": "123"
}

服务器响应成功:
{
  "type": "disband_group_ok",
  "group_id": "123"
}

服务器响应失败:
{
  "type": "disband_group_err",
  "content": "错误原因"
}

⚠️ 服务器操作:
1. 验证操作者是否为群主
2. 从所有成员的群聊列表中移除此群
3. 保留群聊的聊天记录（用于查看历史）
4. 通知所有成员群聊已解散
```

### 5. **WebSocket - 退出群聊** (需要实现)
```
客户端发送:
{
  "action": "quit_group",
  "group_id": "123"
}

服务器响应成功:
{
  "type": "quit_group_ok",
  "group_id": "123"
}

服务器响应失败:
{
  "type": "quit_group_err",
  "content": "错误原因"
}

⚠️ 服务器操作:
1. 从群聊成员列表中移除该用户
2. 不影响群聊的其他成员
3. 聊天记录继续保留
```

### 6. **sync_data 返回群主信息** (需要改进)
当前 `sync_data` 返回格式:
```javascript
{
  "type": "sync_data",
  "friends": ["user1", "user2"],
  "groups": [
    {
      "id": 123,
      "name": "群聊名"
    }
  ]
}
```

建议改为:
```javascript
{
  "type": "sync_data",
  "friends": ["user1", "user2"],
  "groups": [
    {
      "id": 123,
      "name": "群聊名",
      "owner": "群主用户名",  // ← 新增
      "created_at": 1234567890 // ← 可选
    }
  ]
}
```

---

## 🔐 权限验证

### 群主权限检查
```javascript
// 前端判断是否为群主
const group = globalGroups.find(g => g.id.toString() === groupId);
const isOwner = group && group.owner === currentUser;

// 后端验证（必须！）
if (messageData.action === "rename_group") {
    // 查询群主信息
    const groupOwner = database.getGroupOwner(groupId);
    if (groupOwner !== currentUser) {
        return { type: "rename_group_err", content: "只有群主才能执行此操作" };
    }
    // ... 执行操作
}
```

---

## 📊 数据流程图

```
用户登录
    ↓
WebSocket 连接 → auth_ok
    ↓
syncUserRelations() 
    ↓
收到 sync_data (好友列表 + 群聊列表)
    ↓
[自动加载所有私聊记录]  ← ✨ 新增！
[自动加载所有群聊记录]  ← ✨ 新增！
    ↓
显示聊天列表（含所有历史记录）

---

用户打开群聊
    ↓
显示"⚙️ 设置"按钮  ← ✨ 新增！
    ↓
用户点击"⚙️ 设置"
    ↓
判断是否为群主
    ├→ 是：显示管理选项（重名、公告、解散、成员）
    └→ 否：显示"退出群聊"选项
```

---

## 🧪 测试清单

- [ ] **私聊记录**: 登录后所有私聊历史都显示
- [ ] **私聊刷新**: 不需要手动点击刷新
- [ ] **群聊管理**:
  - [ ] 群主点击"⚙️ 设置"显示管理界面
  - [ ] 普通成员看不到管理选项
  - [ ] 修改群名功能正常
  - [ ] 发布公告功能正常
  - [ ] 查看成员列表正常
  - [ ] 解散群聊功能正常
  - [ ] 退出群聊功能正常
- [ ] **错误处理**: 所有操作有错误提示

---

## 💾 数据库变更建议

### 群聊表
```sql
-- 原有字段
id, name, created_by (或 owner)

-- 建议添加
owner_username VARCHAR(255)      -- 群主用户名（便于查询）
created_at TIMESTAMP             -- 创建时间
announcement TEXT                -- 当前公告
announcement_updated_at TIMESTAMP -- 公告更新时间
```

### 群成员表
```sql
-- 原有字段
group_id, user_id (或 username)

-- 建议添加
is_owner BOOLEAN DEFAULT FALSE    -- 是否为群主
joined_at TIMESTAMP               -- 加入时间
```

---

## 🚀 部署步骤

1. **前端部分**:
   - ✅ 已完成（新的 index.html 包含所有代码）
   - 只需部署新版本

2. **后端部分**:
   - [ ] 实现 6 个新的 WebSocket 消息处理
   - [ ] 实现 `GET /api/group/{groupId}/members`
   - [ ] 修改 `sync_data` 返回格式（添加 owner）
   - [ ] 更新数据库表结构
   - [ ] 充分测试所有新功能

3. **验证**:
   ```bash
   # 测试群主能否解散群聊
   # 测试成员能否退出群聊
   # 测试私聊是否自动加载
   # 测试所有错误提示
   ```

---

## 📝 注意事项

1. **权限验证必须在后端进行**
   - 前端判断只是为了改善UI体验
   - 所有敏感操作都必须在后端验证

2. **并发问题**
   - 群主解散群聊时，若有成员还在发消息，需要处理
   - 建议使用事务确保原子性

3. **离线消息**
   - 如果用户离线时群聊被解散，重新登录时应有提示
   - 可考虑在 `sync_data` 中返回最近的系统通知

4. **群聊公告**
   - 当前实现是简单的发送公告
   - 可考虑在群聊界面顶部固定显示最新公告

---

## 📞 FAQ

**Q: 为什么需要自动加载私聊记录？**
A: 用户期望刷新页面后聊天记录仍然存在，这是现代IM应用的基础体验。

**Q: 群聊管理功能是否可选？**
A: 是的，如果你的应用暂时不需要这些功能，可以不实现。前端代码会正常工作，管理按钮只是不显示。

**Q: 如何处理群名修改后的同步？**
A: 前端会立即更新 `activeTarget.name`，侧边栏列表会重新渲染。建议后端在修改成功后广播给所有群成员。

**Q: 解散群聊后能否恢复？**
A: 不能。这是设计的，群聊解散意味着移除所有成员。聊天记录可以保留以便查看历史。

---

**所有改动都是向后兼容的。没有实现新后端接口的情况下，应用仍可正常运行！** ✅
