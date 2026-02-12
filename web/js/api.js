// CineVault API client module (P11-04)
// This is the foundation for modularizing the frontend.
// Currently index.html contains all logic inline; this module will be
// incrementally adopted as the app is refactored.

export const API_BASE = '/api/v1';

export async function api(method, path, body) {
    const token = localStorage.getItem('token');
    const opts = {
        method,
        headers: {
            'Content-Type': 'application/json',
        },
    };
    if (token) opts.headers['Authorization'] = 'Bearer ' + token;
    if (body) opts.body = JSON.stringify(body);

    try {
        const resp = await fetch(API_BASE + path, opts);
        if (resp.status === 401) {
            localStorage.removeItem('token');
            localStorage.removeItem('user');
            window.location.reload();
            return { success: false, error: 'Session expired' };
        }
        return await resp.json();
    } catch (err) {
        return { success: false, error: err.message };
    }
}

export function toast(message, type = 'success') {
    const container = document.getElementById('toastContainer');
    if (!container) return;
    const t = document.createElement('div');
    t.className = 'toast toast-' + type;
    t.textContent = message;
    container.appendChild(t);
    setTimeout(() => t.classList.add('show'), 10);
    setTimeout(() => { t.classList.remove('show'); setTimeout(() => t.remove(), 300); }, 3000);
}
