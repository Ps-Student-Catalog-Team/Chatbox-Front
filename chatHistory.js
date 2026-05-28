/**
 * 聊天记录管理模块
 * 包含消息历史加载、缓存、刷新等功能
 */

/**
 * 从服务器加载特定聊天的历史消息
 * @param {string} type - 聊天类型：public, private, group
 * @param {string} id - 聊天ID
 */
async function loadHistoryMessages(type, id) {
    try {
        const response = await fetch(`${httpUri}/api/messages?type=${type}&id=${id}&limit=200`, {
            method: 'GET',
            headers: authHeaders(),
            credentials: 'same-origin'
        });
        const msgs = await response.json();
        if (Array.isArray(msgs)) {
            const sessionKey = `${type}:${id}`;
            cacheMessages[sessionKey] = msgs;
            if (activeTarget && `${activeTarget.type}:${activeTarget.id}` === sessionKey) {
                renderChatBubbles();
            }
        }
    } catch (err) {
        console.warn("加载历史消息失败", err);
    }
}

/**
 * 自动加载所有私聊和群聊的历史消息
 * 在用户登录后自动调用，提高首次加载体验
 */
async function loadAllPrivateChatHistory() {
    const loadTasks = [];
    
    // 加载所有好友的私聊记录
    if (globalFriends && globalFriends.length > 0) {
        globalFriends.forEach(friend => {
            loadTasks.push(loadHistoryMessages('private', friend));
        });
    }
    
    // 加载所有群聊记录
    if (globalGroups && globalGroups.length > 0) {
        globalGroups.forEach(group => {
            loadTasks.push(loadHistoryMessages('group', group.id.toString()));
        });
    }
    
    // 并行加载所有消息
    if (loadTasks.length > 0) {
        await Promise.all(loadTasks);
        console.log(`[聊天记录] 已自动加载 ${globalFriends.length} 个私聊和 ${globalGroups.length} 个群聊的历史消息`);
    }
}

/**
 * 刷新当前活跃聊天的消息
 */
function refreshAllMessages() {
    if (!activeTarget) {
        alert("请先选择一个聊天窗口");
        return;
    }
    loadHistoryMessages(activeTarget.type, activeTarget.id);
}

/**
 * 清空私聊记录
 * 仅清空本地缓存，不影响服务器数据
 */
function clearPrivateChatHistory() {
    if (!activeTarget || activeTarget.type !== 'private') {
        alert("请先选择一个私聊");
        return;
    }

    const confirmed = confirm(`确定要清空与 "${activeTarget.name}" 的聊天记录吗？\n※ 此操作仅清除本地记录，对方的记录不会被删除`);
    if (confirmed) {
        const sessionKey = `private:${activeTarget.id}`;
        cacheMessages[sessionKey] = [];
        renderChatBubbles();
        alert("已清空聊天记录");
    }
}

/**
 * 处理接收到的消息并路由到对应的聊天窗口
 * @param {object} msg - 接收到的消息对象
 */
function routeIncomingMessage(msg) {
    let sessionKey = "";
    if (msg.target_type === "public") {
        sessionKey = "public:global";
    } else if (msg.target_type === "group") {
        sessionKey = `group:${msg.target_id}`;
    } else {
        const chatPartner = (msg.sender === currentUser) ? msg.target_id : msg.sender;
        sessionKey = `private:${chatPartner}`;
    }
    if (!cacheMessages[sessionKey]) cacheMessages[sessionKey] = [];
    cacheMessages[sessionKey] = cacheMessages[sessionKey].filter(m => {
        return !(m.id < 0 && m.sender === msg.sender && m.content === msg.content && m.target_type === msg.target_type && m.target_id === msg.target_id);
    });
    if (!cacheMessages[sessionKey].some(m => m.id === msg.id)) {
        cacheMessages[sessionKey].push(msg);
        if (cacheMessages[sessionKey].length > 200) cacheMessages[sessionKey].shift();
    }
    if (activeTarget && `${activeTarget.type}:${activeTarget.id}` === sessionKey) {
        renderChatBubbles();
    }
    if (currentTab === "chats") renderSidebarList();
}
