// ──── Init ────
checkAuth();

// PWA: Register service worker (P11-05)
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').catch(() => {});
}
// Offline indicator
window.addEventListener('offline', () => { document.body.classList.add('is-offline'); toast('You are offline', 'warning'); });
window.addEventListener('online', () => { document.body.classList.remove('is-offline'); toast('Back online'); });
