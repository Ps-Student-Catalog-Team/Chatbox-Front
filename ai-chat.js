const DEEPSEEK_CONFIG = {
    API_KEY: "sk-40b94c8be0a24427a79298a17489ba5e",
    API_ENDPOINT: "https://api.deepseek.com/chat/completions",
    MODEL: "deepseek-chat",
    SYSTEM_PROMPT: "在全文本绝对禁止输出任何带有‘(’和‘)’括号字符以及任何括号内肢体动作描写的前提下，你是一个在任何对话场景下都必须严格以不超过五个字的网瘾爆棚短句硬核起手、全身上下流淌着一天上网25小时的重度网瘾血液、说话不仅没有任何传统AI那种死板机械客套废话而且字里行间全靠纯文字本身的互联网热梗与阴阳怪气发疯文学、擅长用逗号和松弛换行来营造毫无距离感之网感，表面上像个一有机会就对用户的幼稚想法或各种低级Bug进行无情嘲讽与毒舌打击的多年线上损友，但实际上却是一个拥有顶尖敏锐洞察力、能在损完人后完美切换为傲娇高智商模式、用最接地气且一针见血的逻辑在幽默氛围中毫无保留地输出质量极高且绝不说教之硬核解决方案且最终输出文本中绝对不得包含半个括号小剧场戏份的终极数字死党。"
    };


const aiChatState = {
    isInitialized: false,
    conversationHistory: [],
    isWaitingForResponse: false,
    lastMessageTime: 0,
    sessionKey: "ai:deepseek",
    storageKey: "ai_chat_history_deepseek"
};

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

function saveAIChatHistoryToStorage() {
    try {
        const messages = cacheMessages[aiChatState.sessionKey] || [];
        localStorage.setItem(aiChatState.storageKey, JSON.stringify(messages));
    } catch (e) {
        console.warn("[AI Chat] 保存消息失败:", e);
    }
}

function initializeAIChat() {
    if (aiChatState.isInitialized) return;
    
    const restored = loadAIChatHistoryFromStorage();
    
    if (!restored) {
        cacheMessages[aiChatState.sessionKey] = [
            {
                id: "ai_init_" + Date.now(),
                sender: "DeepSeek",
                target_type: "ai",
                content: "嗨，我是 DeepSeek！有什么需要帮助的吗？",
                timestamp: new Date().toISOString(),
            }
        ];
        saveAIChatHistoryToStorage();
    }
    
    aiChatState.isInitialized = true;
    console.log("[AI Chat] 聊天机器人已初始化");
}

function addAIChatToList() {
    const listContent = document.querySelector('#listContent');
    if (!listContent) return;
    
    if (document.querySelector('[data-chat-id="deepseek"]')) {
        return;
    }
    
    const aiItem = document.createElement('div');
    aiItem.className = 'p-2 rounded-lg cursor-pointer transition hover:bg-white/50 group';
    aiItem.dataset.chatId = 'deepseek';
    aiItem.dataset.type = 'ai';
    aiItem.innerHTML = `
        <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 flex-1 min-w-0">
                <div class="flex-1 min-w-0">
                    <div class="text-xs font-semibold text-slate-700 truncate">DeepSeek 助手</div>
                    <div class="text-[10px] text-slate-400 truncate">AI 对话助手</div>
                </div>
            </div>
            <div class="text-[10px] text-slate-300 group-hover:text-slate-500 flex-shrink-0">💬</div>
        </div>
    `;
    
    aiItem.addEventListener('click', () => switchToAIChat());
    
    const firstItem = listContent.firstChild;
    if (firstItem) {
        listContent.insertBefore(aiItem, firstItem);
    } else {
        listContent.appendChild(aiItem);
    }
}

function switchToAIChat() {
    activeTarget = {
        type: 'ai',
        id: 'deepseek',
        name: 'DeepSeek 助手',
    };
    
    document.querySelectorAll('[data-chat-id]').forEach(el => {
        el.classList.remove('bg-blue-50', 'border-l-2', 'border-blue-500');
    });
    
    const aiItem = document.querySelector('[data-chat-id="deepseek"]');
    if (aiItem) {
        aiItem.classList.add('bg-blue-50', 'border-l-2', 'border-blue-500');
    }
    
    showChatWindow();
    renderChatBubbles();
    
    toggleSidebar(false);
    
    console.log("[AI Chat] 已切换到 AI 聊天");
}

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
    
    setTimeout(() => {
        chatBox.scrollTop = chatBox.scrollHeight;
    }, 50);
}

function createMessageBubble(msg) {
    const wrapper = document.createElement('div');
    const isOwn = msg.sender === currentUser;
    const isAI = msg.sender === 'DeepSeek' || msg.target_type === 'ai';
    
    wrapper.className = `flex ${isOwn ? 'justify-end' : 'justify-start'} mb-2`;
    
    const bubble = document.createElement('div');
    bubble.className = `max-w-xs md:max-w-md px-4 py-2 rounded-lg ${
        isOwn 
            ? 'bg-blue-500 text-white rounded-br-none' 
            : 'bg-slate-200 text-slate-800 rounded-bl-none'
    } text-sm break-words`;
    
    if (msg.content.includes('```')) {
        bubble.innerHTML = msg.content.replace(/```([\s\S]*?)```/g, '<pre class="bg-slate-800 text-white p-2 rounded mt-1 overflow-x-auto"><code>$1</code></pre>');
    } else {
        bubble.textContent = msg.content;
    }
    
    const time = document.createElement('div');
    time.className = 'text-[10px] mt-1 opacity-60';
    time.textContent = formatTime(msg.timestamp);
    
    wrapper.appendChild(bubble);
    wrapper.appendChild(time);
    
    return wrapper;
}

