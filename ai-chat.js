/**
 * Gemini AI 聊天机器人集成模块
 * 与 Chatbox 聊天系统无缝集成
 */

// ============ 全局配置 ============
const GEMINI_CONFIG = {
    API_KEY: "AQ.Ab8RN6KOAcaw53rPLV8Pgua_Xv9yaRrmLqQW-QprX8Be_w39IA",
    API_ENDPOINT: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
    MODEL: "gemini-2.5-flash",
    SYSTEM_PROMPT: "你是一个神人ai，你啥都知道，很会整人，但是又不失正经，说话不会太深奥，一言一行宛如一个推特网友，语言组织能力强悍，毒舌，说话上下文不会太长，很拽，对于各个问题基本都可以迎刃而解，遇到不知道的直接说我不知道，啥都能说。"
};

// ============ 聊天机器人状态 ============
const aiChatState = {
    isInitialized: false,
    conversationHistory: [],
    isWaitingForResponse: false,
    lastMessageTime: 0,
    sessionKey: "ai:gemini",
    storageKey: "ai_chat_history_gemini"  // localStorage 键
};

/**
 * 从 localStorage 恢复消息记录
 */
function loadAIChatHistoryFromStorage() {
    try {
        const stored = localStorage.getItem(aiChatState.storageKey);
        if (stored) {
            const history = JSON.parse(stored);
            cacheMessages[aiChatState.sessionKey] = history;
            console.log(`[AI Chat] 已从本地恢复 ${history.length} 条消息`);
            return history;
        }
    } catch (e) {
        console.warn("[AI Chat] 恢复消息失败:", e);
    }
    return null;
}

/**
 * 保存消息记录到 localStorage
 */
function saveAIChatHistoryToStorage() {
    try {
        const messages = cacheMessages[aiChatState.sessionKey] || [];
        localStorage.setItem(aiChatState.storageKey, JSON.stringify(messages));
    } catch (e) {
        console.warn("[AI Chat] 保存消息失败:", e);
    }
}

/**
 * 初始化 AI 聊天机器人
 * 添加到聊天列表中
 */
function initializeAIChat() {
    if (aiChatState.isInitialized) return;
    
    // 尝试从 localStorage 恢复消息
    const restored = loadAIChatHistoryFromStorage();
    
    if (!restored) {
        // 如果没有保存的记录，初始化新的
        cacheMessages[aiChatState.sessionKey] = [
            {
                id: "ai_init_" + Date.now(),
                sender: "Gemini",
                target_type: "ai",
                content: "👋 嗨，我是 Gemini！有什么需要帮助的吗？",
                timestamp: new Date().toISOString(),
                avatar: "🤖"
            }
        ];
        saveAIChatHistoryToStorage();
    }
    
    aiChatState.isInitialized = true;
    console.log("[AI Chat] 聊天机器人已初始化");
}

/**
 * 将 AI 助手添加到聊天列表
 */
function addAIChatToList() {
    const listContent = document.querySelector('#listContent');
    if (!listContent) return;
    
    // 检查是否已经存在
    if (document.querySelector('[data-chat-id="ai-gemini"]')) {
        return;
    }
    
    const aiItem = document.createElement('div');
    aiItem.className = 'p-2 rounded-lg cursor-pointer transition hover:bg-white/50 group';
    aiItem.dataset.chatId = 'ai-gemini';
    aiItem.dataset.type = 'ai';
    aiItem.innerHTML = `
        <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 flex-1 min-w-0">
                <span class="text-lg flex-shrink-0">🤖</span>
                <div class="flex-1 min-w-0">
                    <div class="text-xs font-semibold text-slate-700 truncate">Gemini 助手</div>
                    <div class="text-[10px] text-slate-400 truncate">AI 对话助手</div>
                </div>
            </div>
            <div class="text-[10px] text-slate-300 group-hover:text-slate-500 flex-shrink-0">💬</div>
        </div>
    `;
    
    aiItem.addEventListener('click', () => switchToAIChat());
    
    // 插入到列表顶部（在现有项之前）
    const firstItem = listContent.firstChild;
    if (firstItem) {
        listContent.insertBefore(aiItem, firstItem);
    } else {
        listContent.appendChild(aiItem);
    }
}

/**
 * 切换到 AI 聊天
 */
function switchToAIChat() {
    activeTarget = {
        type: 'ai',
        id: 'gemini',
        name: 'Gemini 助手',
        avatar: '🤖'
    };
    
    // 更新 UI
    document.querySelectorAll('[data-chat-id]').forEach(el => {
        el.classList.remove('bg-blue-50', 'border-l-2', 'border-blue-500');
    });
    
    const aiItem = document.querySelector('[data-chat-id="ai-gemini"]');
    if (aiItem) {
        aiItem.classList.add('bg-blue-50', 'border-l-2', 'border-blue-500');
    }
    
    // 显示聊天窗口
    showChatWindow();
    renderChatBubbles();
    
    // 关闭侧边栏（移动设备）
    toggleSidebar(false);
    
    console.log("[AI Chat] 已切换到 AI 聊天");
}

