(function () {
    if (window.__chatboxCustomSampleLoaded) return;
    window.__chatboxCustomSampleLoaded = true;

    const style = document.createElement('style');
    style.type = 'text/css';
    style.textContent = `
        #sampleCustomExtensionStatus {
            position: fixed;
            right: 16px;
            bottom: 76px;
            z-index: 9999;
            padding: 8px 12px;
            border-radius: 999px;
            color: #0f172a;
            background: #e0f2fe;
            border: 1px solid #bae6fd;
            font-size: 12px;
            box-shadow: 0 8px 30px rgba(14, 165, 233, 0.12);
        }
    `;
    document.head.appendChild(style);

    const badge = document.createElement('div');
    badge.id = 'sampleCustomExtensionStatus';
    badge.textContent = '自定义扩展已加载';
    document.body.appendChild(badge);

    console.log('[Chatbox Custom Extension] sample-extension.js loaded');
})();
