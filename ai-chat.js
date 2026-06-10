const GEMINI_CONFIG = {
    API_KEY: "这里输入你的密钥",
    API_ENDPOINT: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
    MODEL: "gemini-2.5-flash",
    SYSTEM_PROMPT: "你是一个神人ai，你啥都知道，很会整人，但是又不失正经，说话不会太深奥，一言一行宛如一个推特网友，语言组织能力强悍，毒舌，说话上下文不会太长，很拽，对于各个问题基本都可以迎刃而解，遇到不知道的直接说我不知道，啥都能说。"//为ai设置一个系统提示，告诉它应该如何表现，越详细越好。
};

const aiChatState = {
    isInitialized: false,
    conversationHistory: [],
    isWaitingForResponse: false,
    lastMessageTime: 0,
    sessionKey: "ai:gemini",
    storageKey: "ai_chat_history_gemini"
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
                sender: "Gemini",
                target_type: "ai",
                content: "嗨，我是 Gemini！有什么需要帮助的吗？",
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
                <div class="flex-1 min-w-0">
                    <div class="text-xs font-semibold text-slate-700 truncate">Gemini 助手</div>
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
        id: 'gemini',
        name: 'Gemini 助手',
    };
    
    document.querySelectorAll('[data-chat-id]').forEach(el => {
        el.classList.remove('bg-blue-50', 'border-l-2', 'border-blue-500');
    });
    
    const aiItem = document.querySelector('[data-chat-id="ai-gemini"]');
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
    const isAI = msg.sender === 'Gemini' || msg.target_type === 'ai';
    
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
        const response = await callGeminiAPI(userMessage);
        
        if (response && response.trim()) {
            const aiMsg = {
                id: `msg_${Date.now()}`,
                sender: 'Gemini',
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
            sender: 'Gemini',
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

async function callGeminiAPI(userMessage) {
    try {
        const allMessages = cacheMessages[aiChatState.sessionKey] || [];
        const recentMessages = allMessages.slice(-3);
        
        const messageParts = [];
        
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

function clearAIChatHistory() {
    const confirmed = confirm('确定要清空所有 AI 聊天记录吗？');
    if (confirmed) {
        cacheMessages[aiChatState.sessionKey] = [
            {
                id: "ai_init_" + Date.now(),
                sender: "Gemini",
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
