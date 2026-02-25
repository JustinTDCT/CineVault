import { qs, clear, el, qsa } from './utils/dom.js';
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
    auth: () => import('./modules/auth.js'),
};

async function boot() {
    try {
        currentUser = await api.users.me();
        buildNav();
        handleRoute();
    } catch {
        showAuth();
    }

    window.addEventListener('hashchange', handleRoute);

    qs('#sidebar-toggle').addEventListener('click', () => {
        qs('#sidebar').classList.toggle('collapsed');
    });
}

function showAuth() {
    const container = qs('#page-container');
    import('./modules/auth.js').then(mod => {
        mod.init(container, (data) => {
            currentUser = { user_id: data.user_id, is_admin: data.is_admin };
            api.users.me().then(u => {
                currentUser = u;
                buildNav();
                window.location.hash = '#/dashboard';
            });
        });
    });
}

async function buildNav() {
    const navList = qs('#nav-libraries');
    const existing = qsa('.nav-lib-item');
    existing.forEach(e => e.remove());

    try {
        const libs = await api.libraries.list();
        libs.forEach(lib => {
            const li = el('li', { class: 'nav-lib-item' });
            li.appendChild(el('a', {
                class: 'nav-link',
                href: `#/library/${lib.id}`,
                'data-route': `library/${lib.id}`,
            },
                el('span', { class: 'nav-icon' }, libIcon(lib.library_type)),
                el('span', { class: 'nav-label' }, lib.name)
            ));
            navList.insertAdjacentElement('afterend', li);
        });
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

boot();
