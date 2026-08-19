class AIChatController {
    constructor(config = {}) {
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

        this.state = {
            isSending: false,
            defaultSendText: '发送'
        };

        this.elements = {};
        
        this.abortController = null;
        
        this.handleInputKeydown = this.handleInputKeydown.bind(this);
        this.handleModalClick = this.handleModalClick.bind(this);
        this.handleDocumentKeydown = this.handleDocumentKeydown.bind(this);
        this.sendAIMessage = this.sendAIMessage.bind(this);
        this.closeAIChat = this.closeAIChat.bind(this);
        this.openAIChat = this.openAIChat.bind(this);
    }

    init() {
        this.cacheDOM();
        if (!this.elements.modal) {
            console.warn('[AIChat] 核心 DOM 节点未找到，初始化中止。');
            return;
        }
        
        if (this.elements.sendButton) {
            this.state.defaultSendText = this.elements.sendButton.textContent || '发送';
        }

        this.updateSidebarExtensions();
        this.attachEventListeners();
    }

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
            existingBtn.removeEventListener('click', this.openAIChat);
            existingBtn.remove();
        }
    }

    openAIChat() {
        if (window.openModalWithAnimation) {
            window.openModalWithAnimation(this.elements.modal);
        } else {
            this.elements.modal.classList.remove('hidden');
        }
        setTimeout(() => this.elements.input?.focus(), 50);
    }

    closeAIChat() {
        if (window.closeModalWithAnimation) {
            window.closeModalWithAnimation(this.elements.modal);
        } else {
            this.elements.modal.classList.add('hidden');
        }
        if (this.state.isSending && this.abortController) {
            this.abortController.abort();
            this.abortController = null;
        }
    }

    setSendingState(isSending) {
        this.state.isSending = isSending;
        const { input, sendButton } = this.elements;
        
        if (input) input.disabled = isSending;
        if (sendButton) {
            sendButton.disabled = isSending;
            sendButton.textContent = isSending ? '发送中...' : this.state.defaultSendText;
            // 样式调整：发送中可降低不透明度
            if (isSending) {
                sendButton.classList.add('opacity-50', 'cursor-not-allowed');
            } else {
                sendButton.classList.remove('opacity-50', 'cursor-not-allowed');
            }
        }
    }

    appendChatMessage(text, type = 'user') {
        const win = this.elements.chatWindow;
        if (!win) return null;

        const styles = {
            user: 'text-sm text-slate-800 mb-2 font-medium',
            ai: 'text-sm text-slate-700 mb-3 whitespace-pre-wrap', 
            error: 'text-sm text-rose-500 mb-2 font-bold'
        };

        const msgDiv = document.createElement('div');
        msgDiv.className = styles[type] || styles.ai;
        msgDiv.textContent = text; 

        win.appendChild(msgDiv);
        win.scrollTop = win.scrollHeight;
        
        return msgDiv;
    }

    createEmptyAIMessage() {
        return this.appendChatMessage('AI：', 'ai');
    }

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

    async sendAIMessage() {
        if (this.state.isSending) return;

        const prompt = this.elements.input?.value.trim();
        if (!prompt) return;

        this.appendChatMessage(`你：${prompt}`, 'user');
        this.elements.input.value = '';
        this.setSendingState(true);

        this.abortController = new AbortController();

        const aiMessageNode = this.createEmptyAIMessage();
        let aiResponseText = '';

        try {
            const response = await fetch(this.config.apiEndpoint, {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Accept': 'text/event-stream' 
                },
                body: JSON.stringify({ prompt, stream: true }),
                signal: this.abortController.signal
            });

            if (!response.ok) {
                const errorMessage = await this.extractErrorMessage(response);
                if (aiMessageNode) {
                    aiMessageNode.className = 'text-sm text-rose-500 mb-2 font-bold';
                    aiMessageNode.textContent = errorMessage;
                }
                console.error('[AIChat] 请求失败:', errorMessage);
                return;
            }

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

            const reader = response.body.getReader();
            const decoder = new TextDecoder('utf-8');
            let done = false;
            let buffer = '';

            while (!done) {
                const { value, done: readerDone } = await reader.read();
                done = readerDone;

                if (value) {
                    buffer += decoder.decode(value, { stream: true });
                    const lines = buffer.split('\n');
                    buffer = lines.pop(); 

                    for (const line of lines) {
                        const trimmedLine = line.trim();
                        if (!trimmedLine) continue;

                        if (trimmedLine.startsWith('data: ')) {
                            const dataStr = trimmedLine.slice(6).trim();
                            
                            if (dataStr === '[DONE]') {
                                done = true;
                                break; 
                            }

                            try {
                                const dataObj = JSON.parse(dataStr);
                                const delta = dataObj.delta || dataObj.content || dataObj.text || '';
                                
                                if (delta) {
                                    aiResponseText += delta;
                                    if (aiMessageNode) {
                                        aiMessageNode.textContent = `AI：${aiResponseText}`;
                                    }
                                    if (this.elements.chatWindow) {
                                        this.elements.chatWindow.scrollTop = this.elements.chatWindow.scrollHeight;
                                    }
                                }
                            } catch (e) {
                            }
                        }
                    }
                }
            }
        } catch (error) {
            if (error.name === 'AbortError') {
                if (aiMessageNode && !aiResponseText) {
                    aiMessageNode.textContent = 'AI：(请求已取消)';
                } else if (aiMessageNode) {
                    aiMessageNode.textContent += '\n[已停止生成]';
                }
                return;
            }

            if (aiMessageNode) {
                aiMessageNode.className = 'text-sm text-rose-500 mb-2 font-bold';
                aiMessageNode.textContent += '\n(网络连接中断或请求失败)';
            }
            console.error('[AIChat] 流式消息发送失败：', error);
        } finally {
            this.setSendingState(false);
            this.abortController = null;
            

            if (!this.elements.modal?.classList.contains('hidden')) {
                this.elements.input?.focus();
            }
        }
    }

    handleInputKeydown(event) {
        if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
            event.preventDefault();
            this.sendAIMessage();
        }
    }

    handleModalClick(event) {
        if (event.target === event.currentTarget) {
            this.closeAIChat();
        }
    }

    handleDocumentKeydown(event) {
        if (event.key === 'Escape' && !this.elements.modal.classList.contains('hidden')) {
            this.closeAIChat();
        }
    }

    attachEventListeners() {
        const { modal, sendButton, closeButton, input } = this.elements;

        modal?.addEventListener('click', this.handleModalClick);
        sendButton?.addEventListener('click', this.sendAIMessage);
        closeButton?.addEventListener('click', this.closeAIChat);
        input?.addEventListener('keydown', this.handleInputKeydown);
        document.addEventListener('keydown', this.handleDocumentKeydown);
    }

    destroy() {
        if (this.abortController) {
            this.abortController.abort();
            this.abortController = null;
        }

        const { modal, sendButton, closeButton, input } = this.elements;

        modal?.removeEventListener('click', this.handleModalClick);
        sendButton?.removeEventListener('click', this.sendAIMessage);
        closeButton?.removeEventListener('click', this.closeAIChat);
        input?.removeEventListener('keydown', this.handleInputKeydown);
        document.removeEventListener('keydown', this.handleDocumentKeydown);
        
        const existingBtn = document.getElementById('btnAICHat');
        if (existingBtn) {
            existingBtn.removeEventListener('click', this.openAIChat);
            existingBtn.remove();
        }
    }
}

window.AIChat = {
    updateSidebarExtensions() {
        if (window.aiChatInstance?.updateSidebarExtensions) {
            window.aiChatInstance.updateSidebarExtensions();
        }
    }
};

document.addEventListener('DOMContentLoaded', () => {
    window.aiChatInstance = new AIChatController({
        apiEndpoint: '/api/ai/chat' // 可在外部动态修改
    });
    window.aiChatInstance.init();
    window.AIChat = window.aiChatInstance;
});