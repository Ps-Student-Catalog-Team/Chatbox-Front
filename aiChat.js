class AIChatController {
    constructor(config = {}) {
        // 配置项解耦
        this.config = {
            apiEndpoint: config.apiEndpoint || '/api/ai/chat',
            defaultExtensionsState: { ai_chat: true },
            selectors: {
                sidebarExtensions: '#sidebarExtensionsContainer',
                modal: '#aiChatModal',
                chatWindow: '#aiChatWindow',
                input: '#aiPromptInput',
                sendButton: '#aiChatSendButton',
                closeButton: '#aiChatCloseButton',
                ...config.selectors
            }
        };

        // 状态管理中心化
        this.state = {
            isSending: false,
            defaultSendText: '发送'
        };

        // DOM 节点缓存字典
        this.elements = {};
        
        // 绑定 this 上下文，确保事件监听器可以正确调用类方法
        this.handleInputKeydown = this.handleInputKeydown.bind(this);
        this.handleModalClick = this.handleModalClick.bind(this);
        this.handleDocumentKeydown = this.handleDocumentKeydown.bind(this);
        this.sendAIMessage = this.sendAIMessage.bind(this);
        this.closeAIChat = this.closeAIChat.bind(this);
        this.openAIChat = this.openAIChat.bind(this);
    }

    // 初始化入口
    init() {
        this.cacheDOM();
        if (!this.elements.modal) {
            console.warn('[AIChat] 核心 DOM 节点未找到，初始化中止。');
            return;
        }
        
        // 记录按钮默认文本
        if (this.elements.sendButton) {
            this.state.defaultSendText = this.elements.sendButton.textContent || '发送';
        }

        this.updateSidebarExtensions();
        this.attachEventListeners();
    }

    // 统一缓存 DOM，避免重复查询
    cacheDOM() {
        const { selectors } = this.config;
        for (const [key, selector] of Object.entries(selectors)) {
            this.elements[key] = document.querySelector(selector);
        }
    }

    getExtensionsStateSafe() {
        return typeof window.getExtensionsState === 'function' 
            ? (window.getExtensionsState() || this.config.defaultExtensionsState) 
            : this.config.defaultExtensionsState;
    }

    // 更新侧边栏入口按钮
    updateSidebarExtensions() {
        const state = this.getExtensionsStateSafe();
        const container = this.elements.sidebarExtensions;
        if (!container) return;

        const existingBtn = document.getElementById('btnAICHat');

        if (state.ai_chat && !existingBtn) {
            const btn = document.createElement('button');
            btn.id = 'btnAICHat';
            btn.className = 'p-3 rounded-xl transition cursor-pointer hover:bg-slate-800 hover:text-white';
            btn.title = 'AI 聊天';
            btn.innerText = '🤖';
            btn.addEventListener('click', this.openAIChat);
            container.appendChild(btn);
        } else if (!state.ai_chat && existingBtn) {
            existingBtn.remove();
        }
    }

    openAIChat() {
        this.elements.modal.classList.remove('hidden');
        this.elements.input?.focus();
    }

    closeAIChat() {
        this.elements.modal.classList.add('hidden');
    }

    setSendingState(isSending) {
        this.state.isSending = isSending;
        const { input, sendButton } = this.elements;
        
        if (input) input.disabled = isSending;
        if (sendButton) {
            sendButton.disabled = isSending;
            sendButton.textContent = isSending ? '发送中...' : this.state.defaultSendText;
        }
    }

    // 渲染消息气泡，并返回该 DOM 节点以便流式追加内容
    appendChatMessage(text, type = 'user') {
        const win = this.elements.chatWindow;
        if (!win) return null;

        // 样式字典化
        const styles = {
            user: 'text-sm text-slate-800 mb-2 font-medium',
            ai: 'text-sm text-slate-700 mb-3 whitespace-pre-wrap', // 增加 whitespace-pre-wrap 以支持换行
            error: 'text-sm text-rose-500 mb-2 font-bold'
        };

        const msgDiv = document.createElement('div');
        msgDiv.className = styles[type] || styles.ai;
        msgDiv.textContent = text; 

        win.appendChild(msgDiv);
        win.scrollTop = win.scrollHeight;
        
        return msgDiv;
    }

    // 创建一个空的 AI 消息气泡，为流式输出占位
    createEmptyAIMessage() {
        return this.appendChatMessage('AI：', 'ai');
    }

    // 提取 HTTP 非 200 状态下的错误信息
    async extractErrorMessage(response) {
        if (response.ok) return null;
        
        let detail = '';
        try {
            const body = await response.json();
            detail = body?.error || body?.message || '';
        } catch (e) {
            console.warn('[AIChat] 无法解析错误响应体为 JSON');
        }

        const statusMap = {
            401: 'AI 请求未经授权，请重新登录或检查权限。',
            403: 'AI 请求被拒绝访问。',
            404: 'AI 服务未找到。'
        };

        if (response.status >= 500) return 'AI 服务内部错误，请稍后重试。';
        if (statusMap[response.status]) return statusMap[response.status];
        
        return detail ? `AI 错误：${detail}` : 'AI 服务请求失败，请稍后重试。';
    }

    // 核心流式发送逻辑
    async sendAIMessage() {
        if (this.state.isSending) return;

        const prompt = this.elements.input?.value.trim();
        if (!prompt) return;

        // 1. 渲染用户消息，清空输入框并锁定发送状态
        this.appendChatMessage(`你：${prompt}`, 'user');
        this.elements.input.value = '';
        this.setSendingState(true);

        // 2. 预先创建一个空的 AI 消息气泡
        const aiMessageNode = this.createEmptyAIMessage();
        let aiResponseText = '';

        try {
            const response = await fetch(this.config.apiEndpoint, {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Accept': 'text/event-stream' // 声明接收流式数据
                },
                body: JSON.stringify({ prompt, stream: true }) 
            });

            // 异常处理
            if (!response.ok) {
                const errorMessage = await this.extractErrorMessage(response);
                if (aiMessageNode) {
                    aiMessageNode.className = 'text-sm text-rose-500 mb-2 font-bold';
                    aiMessageNode.textContent = errorMessage;
                }
                return;
            }

            // 如果后端没有采用 SSE/流式返回（例如返回 JSON），做回退处理
            const ct = (response.headers.get('Content-Type') || '').toLowerCase();
            if (!ct.includes('text/event-stream') && ct.includes('application/json')) {
                try {
                    const j = await response.json();
                    const reply = j.reply || j.text || j.result || '';
                    if (aiMessageNode) aiMessageNode.textContent = `AI：${reply}`;
                } catch (e) {
                    if (aiMessageNode) aiMessageNode.textContent = 'AI：(无法解析非流响应)';
                }
                return;
            }

            // 3. 读取流数据
            const reader = response.body.getReader();
            const decoder = new TextDecoder('utf-8');
            let done = false;
            let buffer = '';

            // 4. 循环解析 SSE 格式流
            while (!done) {
                const { value, done: readerDone } = await reader.read();
                done = readerDone;

                if (value) {
                    buffer += decoder.decode(value, { stream: true });
                    const lines = buffer.split('\n');
                    buffer = lines.pop(); // 保留最后一行未完整的数据

                    for (const line of lines) {
                        const trimmedLine = line.trim();
                        if (!trimmedLine) continue;

                        if (trimmedLine.startsWith('data: ')) {
                            const dataStr = trimmedLine.slice(6);
                            
                            if (dataStr === '[DONE]') break; 

                            try {
                                const dataObj = JSON.parse(dataStr);
                                // 注意：这里的 delta/content 取决于你后端的字段设定
                                const delta = dataObj.delta || dataObj.content || dataObj.text || '';
                                
                                if (delta) {
                                    aiResponseText += delta;
                                    // 实时更新气泡文字并滚动到底部
                                    if (aiMessageNode) {
                                        aiMessageNode.textContent = `AI：${aiResponseText}`;
                                    }
                                    if (this.elements.chatWindow) {
                                        this.elements.chatWindow.scrollTop = this.elements.chatWindow.scrollHeight;
                                    }
                                }
                            } catch (e) {
                                console.warn('[AIChat] 解析流式 JSON 失败:', dataStr);
                            }
                        }
                    }
                }
            }
        } catch (error) {
            if (aiMessageNode) {
                aiMessageNode.className = 'text-sm text-rose-500 mb-2 font-bold';
                aiMessageNode.textContent += '\n(网络连接中断或请求失败)';
            }
            console.error('[AIChat] 流式消息发送失败：', error);
        } finally {
            this.setSendingState(false);
            this.elements.input?.focus();
        }
    }

    // 键盘事件处理
    handleInputKeydown(event) {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            this.sendAIMessage();
        }
    }

    // 点击遮罩层关闭模态框
    handleModalClick(event) {
        if (event.target === event.currentTarget) {
            this.closeAIChat();
        }
    }

    // ESC 快捷键关闭
    handleDocumentKeydown(event) {
        if (event.key === 'Escape' && !this.elements.modal.classList.contains('hidden')) {
            this.closeAIChat();
        }
    }

    // 挂载所有事件监听
    attachEventListeners() {
        const { modal, sendButton, closeButton, input } = this.elements;

        modal?.addEventListener('click', this.handleModalClick);
        sendButton?.addEventListener('click', this.sendAIMessage);
        closeButton?.addEventListener('click', this.closeAIChat);
        input?.addEventListener('keydown', this.handleInputKeydown);
        document.addEventListener('keydown', this.handleDocumentKeydown);
    }

    // 卸载机制 (供 SPA 页面切换时清理内存使用)
    destroy() {
        const { modal, sendButton, closeButton, input } = this.elements;

        modal?.removeEventListener('click', this.handleModalClick);
        sendButton?.removeEventListener('click', this.sendAIMessage);
        closeButton?.removeEventListener('click', this.closeAIChat);
        input?.removeEventListener('keydown', this.handleInputKeydown);
        document.removeEventListener('keydown', this.handleDocumentKeydown);
        
        const existingBtn = document.getElementById('btnAICHat');
        if (existingBtn) existingBtn.remove();
    }
}

// 启动代码
document.addEventListener('DOMContentLoaded', () => {
    // 实例化并暴露给全局
    window.aiChatInstance = new AIChatController({
        apiEndpoint: '/api/ai/chat' // 可在外部动态修改
    });
    window.aiChatInstance.init();
});