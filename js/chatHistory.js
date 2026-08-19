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
    }
}

function refreshAllMessages() {
    if (!activeTarget) {
        alert("请先选择一个聊天窗口");
        return;
    }
    loadHistoryMessages(activeTarget.type, activeTarget.id);
}


async function clearPrivateChatHistory() {
    if (!activeTarget || activeTarget.type !== 'private') {
        alert("请先选择一个私聊");
        return;
    }

    const confirmed = await showConfirmDialog(`清空与 "${activeTarget.name}" 的聊天记录后无法恢复。\n※ 此操作仅清除本地记录，对方的记录不会被删除`, '确认清空？', 'clear');
    if (confirmed) {
        const sessionKey = `private:${activeTarget.id}`;
        cacheMessages[sessionKey] = [];
        renderChatBubbles();
        alert("已清空聊天记录");
    }
}


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
        // 标记为新消息（在路由阶段），避免为自己发送的消息标记为新消息
        let markNew = false;
        if (msg.sender !== currentUser) {
            const isCurrent = activeTarget && `${activeTarget.type}:${activeTarget.id}` === sessionKey;
            if (isCurrent) {
                // 如果标签页可见并有焦点，不标记为新消息
                if (document.visibilityState === 'visible' && document.hasFocus()) {
                    markNew = false;
                } else {
                    // 浏览器不可见或无焦点：延迟在切回标签页时显示新消息条
                    msg._pendingNew = true;
                }
            } else {
                markNew = true;
            }
        }
        msg.isNew = markNew;
        cacheMessages[sessionKey].push(msg);
        if (cacheMessages[sessionKey].length > 200) cacheMessages[sessionKey].shift();
    }
    if (activeTarget && `${activeTarget.type}:${activeTarget.id}` === sessionKey) {
        renderChatBubbles();
    }
    if (currentTab === "chats") renderSidebarList();
    
    // 触发新消息通知
    if (typeof NotificationManager !== 'undefined' && msg.sender !== currentUser) {
        NotificationManager.handleNewMessage(msg);
    }
}

// 将延迟（在不可见时接收）的“新消息”在切回可见时刷新并展示
function flushPendingNewMessagesForActive() {
    if (!activeTarget) return;
    const sessionKey = `${activeTarget.type}:${activeTarget.id}`;
    const msgs = cacheMessages[sessionKey] || [];
    let changed = false;
    for (let m of msgs) {
        if (m._pendingNew) {
            m.isNew = true;
            delete m._pendingNew;
            changed = true;
        }
    }
    if (changed) {
        try { renderChatBubbles(); } catch (e) { console.warn('刷新延迟新消息失败', e); }
    }
}
