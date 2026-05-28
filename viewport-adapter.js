/**
 * 视口适配脚本 - 专门处理手机折叠屏和浏览器UI的适配问题
 * 支持：
 * - 动态viewport高度（处理浏览器地址栏收缩）
 * - 折叠屏屏幕方向变化
 * - Safe area inset（notch、折叠区等）
 * - 弹出键盘时的自动调整
 */

class ViewportAdapter {
    constructor() {
        this.root = document.documentElement;
        this.lastInnerHeight = window.innerHeight;
        this.lastInnerWidth = window.innerWidth;
        this.resizeTimeout = null;
        this.isPhone = this.detectPhone();
        this.isFoldable = this.detectFoldableDevice();
        
        this.init();
    }

    /**
     * 检测是否为手机设备
     */
    detectPhone() {
        return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
    }

    /**
     * 检测是否为折叠屏设备（三星 Fold、Google Pixel Fold 等）
     */
    detectFoldableDevice() {
        const ua = navigator.userAgent.toLowerCase();
        return /fold|galaxy.*z|pixel fold/i.test(ua);
    }

    /**
     * 初始化适配
     */
    init() {
        this.updateCSSVariables();
        this.setupListeners();
        this.applyFoldableStyles();
        console.log(`[ViewportAdapter] 已初始化 | 手机: ${this.isPhone} | 折叠屏: ${this.isFoldable}`);
    }

    /**
     * 更新CSS变量，用于动态高度计算
     */
    updateCSSVariables() {
        // 使用 window.visualViewport 获取实际可用高度
        const viewportHeight = window.visualViewport 
            ? window.visualViewport.height 
            : window.innerHeight;
        
        const viewportWidth = window.visualViewport 
            ? window.visualViewport.width 
            : window.innerWidth;

        // 设置CSS变量供HTML使用
        this.root.style.setProperty('--viewport-height', `${viewportHeight}px`);
        this.root.style.setProperty('--viewport-width', `${viewportWidth}px`);
        this.root.style.setProperty('--safe-height', `${Math.min(viewportHeight, window.innerHeight)}px`);
        
        // safe area inset（用于处理notch和折叠区）
        const safeAreaTop = this.getCSSVariable('--safe-area-inset-top') || '0px';
        const safeAreaBottom = this.getCSSVariable('--safe-area-inset-bottom') || '0px';
        const safeAreaLeft = this.getCSSVariable('--safe-area-inset-left') || '0px';
        const safeAreaRight = this.getCSSVariable('--safe-area-inset-right') || '0px';

        this.root.style.setProperty('--usable-height', 
            `calc(${viewportHeight}px - ${safeAreaTop} - ${safeAreaBottom})`);
    }

    /**
     * 获取CSS变量值
     */
    getCSSVariable(varName) {
        return getComputedStyle(this.root).getPropertyValue(varName).trim();
    }

    /**
     * 为折叠屏设备应用特殊样式
     */
    applyFoldableStyles() {
        if (!this.isFoldable) return;

        const style = document.createElement('style');
        style.textContent = `
            /* 折叠屏特殊处理 */
            body {
                /* 禁用水平滚动 */
                overflow-x: hidden !important;
                /* 使用环境变量处理折叠线 */
                padding-top: max(0px, env(safe-area-inset-top));
                padding-bottom: max(0px, env(safe-area-inset-bottom));
                padding-left: max(0px, env(safe-area-inset-left));
                padding-right: max(0px, env(safe-area-inset-right));
            }

            #mainApp {
                /* 使用环境变量确保不被折叠线遮挡 */
                padding-top: max(0px, env(safe-area-inset-top));
                padding-bottom: max(0px, env(safe-area-inset-bottom));
            }

            /* 防止键盘弹起时输入框被遮挡 */
            #messageInput:focus,
            #usernameInput:focus,
            #passwordInput:focus,
            #newGroupName:focus {
                /* 自动滚动到视口中心 */
                scroll-behavior: smooth;
            }
        `;
        document.head.appendChild(style);
    }

    /**
     * 设置监听器
     */
    setupListeners() {
        // 监听 visualViewport 变化（浏览器UI变化时触发）
        if (window.visualViewport) {
            window.visualViewport.addEventListener('resize', () => this.handleViewportChange());
            window.visualViewport.addEventListener('scroll', () => this.handleViewportChange());
        }

        // 监听窗口变化
        window.addEventListener('resize', () => this.handleWindowResize());

        // 监听屏幕方向变化（折叠屏外屏→内屏）
        window.addEventListener('orientationchange', () => this.handleOrientationChange());

        // 监听屏幕尺寸查询变化
        if (window.matchMedia) {
            // 监听折叠线方向的变化
            const foldQuery = window.matchMedia('(fold-left: 0px)');
            foldQuery.addListener(() => this.handleFoldChange());
        }

        // 防止输入框在键盘弹起时被遮挡
        document.addEventListener('focusin', (e) => this.handleFocusIn(e));

        // Android特殊处理：监听软键盘
        if (this.isPhone && /Android/i.test(navigator.userAgent)) {
            this.setupAndroidKeyboardHandling();
        }
    }