/**
 * 显示聊天窗口
 */
function showChatWindow() {
    const emptyView = document.querySelector('#emptyView');
    const activeChatWindow = document.querySelector('#activeChatWindow');
    const activeChatName = document.querySelector('#activeChatName');
    const activeChatMeta = document.querySelector('#activeChatMeta');
    const onlineCountBadge = document.querySelector('#onlineCountBadge');
    
    if (emptyView) emptyView.classList.add('hidden');
    if (activeChatWindow) activeChatWindow.classList.remove('hidden');
    if (activeChatName) activeChatName.textContent = activeTarget?.name || '聊天窗口';
    if (activeChatMeta) activeChatMeta.textContent = activeTarget?.type === 'ai' ? 'AI 对话助手' : '通道就绪';
    if (onlineCountBadge) onlineCountBadge.classList.add('hidden');
}

/**
 * 渲染聊天气泡
 */
function renderChatBubbles() {
    const chatBox = document.querySelector('#chatBox');
    if (!chatBox) return;
    
    chatBox.innerHTML = '';
    
    let sessionKey = "";
    if (activeTarget.type === 'ai') {
        sessionKey = aiChatState.sessionKey;
    } else {
        return;
    }
    
    const messages = cacheMessages[sessionKey] || [];
    
    messages.forEach(msg => {
        const bubble = createMessageBubble(msg);
        chatBox.appendChild(bubble);
    });
    
    // 滚动到最下方
    setTimeout(() => {
        chatBox.scrollTop = chatBox.scrollHeight;
    }, 50);
}

/**
 * 创建消息气泡
 */
function createMessageBubble(msg) {
    const wrapper = document.createElement('div');
    const isOwn = msg.sender === currentUser;
    const isAI = msg.sender === 'Gemini' || msg.target_type === 'ai';
    
    wrapper.className = `flex ${isOwn ? 'justify-end' : 'justify-start'} mb-2`;
    
    const bubble = document.createElement('div');
    bubble.className = `max-w-xs md:max-w-md px-4 py-2 rounded-lg ${
        isOwn 
            ? 'bg-blue-500 text-white rounded-br-none' 
            : 'bg-slate-200 text-slate-800 rounded-bl-none'
    } text-sm break-words`;
    
    // 处理代码块
    if (msg.content.includes('```')) {
        bubble.innerHTML = msg.content.replace(/```([\s\S]*?)```/g, '<pre class="bg-slate-800 text-white p-2 rounded mt-1 overflow-x-auto"><code>$1</code></pre>');
    } else {
        bubble.textContent = msg.content;
    }
    
    // 添加时间戳
    const time = document.createElement('div');
    time.className = 'text-[10px] mt-1 opacity-60';
    time.textContent = formatTime(msg.timestamp);
    
    wrapper.appendChild(bubble);
    wrapper.appendChild(time);
    
    return wrapper;
}

/**
 * 格式化时间
 */
function formatTime(timestamp) {
    const date = new Date(timestamp);
    return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
}

/**
 * 发送消息到 Gemini AI
 */
async function sendMessageToAI(userMessage) {
    if (!userMessage.trim() || aiChatState.isWaitingForResponse) {
        return;
    }
    
    aiChatState.isWaitingForResponse = true;
    
    // 添加用户消息到缓存
    const userMsg = {
        id: `msg_${Date.now()}`,
        sender: currentUser,
        target_type: 'ai',
        content: userMessage,
        timestamp: new Date().toISOString(),
        avatar: document.querySelector('#userAvatar')?.style.backgroundImage || ''
    };
    
    cacheMessages[aiChatState.sessionKey].push(userMsg);
    saveAIChatHistoryToStorage();  // ✨ 保存到本地
    renderChatBubbles();
    
    try {
        // 调用 Gemini API，传入最后3条消息作为上下文
        const response = await callGeminiAPI(userMessage);
        
        if (response && response.trim()) {
            // 添加 AI 响应到缓存
            const aiMsg = {
                id: `msg_${Date.now()}`,
                sender: 'Gemini',
                target_type: 'ai',
                content: response,
                timestamp: new Date().toISOString(),
                avatar: '🤖'
            };
            
            cacheMessages[aiChatState.sessionKey].push(aiMsg);
            saveAIChatHistoryToStorage();  // ✨ 保存到本地
            renderChatBubbles();
        }
    } catch (error) {
        console.error("[AI Chat] 错误:", error);
        
        // 添加错误消息
        const errorMsg = {
            id: `msg_${Date.now()}`,
            sender: 'Gemini',
            target_type: 'ai',
            content: `❌ 出错了: ${error.message || '无法连接到 AI 服务'}`,
            timestamp: new Date().toISOString()
        };
        
        cacheMessages[aiChatState.sessionKey].push(errorMsg);
        saveAIChatHistoryToStorage();  // ✨ 保存到本地
        renderChatBubbles();
    } finally {
        aiChatState.isWaitingForResponse = false;
    }
}

