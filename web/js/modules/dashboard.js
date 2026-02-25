import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';

export async function init(container) {
    clear(container);
    container.appendChild(el('div', { class: 'page-title' }, 'Dashboard'));

    try {
        const libs = await api.libraries.list();
        const libList = Array.isArray(libs) ? libs : [];

        const stats = el('div', { class: 'stats-grid' });
        stats.appendChild(statCard(String(libList.length), 'Libraries'));

        let totalItems = 0;
        libList.forEach(lib => { totalItems += (lib.item_count || 0); });
        stats.appendChild(statCard(String(totalItems), 'Total Items'));

        container.appendChild(stats);

        const section = el('div', {});
        section.appendChild(el('div', { class: 'section-title' }, 'Your Libraries'));

        if (libList.length === 0) {
            section.appendChild(el('div', { class: 'empty-state' },
                el('div', { class: 'empty-state-icon' }, '\uD83C\uDFAC'),
                el('div', { class: 'empty-state-text' }, 'No libraries yet. Add one in Settings.')
            ));
        } else {
            const grid = el('div', { class: 'card-grid' });
            libList.forEach(lib => {
                const typeLabel = (lib.library_type || '').replace(/_/g, ' ');
                const card = el('div', { class: 'card' },
                    el('div', { style: { fontSize: '2rem', marginBottom: '8px' } }, libIcon(lib.library_type)),
                    el('div', { style: { fontWeight: '700', fontSize: '1rem', marginBottom: '4px' } }, lib.name),
                    el('div', { style: { fontSize: '0.78rem', color: 'var(--text-muted)', textTransform: 'capitalize' } }, typeLabel),
                    el('div', { style: { fontSize: '0.85rem', color: 'var(--accent)', marginTop: '8px', fontWeight: '600' } },
                        `${lib.item_count || 0} items`)
                );
                card.style.cursor = 'pointer';
                card.addEventListener('click', () => {
                    window.location.hash = `#/library/${lib.id}`;
                });
                grid.appendChild(card);
            });
            section.appendChild(grid);
        }
        container.appendChild(section);
    } catch (e) {
        container.appendChild(el('div', { class: 'empty-state' },
            el('div', { class: 'empty-state-icon' }, '\u26A0\uFE0F'),
            el('div', { class: 'empty-state-text' }, 'Failed to load dashboard')
        ));
    }
}

function statCard(value, label) {
    return el('div', { class: 'stat-card' },
        el('div', { class: 'stat-value' }, value),
        el('div', { class: 'stat-label' }, label)
    );
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
