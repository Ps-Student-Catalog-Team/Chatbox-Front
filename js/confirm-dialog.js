(() => {
    let dialog = null;
    let resolveDialog = null;

    const dialogTypes = {
        delete: { icon: '🗑️', iconClass: 'bg-red-50', buttonClass: 'bg-red-500 hover:bg-red-600' },
        account: { icon: '👤', iconClass: 'bg-blue-50', buttonClass: 'bg-blue-600 hover:bg-blue-700' },
        exit: { icon: '🚪', iconClass: 'bg-amber-50', buttonClass: 'bg-amber-500 hover:bg-amber-600' },
        clear: { icon: '🧹', iconClass: 'bg-orange-50', buttonClass: 'bg-orange-500 hover:bg-orange-600' },
        default: { icon: '❓', iconClass: 'bg-slate-100', buttonClass: 'bg-slate-700 hover:bg-slate-800' }
    };

    function ensureDialog() {
        if (dialog) return dialog;
        dialog = document.createElement('div');
        dialog.className = 'confirm-dialog fixed inset-0 z-[100] hidden items-center justify-center bg-slate-900/35 p-4 backdrop-blur-sm';
        dialog.innerHTML = `
            <div role="dialog" aria-modal="true" class="confirm-dialog-panel w-full max-w-md rounded-[28px] bg-white p-7 text-center shadow-2xl">
                <div data-confirm-icon class="confirm-dialog-icon mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-red-50 text-4xl">🗑️</div>
                <h2 data-confirm-title class="text-2xl font-bold text-slate-800">确认操作？</h2>
                <p data-confirm-message class="mt-3 whitespace-pre-line text-base text-slate-500"></p>
                <div class="mt-7 grid grid-cols-2 gap-4">
                    <button type="button" data-confirm-cancel class="rounded-2xl bg-slate-200 px-4 py-3 text-base font-medium text-slate-700 transition hover:bg-slate-300">取消</button>
                    <button type="button" data-confirm-ok class="rounded-2xl bg-red-500 px-4 py-3 text-base font-semibold text-white transition hover:bg-red-600">确定</button>
                </div>
            </div>`;
        document.body.appendChild(dialog);
        const close = result => {
            dialog.classList.remove('confirm-dialog-visible');
            dialog.classList.add('confirm-dialog-closing');
            setTimeout(() => {
                dialog.classList.add('hidden');
                dialog.classList.remove('flex', 'confirm-dialog-closing');
            }, 180);
            const resolve = resolveDialog;
            resolveDialog = null;
            if (resolve) resolve(result);
        };
        dialog.querySelector('[data-confirm-cancel]').onclick = () => close(false);
        dialog.querySelector('[data-confirm-ok]').onclick = () => close(true);
        dialog.addEventListener('click', event => {
            if (event.target === dialog) close(false);
        });
        return dialog;
    }

    window.showConfirmDialog = function(message, title = '确认操作？', type = 'default') {
        const modal = ensureDialog();
        const config = dialogTypes[type] || dialogTypes.default;
        const icon = modal.querySelector('[data-confirm-icon]');
        const okButton = modal.querySelector('[data-confirm-ok]');
        icon.innerText = config.icon;
        icon.className = `confirm-dialog-icon mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full text-4xl ${config.iconClass}`;
        okButton.className = `rounded-2xl px-4 py-3 text-base font-semibold text-white transition ${config.buttonClass}`;
        modal.querySelector('[data-confirm-title]').innerText = title;
        modal.querySelector('[data-confirm-message]').innerText = message;
        modal.classList.remove('hidden');
        modal.classList.add('flex');
        requestAnimationFrame(() => modal.classList.add('confirm-dialog-visible'));
        return new Promise(resolve => { resolveDialog = resolve; });
    };

    const style = document.createElement('style');
    style.textContent = `
        .confirm-dialog { opacity: 0; transition: opacity 180ms ease; }
        .confirm-dialog-visible { opacity: 1; }
        .confirm-dialog-panel { transform: translateY(18px) scale(.94); opacity: 0; transition: transform 220ms cubic-bezier(.2,.8,.2,1), opacity 180ms ease; }
        .confirm-dialog-visible .confirm-dialog-panel { transform: translateY(0) scale(1); opacity: 1; }
        .confirm-dialog-closing .confirm-dialog-panel { transform: translateY(10px) scale(.96); opacity: 0; }
        .confirm-dialog-icon { animation: confirm-dialog-pop 300ms cubic-bezier(.2,.8,.2,1) both; }
        @keyframes confirm-dialog-pop { from { transform: scale(.65) rotate(-8deg); opacity: 0; } to { transform: scale(1) rotate(0); opacity: 1; } }
    `;
    document.head.appendChild(style);
})();