/**
 * 调用 Gemini API
 * 💡 上下文记忆：只发送最后 3 条消息（1条用户 + 1条AI + 新问题）
 */
async function callGeminiAPI(userMessage) {
    try {
        // ✨ 获取最后 3 条消息作为上下文（包括当前消息）
        const allMessages = cacheMessages[aiChatState.sessionKey] || [];
        const recentMessages = allMessages.slice(-3);  // 只取最后3条
        
        // 构建消息链（用于上下文）
        const messageParts = [];
        
        // 添加前面的消息作为上下文
        for (let i = 0; i < recentMessages.length - 1; i++) {
            const msg = recentMessages[i];
            if (msg.sender !== 'Gemini') {
                messageParts.push({
                    role: "user",
                    parts: [{ text: msg.content }]
                });
            } else {
                messageParts.push({
                    role: "model",
                    parts: [{ text: msg.content }]
                });
            }
        }
        
        // 添加当前用户消息
        messageParts.push({
            role: "user",
            parts: [{ text: userMessage }]
        });
        
        const payload = {
            contents: messageParts,
            systemInstruction: {
                role: "user",
                parts: [{
                    text: GEMINI_CONFIG.SYSTEM_PROMPT
                }]
            },
            generationConfig: {
                temperature: 0.9,
                topP: 0.95,
                topK: 40,
                maxOutputTokens: 2048
            }
        };
        
        const response = await fetch(
            `${GEMINI_CONFIG.API_ENDPOINT}?key=${GEMINI_CONFIG.API_KEY}`,
            {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(payload)
            }
        );
        
        if (!response.ok) {
            throw new Error(`API 错误: ${response.status}`);
        }
        
        const data = await response.json();
        
        if (data.candidates && data.candidates[0] && data.candidates[0].content) {
            const textPart = data.candidates[0].content.parts[0];
            return textPart.text;
        }
        
        throw new Error('无效的 API 响应');
    } catch (error) {
        console.error("[Gemini API] 错误:", error);
        throw error;
    }
}

/**
 * 清空 AI 聊天记录
 */
function clearAIChatHistory() {
    const confirmed = confirm('确定要清空所有 AI 聊天记录吗？');
    if (confirmed) {
        cacheMessages[aiChatState.sessionKey] = [
            {
                id: "ai_init_" + Date.now(),
                sender: "Gemini",
                target_type: "ai",
                content: "👋 记录已清空，我们重新开始吧！",
                timestamp: new Date().toISOString()
            }
        ];
        saveAIChatHistoryToStorage();  // ✨ 同步保存到本地
        renderChatBubbles();
        alert("已清空聊天记录");
    }
}

/**
 * 处理主应用中的消息发送
 * 拦截 AI 聊天的消息并转发到 sendMessageToAI
 */
function handleMessageSubmit(e) {
    if (e) e.preventDefault();
    
    const userInput = document.querySelector('#user-input');
    const message = userInput.value.trim();
    
    if (!message) return;
    
    if (activeTarget && activeTarget.type === 'ai') {
        // AI 聊天
        sendMessageToAI(message);
    } else {
        // 普通聊天 - 调用原始的消息发送函数
        if (typeof sendMessage === 'function') {
            sendMessage(message);
        }
    }
    
    userInput.value = '';
    userInput.focus();
}

/**
 * 页面加载完成后初始化
 */
document.addEventListener('DOMContentLoaded', () => {
    console.log("[AI Chat] 等待主应用初始化...");
    
    // 延迟初始化，确保主应用已加载
    setTimeout(() => {
        if (typeof cacheMessages !== 'undefined') {
            initializeAIChat();
            addAIChatToList();
            console.log("[AI Chat] 初始化完成");
        }
    }, 1000);
});

// ============ 导出函数供外部调用 ============
if (typeof window !== 'undefined') {
    window.initializeAIChat = initializeAIChat;
    window.addAIChatToList = addAIChatToList;
    window.switchToAIChat = switchToAIChat;
    window.sendMessageToAI = sendMessageToAI;
    window.clearAIChatHistory = clearAIChatHistory;
    window.handleMessageSubmit = handleMessageSubmit;
}