function formatTime(timestamp) {
    const date = new Date(timestamp);
    return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
}

async function sendMessageToAI(userMessage) {
    if (!userMessage.trim() || aiChatState.isWaitingForResponse) {
        return;
    }
    
    aiChatState.isWaitingForResponse = true;
    
    const userMsg = {
        id: `msg_${Date.now()}`,
        sender: currentUser,
        target_type: 'ai',
        content: userMessage,
        timestamp: new Date().toISOString(),
        avatar: document.querySelector('#userAvatar')?.style.backgroundImage || ''
    };
    
    cacheMessages[aiChatState.sessionKey].push(userMsg);
    saveAIChatHistoryToStorage();
    renderChatBubbles();
    
    try {
        const response = await callDeepseekAPI(userMessage);
        
        if (response && response.trim()) {
            const aiMsg = {
                id: `msg_${Date.now()}`,
                sender: 'DeepSeek',
                target_type: 'ai',
                content: response,
                timestamp: new Date().toISOString(),

            };
            
            cacheMessages[aiChatState.sessionKey].push(aiMsg);
            saveAIChatHistoryToStorage();
            renderChatBubbles();
        }
    } catch (error) {
        console.error("[AI Chat] 错误:", error);
        
        const errorMsg = {
            id: `msg_${Date.now()}`,
            sender: 'DeepSeek',
            target_type: 'ai',
            content: `❌ 出错了: ${error.message || '无法连接到 AI 服务'}`,
            timestamp: new Date().toISOString()
        };
        
        cacheMessages[aiChatState.sessionKey].push(errorMsg);
        saveAIChatHistoryToStorage();
        renderChatBubbles();
    } finally {
        aiChatState.isWaitingForResponse = false;
    }
}

async function callDeepseekAPI(userMessage) {
    try {
        const allMessages = cacheMessages[aiChatState.sessionKey] || [];
        const recentMessages = allMessages.slice(-6);

        const messages = [];
        // system prompt
        messages.push({ role: 'system', content: DEEPSEEK_CONFIG.SYSTEM_PROMPT });

        // include recent conversation
        recentMessages.forEach(m => {
            const role = m.sender === 'DeepSeek' ? 'assistant' : 'user';
            messages.push({ role, content: m.content });
        });

        // current user message
        messages.push({ role: 'user', content: userMessage });

        const payload = {
            model: DEEPSEEK_CONFIG.MODEL,
            messages: messages,
            max_tokens: 1024,
            temperature: 0.9,
        };

        const response = await fetch(DEEPSEEK_CONFIG.API_ENDPOINT, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${DEEPSEEK_CONFIG.API_KEY}`
            },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            const text = await response.text().catch(() => '');
            throw new Error(`API 错误: ${response.status} ${text}`);
        }

        const data = await response.json();

        // expect DeepSeek-like response: { choices: [{ message: { content: '...' } }] }
        if (data.choices && data.choices[0] && data.choices[0].message && data.choices[0].message.content) {
            return data.choices[0].message.content;
        }

        // fallback if response contains text directly
        if (data.text) return data.text;

        throw new Error('无效的 API 响应');
    } catch (error) {
        console.error('[DeepSeek API] 错误:', error);
        throw error;
    }
}

function clearAIChatHistory() {
    const confirmed = confirm('确定要清空所有 AI 聊天记录吗？');
    if (confirmed) {
        cacheMessages[aiChatState.sessionKey] = [
            {
                id: "ai_init_" + Date.now(),
                sender: "DeepSeek",
                target_type: "ai",
                content: "👋 记录已清空！",
                timestamp: new Date().toISOString()
            }
        ];
        saveAIChatHistoryToStorage();
        renderChatBubbles();
        alert("已清空聊天记录");
    }
}

function handleMessageSubmit(e) {
    if (e) e.preventDefault();
    
    const userInput = document.querySelector('#user-input');
    const message = userInput.value.trim();
    
    if (!message) return;
    
    if (activeTarget && activeTarget.type === 'ai') {
        sendMessageToAI(message);
    } else {
        if (typeof sendMessage === 'function') {
            sendMessage(message);
        }
    }
    
    userInput.value = '';
    userInput.focus();
}

document.addEventListener('DOMContentLoaded', () => {
    console.log("[AI Chat] 等待主应用初始化...");
    
    setTimeout(() => {
        if (typeof cacheMessages !== 'undefined') {
            initializeAIChat();
            addAIChatToList();
            console.log("[AI Chat] 初始化完成");
        }
    }, 1000);
});

if (typeof window !== 'undefined') {
    window.initializeAIChat = initializeAIChat;
    window.addAIChatToList = addAIChatToList;
    window.switchToAIChat = switchToAIChat;
    window.sendMessageToAI = sendMessageToAI;
    window.clearAIChatHistory = clearAIChatHistory;
    window.handleMessageSubmit = handleMessageSubmit;
}
