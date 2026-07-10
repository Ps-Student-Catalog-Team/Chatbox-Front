(function () {
    const API_ENDPOINT = '/api/ai/chat';
    const SELECTORS = {
        sidebarExtensions: '#sidebarExtensionsContainer',
        modal: '#aiChatModal',
        chatWindow: '#aiChatWindow',
        input: '#aiPromptInput',
        sendButton: '#aiChatSendButton',
        closeButton: '#aiChatCloseButton'
    };
    const DEFAULT_EXTENSIONS_STATE = { ai_chat: true };
    let isSending = false;
    let defaultSendButtonText = '发送';

    function getExtensionsStateSafe() {
        if (typeof getExtensionsState === 'function') {
            return getExtensionsState() || DEFAULT_EXTENSIONS_STATE;
        }
        return DEFAULT_EXTENSIONS_STATE;
    }

    function getElement(selector) {
        return document.querySelector(selector);
    }

    function updateSidebarExtensions() {
        const st = getExtensionsStateSafe();
        const sidebarButtonsContainer = getElement(SELECTORS.sidebarExtensions);
        if (!sidebarButtonsContainer) return;

        const existing = document.getElementById('btnAICHat');
        if (st.ai_chat) {
            if (!existing) {
                const btn = document.createElement('button');
                btn.id = 'btnAICHat';
                btn.className = 'p-3 rounded-xl transition cursor-pointer hover:bg-slate-800 hover:text-white';
                btn.title = 'AI 聊天';
                btn.innerText = '🤖';
                btn.addEventListener('click', openAIChat);
                sidebarButtonsContainer.appendChild(btn);
            }
        } else if (existing) {
            existing.remove();
        }
    }

    function openAIChat() {
        const modal = getElement(SELECTORS.modal);
        if (!modal) return;
        modal.classList.remove('hidden');

        const input = getElement(SELECTORS.input);
        if (input) input.focus();
    }

    function closeAIChat() {
        const modal = getElement(SELECTORS.modal);
        if (!modal) return;
        modal.classList.add('hidden');
    }

    function setSendingState(sending) {
        const input = getElement(SELECTORS.input);
        const sendButton = getElement(SELECTORS.sendButton);
        if (input) input.disabled = sending;
        if (sendButton) {
            sendButton.disabled = sending;
            sendButton.textContent = sending ? '发送中...' : defaultSendButtonText;
        }
        isSending = sending;
    }

    function appendChatMessage(text, className) {
        const win = getElement(SELECTORS.chatWindow);
        if (!win) return;
        const msgDiv = document.createElement('div');
        msgDiv.className = className;
        msgDiv.innerText = text;
        win.appendChild(msgDiv);
        scrollChatWindow();
    }

    function scrollChatWindow() {
        const win = getElement(SELECTORS.chatWindow);
        if (!win) return;
        win.scrollTop = win.scrollHeight;
    }

    async function extractErrorMessage(response) {
        if (response.ok) return null;
        let body = null;
        try {
            body = await response.json();
        } catch (e) {
            // ignore invalid JSON body
        }

        const detail = body?.error || body?.message || '';
        console.error('[AIChat] AI 请求失败：', response.status, response.statusText, detail);

        if (response.status === 401) return 'AI 请求未经授权，请重新登录或检查权限。';
        if (response.status === 403) return 'AI 请求被拒绝访问。';
        if (response.status === 404) return 'AI 服务未找到。';
        if (response.status >= 500) return 'AI 服务内部错误，请稍后重试。';
        return detail ? `AI 错误：${detail}` : 'AI 服务请求失败，请稍后重试。';
    }

    async function sendAIMessage() {
        if (isSending) return;

        const input = getElement(SELECTORS.input);
        if (!input) return;

        const prompt = input.value.trim();
        if (!prompt) return;

        appendChatMessage('你：' + prompt, 'text-sm text-slate-800 mb-2');
        input.value = '';
        setSendingState(true);

        try {
            const response = await fetch(API_ENDPOINT, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ prompt })
            });

            const errorMessage = await extractErrorMessage(response);
            if (errorMessage) {
                appendChatMessage(errorMessage, 'text-sm text-rose-500');
                return;
            }

            const data = await response.json().catch(() => ({}));
            const reply = data.reply || data.content || data.answer;
            if (!reply) {
                appendChatMessage('AI 未返回有效内容。', 'text-sm text-rose-500');
                console.error('[AIChat] 未识别 AI 响应结构：', data);
                return;
            }

            appendChatMessage('AI：' + reply, 'text-sm text-slate-700 mb-3');
        } catch (error) {
            appendChatMessage('调用 AI 接口失败，请检查网络或稍后重试。', 'text-sm text-rose-500');
            console.error('[AIChat] 发送消息失败：', error);
        } finally {
            setSendingState(false);
        }
    }

    function handleInputKeydown(event) {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            sendAIMessage();
        }
    }

    function handleModalClick(event) {
        if (event.target === event.currentTarget) {
            closeAIChat();
        }
    }

    function handleDocumentKeydown(event) {
        if (event.key === 'Escape') {
            closeAIChat();
        }
    }

    function attachEventListeners() {
        const modal = getElement(SELECTORS.modal);
        const sendButton = getElement(SELECTORS.sendButton);
        const closeButton = getElement(SELECTORS.closeButton);
        const input = getElement(SELECTORS.input);

        if (modal) {
            modal.addEventListener('click', handleModalClick);
        }
        if (sendButton) {
            defaultSendButtonText = sendButton.textContent || defaultSendButtonText;
            sendButton.addEventListener('click', sendAIMessage);
        }
        if (closeButton) {
            closeButton.addEventListener('click', closeAIChat);
        }
        if (input) {
            input.addEventListener('keydown', handleInputKeydown);
        }
        document.addEventListener('keydown', handleDocumentKeydown);
    }

    function init() {
        attachEventListeners();
        updateSidebarExtensions();
    }

    function ready(callback) {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', callback);
        } else {
            callback();
        }
    }

    window.AIChat = {
        init,
        updateSidebarExtensions,
        openAIChat,
        closeAIChat,
        sendAIMessage
    };

    ready(init);
})();
