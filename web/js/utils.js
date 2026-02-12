// CineVault utility functions module (P11-04)
// Extracted common helpers for future modular use.

export function posterSrc(path, updatedAt, width) {
    if (!path) return '';
    const ts = updatedAt ? new Date(updatedAt).getTime() : Date.now();
    let url = path + '?v=' + ts;
    if (width && path.startsWith('http')) url += '&w=' + width;
    return url;
}

export function posterSrcset(path, updatedAt) {
    if (!path || !path.startsWith('http')) return '';
    return posterSrc(path, updatedAt, 300) + ' 300w, ' + posterSrc(path, updatedAt, 500) + ' 500w';
}

export function formatDuration(seconds) {
    if (!seconds) return '';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return h > 0 ? h + 'h ' + m + 'm' : m + 'm';
}

export function debounce(fn, ms) {
    let timer;
    return function (...args) {
        clearTimeout(timer);
        timer = setTimeout(() => fn.apply(this, args), ms);
    };
}
