const API = '/api/v1';
let currentUser = null;
let allLibraries = [];
let ws = null;
// Overlay badge display preferences (fetched from server per-user)
let overlayPrefs = { resolution_hdr: true, audio_codec: true, ratings: true, content_rating: false, edition_type: true, source_type: false };
let _userRegion = '';
let hlsPlayer = null;
let mpegtsPlayer = null;
let currentMediaId = null;
const activeTasks = {};
let taskFadeTimer = null;

function headers() { return { 'Authorization': 'Bearer ' + localStorage.getItem('token'), 'Content-Type': 'application/json' }; }

async function api(method, path, body) {
    const opts = { method, headers: headers() };
    if (body) opts.body = JSON.stringify(body);
    try {
        const res = await fetch(API + path, opts);
        if (res.status === 401 && path !== '/auth/login') {
            localStorage.removeItem('token');
            localStorage.removeItem('user');
            sessionStorage.removeItem('profile_picked');
            currentUser = null;
            if (ws) { ws.close(); ws = null; }
            checkAuth();
            return { success: false, error: 'Session expired' };
        }
        return res.json();
    } catch(e) { return { success: false, error: e.message }; }
}

function toast(msg, type='success') {
    const c = document.getElementById('toastContainer');
    const t = document.createElement('div');
    t.className = 'toast message ' + type;
    t.textContent = msg;
    c.appendChild(t);
    setTimeout(() => t.remove(), 4000);
}

const MEDIA_ICONS = { movies:'&#127916;', adult_movies:'&#128274;', tv_shows:'&#128250;', music:'&#127925;', music_videos:'&#127911;', home_videos:'&#127909;', other_videos:'&#128253;', images:'&#128247;', audiobooks:'&#128214;' };
const MEDIA_LABELS = { movies:'Movies', adult_movies:'Adult Movies', tv_shows:'TV Shows', music:'Music', music_videos:'Music Videos', home_videos:'Home Videos', other_videos:'Other Videos', images:'Photos', audiobooks:'Audiobooks' };
function mediaIcon(type) { return MEDIA_ICONS[type] || '&#128191;'; }
function posterSrc(path, updatedAt, width) { if (!path) return ''; const ts = updatedAt ? new Date(updatedAt).getTime() : Date.now(); let url = path + '?v=' + ts; if (width && path.startsWith('http')) url += '&w=' + width; return url; }
function posterSrcset(path, updatedAt) { if (!path || !path.startsWith('http')) return ''; return posterSrc(path, updatedAt, 300) + ' 300w, ' + posterSrc(path, updatedAt, 500) + ' 500w'; }
async function fetchOverlayPrefs() {
    try {
        const res = await api('GET', '/settings/display');
        if (res.success && res.data && res.data.overlay_settings) {
            overlayPrefs = res.data.overlay_settings;
        }
    } catch(e) { /* keep defaults */ }
}
async function fetchUserRegion() {
    try {
        const res = await api('GET', '/settings/general');
        if (res.success && res.data) {
            _userRegion = res.data.region || '';
        }
    } catch(e) { /* keep default */ }
}
function formatDuration(sec) { if (!sec) return ''; const m = Math.floor(sec/60); const s = sec%60; return m + ':' + String(s).padStart(2,'0'); }
function formatTime(sec) { const h = Math.floor(sec/3600); const m = Math.floor((sec%3600)/60); const s = Math.floor(sec%60); return (h>0?h+':':'')+String(m).padStart(h>0?2:1,'0')+':'+String(s).padStart(2,'0'); }