    /**
     * Android特有的键盘处理
     */
    setupAndroidKeyboardHandling() {
        // 防止页面缩放（键盘弹起时）
        document.addEventListener('touchmove', (e) => {
            if (e.touches.length > 1) {
                e.preventDefault(); // 禁用双指放大
            }
        }, { passive: false });

        // 监听输入框内容变化时自动滚动
        const inputs = document.querySelectorAll('input, textarea');
        inputs.forEach(input => {
            input.addEventListener('focus', () => {
                // 延迟确保键盘完全弹起
                setTimeout(() => {
                    const rect = input.getBoundingClientRect();
                    const viewportHeight = window.visualViewport?.height || window.innerHeight;
                    
                    // 如果输入框在视口下半部分，则滚动
                    if (rect.bottom > viewportHeight * 0.8) {
                        window.scrollBy({
                            top: rect.bottom - viewportHeight + 50,
                            behavior: 'smooth'
                        });
                    }
                }, 350);
            });
        });
    }

    /**
     * 处理 visualViewport 变化（最常见的情况）
     */
    handleViewportChange() {
        this.updateCSSVariables();
    }

    /**
     * 处理窗口大小变化（带防抖）
     */
    handleWindowResize() {
        clearTimeout(this.resizeTimeout);
        this.resizeTimeout = setTimeout(() => {
            const newHeight = window.innerHeight;
            const newWidth = window.innerWidth;

            // 检测键盘是否弹起（高度显著变化）
            const heightDiff = Math.abs(newHeight - this.lastInnerHeight);
            const isKeyboardVisible = heightDiff > 100; // 超过100px认为是键盘

            this.lastInnerHeight = newHeight;
            this.lastInnerWidth = newWidth;

            this.updateCSSVariables();

            if (isKeyboardVisible) {
                this.handleKeyboardChange(newHeight < this.lastInnerHeight);
            }
        }, 150);
    }

    /**
     * 处理屏幕方向变化
     */
    handleOrientationChange() {
        // 延迟一点更新，等待系统完成方向变化
        setTimeout(() => {
            this.updateCSSVariables();
            console.log(`[ViewportAdapter] 方向改变: ${window.innerWidth}x${window.innerHeight}`);
        }, 300);
    }

    /**
     * 处理折叠线变化
     */
    handleFoldChange() {
        console.log('[ViewportAdapter] 检测到折叠线变化');
        this.updateCSSVariables();
        // 重新布局
        document.body.offsetHeight; // 强制重排
    }

    /**
     * 处理键盘变化
     */
    handleKeyboardChange(isKeyboardVisible) {
        const chatInput = document.getElementById('messageInput');
        if (!chatInput) return;

        if (isKeyboardVisible) {
            // 键盘弹起，自动滚动到输入框
            setTimeout(() => {
                chatInput.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }, 100);
        }
    }

    /**
     * 处理输入框获焦（防止被遮挡）
     */
    handleFocusIn(e) {
        const input = e.target;
        if (['INPUT', 'TEXTAREA'].includes(input.tagName)) {
            // 延迟滚动，等待键盘完全弹起
            setTimeout(() => {
                input.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }, 300);
        }
    }

    /**
     * 获取实际可用的视口高度
     */
    getUsableHeight() {
        return window.visualViewport?.height || window.innerHeight;
    }

    /**
     * 获取实际可用的视口宽度
     */
    getUsableWidth() {
        return window.visualViewport?.width || window.innerWidth;
    }

    /**
     * 检查设备是否在折叠状态
     */
    isFolded() {
        if (!this.isFoldable) return false;
        // Fold状态下外屏宽度通常小于内屏
        // 这需要根据实际设备调整
        return window.innerWidth < 600; // 示例值
    }

    /**
     * 锁定竖屏方向（某些应用需要）
     */
    lockPortrait() {
        if (screen.orientation && screen.orientation.lock) {
            screen.orientation.lock('portrait').catch(err => {
                console.warn('[ViewportAdapter] 无法锁定竖屏:', err);
            });
        }
    }

    /**
     * 解锁屏幕方向
     */
    unlockOrientation() {
        if (screen.orientation && screen.orientation.unlock) {
            screen.orientation.unlock();
        }
    }
}

// 页面加载时初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.viewportAdapter = new ViewportAdapter();
    });
} else {
    window.viewportAdapter = new ViewportAdapter();
}
