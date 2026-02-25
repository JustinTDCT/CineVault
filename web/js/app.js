import { qs, clear, el, qsa, show, hide } from './utils/dom.js';
import * as api from './services/api.js';
import * as toast from './components/toast.js';

let currentUser = null;

const routes = {
    dashboard: () => import('./modules/dashboard.js'),
    library: () => import('./modules/library.js'),
    'media-detail': () => import('./modules/media-detail.js'),
    player: () => import('./modules/player.js'),
    settings: () => import('./modules/settings.js'),
    profile: () => import('./modules/profile.js'),
    search: () => import('./modules/search.js'),
    collections: () => import('./modules/collections.js'),
};

async function boot() {
    try {
        currentUser = await api.users.me();
        onAuthenticated();
    } catch {
        showAuth();
    }
    window.addEventListener('hashchange', handleRoute);
    setupUserMenu();
}

function setupUserMenu() {
    const avatar = qs('#userAvatar');
    const dropdown = qs('#userDropdown');
    if (avatar && dropdown) {
        avatar.addEventListener('click', (e) => {
            e.stopPropagation();
            dropdown.classList.toggle('open');
        });
        document.addEventListener('click', () => dropdown.classList.remove('open'));
    }
    const logoutBtn = qs('#logoutBtn');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', async () => {
            try {
                await api.auth.logout();
            } catch { /* ignore */ }
            api.setToken(null);
            currentUser = null;
            window.location.hash = '';
            window.location.reload();
        });
    }
}

async function showAuth() {
    const hasUsers = await checkHasUsers();
    if (hasUsers) {
        showLogin();
    } else {
        showSetup();
    }
}

function showSetup() {
    const overlay = qs('#setupOverlay');
    show(overlay);
    hide(qs('#loginOverlay'));

    const form = qs('#setupForm');
    form.onsubmit = async (e) => {
        e.preventDefault();
        const msgEl = qs('#setupMessage');
        msgEl.textContent = '';

        const full_name = qs('#setupFullName').value.trim();
        const email = qs('#setupEmail').value.trim();
        const password = qs('#setupPassword').value;
        const pin = qs('#setupPin').value.trim();

        if (!full_name || !email || !password) {
            msgEl.textContent = 'All fields except PIN are required.';
            return;
        }

        try {
            const data = await api.auth.register({
                full_name, email, password,
                pin: pin || undefined,
            });
            api.setToken(data.token);
            hide(overlay);
            currentUser = await api.users.me();
            onAuthenticated();
        } catch (err) {
            msgEl.textContent = err.message || 'Registration failed';
        }
    };
}

function showLogin() {
    const overlay = qs('#loginOverlay');
    show(overlay);
    hide(qs('#setupOverlay'));

    const form = qs('#loginForm');
    form.onsubmit = async (e) => {
        e.preventDefault();
        const msgEl = qs('#loginMessage');
        msgEl.textContent = '';

        const email = qs('#loginEmail').value.trim();
        const password = qs('#loginPassword').value;

        try {
            const data = await api.auth.login({ email, password });
            api.setToken(data.token);
            hide(overlay);
            currentUser = await api.users.me();
            onAuthenticated();
        } catch (err) {
            msgEl.textContent = err.message || 'Invalid credentials';
        }
    };
}

function onAuthenticated() {
    hide(qs('#setupOverlay'));
    hide(qs('#loginOverlay'));
    updateUserAvatar();
    buildNav();
    handleRoute();
}

function updateUserAvatar() {
    const avatar = qs('#userAvatar');
    if (currentUser && currentUser.full_name) {
        const initials = currentUser.full_name.split(' ')
            .map(w => w[0]).join('').toUpperCase().slice(0, 2);
        avatar.textContent = initials;
    }
}

async function buildNav() {
    const navSection = qs('#librariesNav');
    qsa('.nav-lib-item').forEach(e => e.remove());

    try {
        const libs = await api.libraries.list();
        if (Array.isArray(libs)) {
            libs.forEach(lib => {
                const link = el('a', {
                    class: 'nav-link nav-lib-item',
                    href: `#/library/${lib.id}`,
                    'data-route': `library/${lib.id}`,
                },
                    el('span', { class: 'nav-icon' }, libIcon(lib.library_type)),
                    el('span', {}, lib.name),
                    el('span', { class: 'nav-badge' }, String(lib.item_count || 0))
                );
                navSection.appendChild(link);
            });
        }
    } catch { /* not authenticated yet */ }

    const userInfo = qs('#user-info');
    if (currentUser) {
        userInfo.textContent = currentUser.full_name || currentUser.email || '';
    }
}

async function handleRoute() {
    const hash = window.location.hash.slice(2) || 'dashboard';
    const parts = hash.split('/');
    const routeName = parts[0];
    const routeParam = parts[1] || null;

    qsa('.nav-link').forEach(link => {
        const route = link.getAttribute('data-route') || '';
        link.classList.toggle('active', route === hash || route === routeName);
    });

    const container = qs('#page-container');
    const loader = routes[routeName];
    if (!loader) {
        clear(container);
        container.appendChild(el('div', { class: 'empty-state' },
            el('div', { class: 'empty-state-icon' }, '\uD83D\uDCC1'),
            el('div', { class: 'empty-state-text' }, 'Page not found')));
        return;
    }

    clear(container);
    container.appendChild(el('div', { class: 'loading-center' }, el('div', { class: 'spinner' })));

    try {
        const mod = await loader();
        clear(container);
        await mod.init(container, routeParam);
    } catch (e) {
        clear(container);
        if (e.status === 401) {
            showAuth();
        } else {
            container.appendChild(el('div', { class: 'empty-state' },
                el('div', { class: 'empty-state-icon' }, '\u26A0\uFE0F'),
                el('div', { class: 'empty-state-text' }, `Error: ${e.message}`)));
        }
    }
}

function libIcon(type) {
    const icons = {
        movies: '\uD83C\uDFAC', tv_shows: '\uD83D\uDCFA', adult_movies: '\uD83D\uDD1E',
        adult_clips: '\uD83D\uDD1E', home_movies: '\uD83C\uDFE0', other_movies: '\uD83C\uDFAC',
        music: '\uD83C\uDFB5', music_videos: '\uD83C\uDFB6', audiobooks: '\uD83C\uDFA7',
        ebooks: '\uD83D\uDCDA', comic_books: '\uD83D\uDCDC',
    };
    return icons[type] || '\uD83D\uDCC1';
}

async function checkHasUsers() {
    try {
        const resp = await fetch('/api/health');
        if (!resp.ok) return false;
        const data = await resp.json();
        return data?.data?.user_count > 0;
    } catch {
        return false;
    }
}

boot();
