/**
 * 消息提醒管理模块
 * 提供：
 * 1. 浏览器桌面通知
 * 2. 侧边栏徽章和未读计数
 * 3. 消息提示音（可选）
 * 4. 浏览器标题动态提醒
 */

const NotificationManager = (() => {
    // 配置
    const CONFIG = {
        enableDesktopNotification: true,
        enableSound: true,
        enableBadge: true,
        enableTitleBlink: true,
        soundUrl: './notification.mp3', // 可选：替换为实际的提示音文件路径
    };

    // 状态
    const state = {
        unreadCount: {}, // { "type:id": count }
        totalUnread: 0,
        originalTitle: document.title,
        titleBlinkInterval: null,
        isTitleBlinking: false,
    };

    // 初始化通知权限
    async function requestNotificationPermission() {
        if (!CONFIG.enableDesktopNotification) return;
        
        if ('Notification' in window) {
            if (Notification.permission === 'granted') {
                console.log('✓ 浏览器通知已启用');
                return true;
            } else if (Notification.permission !== 'denied') {
                try {
                    const permission = await Notification.requestPermission();
                    if (permission === 'granted') {
                        console.log('✓ 浏览器通知权限已授予');
                        return true;
                    }
                } catch (err) {
                    console.warn('获取通知权限失败:', err);
                }
            }
        }
        return false;
    }

    // 播放提示音
    function playNotificationSound() {
        if (!CONFIG.enableSound) return;
        
        try {
            // 使用Web Audio API生成简单的蜂鸣音
            const audioContext = new (window.AudioContext || window.webkitAudioContext)();
            const oscillator = audioContext.createOscillator();
            const gainNode = audioContext.createGain();
            
            oscillator.connect(gainNode);
            gainNode.connect(audioContext.destination);
            
            // 设置声音参数
            oscillator.frequency.value = 800; // 频率
            oscillator.type = 'sine';
            
            gainNode.gain.setValueAtTime(0.3, audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.2);
            
            oscillator.start(audioContext.currentTime);
            oscillator.stop(audioContext.currentTime + 0.2);
        } catch (err) {
            console.warn('播放提示音失败:', err);
        }
    }

    // 显示桌面通知
    function showDesktopNotification(title, options = {}) {
        if (!CONFIG.enableDesktopNotification || Notification.permission !== 'granted') {
            return;
        }

        try {
            const defaultOptions = {
                icon: '/favicon.ico',
                badge: '/favicon.ico',
                tag: 'message-notification',
                requireInteraction: false,
                ...options
            };

            const notification = new Notification(title, defaultOptions);

            // 点击通知时的处理
            notification.onclick = () => {
                notification.close();
                // 可以在这里添加点击通知后的操作
                if (options.targetChat) {
                    const { type, id, name } = options.targetChat;
                    selectActiveChat(type, id, name);
                    loadHistoryMessages(type, id);
                }
                window.focus();
            };

            // 自动关闭（4秒）
            setTimeout(() => notification.close(), 4000);
        } catch (err) {
            console.warn('显示桌面通知失败:', err);
        }
    }

    // 更新标题显示未读数（不闪烁）
    function updateTitleWithUnreadCount() {
        if (!CONFIG.enableTitleBlink) {
            document.title = state.originalTitle;
            return;
        }

        if (state.totalUnread > 0) {
            document.title = `(${state.totalUnread}) ${state.originalTitle}`;
        } else {
            document.title = state.originalTitle;
        }
    }

    // 停止标题闪烁（已废弃，保留兼容性）
    function stopTitleBlink() {
        if (state.titleBlinkInterval) {
            clearInterval(state.titleBlinkInterval);
            state.titleBlinkInterval = null;
        }
        state.isTitleBlinking = false;
        updateTitleWithUnreadCount();
    }

    // 获取未读计数
    function getUnreadCount(type, id) {
        const key = `${type}:${id}`;
        return state.unreadCount[key] || 0;
    }

    // 增加未读计数
    function incrementUnreadCount(type, id) {
        const key = `${type}:${id}`;
        state.unreadCount[key] = (state.unreadCount[key] || 0) + 1;
        state.totalUnread++;
        updateBadges();
    }

    // 重置未读计数
    function resetUnreadCount(type, id) {
        const key = `${type}:${id}`;
        if (state.unreadCount[key]) {
            state.totalUnread -= state.unreadCount[key];
            delete state.unreadCount[key];
        }
        updateBadges();
    }

    // 清空所有未读
    function clearAllUnread() {
        state.unreadCount = {};
        state.totalUnread = 0;
        stopTitleBlink();
        updateBadges();
    }

    // 更新徽章显示
    function updateBadges() {
        if (!CONFIG.enableBadge) return;

        // 更新侧边栏项目的未读徽章
        document.querySelectorAll('.sidebar-item').forEach(item => {
            const badge = item.querySelector('.unread-badge');
            if (badge) badge.remove();
        });

        // 为有未读消息的项目添加徽章
        for (const [key, count] of Object.entries(state.unreadCount)) {
            const [type, id] = key.split(':');
            const selector = `[data-chat-type="${type}"][data-chat-id="${id}"]`;
            const item = document.querySelector(selector);
            
            if (item && count > 0) {
                const badge = document.createElement('span');
                badge.className = 'unread-badge';
                badge.innerText = count > 99 ? '99+' : count;
                item.appendChild(badge);
            }
        }

        // 更新浏览器标签页徽章（如果支持）
        if ('setAppBadge' in navigator && state.totalUnread > 0) {
            try {
                navigator.setAppBadge(state.totalUnread);
            } catch (err) {
                console.warn('设置应用徽章失败:', err);
            }
        } else if ('clearAppBadge' in navigator && state.totalUnread === 0) {
            try {
                navigator.clearAppBadge();
            } catch (err) {
                console.warn('清除应用徽章失败:', err);
            }
        }

        // 根据未读数更新标题（显示计数而非闪烁）
        updateTitleWithUnreadCount();
    }

    // 处理新消息（集中入口）
    function handleNewMessage(msg, isUnread = true) {
        const { sender, target_type: type, target_id: id } = msg;

        // 如果是当前用户的消息或当前窗口是该聊天，不提醒
        if (sender === currentUser) return;

        const isCurrent = activeTarget && 
                         activeTarget.type === type && 
                         activeTarget.id === id;

        if (isCurrent && document.hasFocus()) {
            // 当前窗口已打开且浏览器获得焦点，不提醒
            return;
        }

        // 播放提示音
        playNotificationSound();

        // 增加未读计数
        if (isUnread) {
            incrementUnreadCount(type, id);
        }

        // 构建通知内容
        let senderDisplay = sender;
        let targetName = id;

        if (type === 'group') {
            const group = globalGroups?.find(g => g.id.toString() === id);
            targetName = group?.name || `群组 ${id}`;
        } else if (type === 'private') {
            targetName = sender;
        }

        // 显示桌面通知
        const notificationText = msg.content && msg.content.startsWith('/uploads/') 
            ? '[图片]' 
            : (msg.content || '[空消息]').substring(0, 50);

        const notificationTitle = type === 'public' 
            ? `${sender}: ${notificationText}`
            : `${senderDisplay} (${targetName}): ${notificationText}`;

        showDesktopNotification(notificationTitle, {
            body: `来自 ${targetName}`,
            tag: `msg-${type}-${id}`,
            targetChat: { type, id, name: targetName }
        });
    }

    // 当用户打开聊天窗口时重置未读计数
    function onChatWindowOpened(type, id) {
        resetUnreadCount(type, id);
    }

    // 保存提醒设置到本地存储
    function saveSettings(settings) {
        const merged = { ...CONFIG, ...settings };
        localStorage.setItem('notification_settings', JSON.stringify(merged));
        Object.assign(CONFIG, merged);
        console.log('✓ 提醒设置已保存');
    }

    // 从本地存储加载提醒设置
    function loadSettings() {
        try {
            const saved = localStorage.getItem('notification_settings');
            if (saved) {
                const settings = JSON.parse(saved);
                Object.assign(CONFIG, settings);
                console.log('✓ 提醒设置已加载');
            }
        } catch (err) {
            console.warn('加载提醒设置失败:', err);
        }
    }

    // 获取当前配置
    function getConfig() {
        return { ...CONFIG };
    }

    // 窗口获得焦点时清空未读计数
    window.addEventListener('focus', () => {
        if (activeTarget) {
            resetUnreadCount(activeTarget.type, activeTarget.id);
        }
    });

    // 导出公共接口
    return {
        init: requestNotificationPermission,
        handleNewMessage,
        onChatWindowOpened,
        requestNotificationPermission,
        showDesktopNotification,
        playNotificationSound,
        incrementUnreadCount,
        resetUnreadCount,
        clearAllUnread,
        getUnreadCount,
        saveSettings,
        loadSettings,
        getConfig,
        stopTitleBlink,
    };
})();

// 初始化通知系统
document.addEventListener('DOMContentLoaded', () => {
    NotificationManager.loadSettings();
    NotificationManager.init();
});
