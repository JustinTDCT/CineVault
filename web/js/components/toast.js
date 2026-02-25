import { el, qs } from '../utils/dom.js';

export function show(message, type = 'info', duration = 4000) {
    const container = qs('#toast-container');
    const toast = el('div', { class: `toast toast-${type}` }, message);
    container.appendChild(toast);

    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transition = 'opacity 0.3s';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

export function success(msg) { show(msg, 'success'); }
export function error(msg) { show(msg, 'error'); }
export function info(msg) { show(msg, 'info'); }
