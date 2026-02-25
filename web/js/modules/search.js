import { el, clear, qs } from '../utils/dom.js';
import * as api from '../services/api.js';
import { mediaCard } from '../components/card.js';

export async function init(container) {
    clear(container);
    container.appendChild(el('div', { class: 'page-title' }, 'Search'));

    const form = el('div', { style: { display: 'flex', gap: '8px', marginBottom: '20px' } });
    const input = el('input', {
        class: 'form-input',
        type: 'text',
        placeholder: 'Search your libraries...',
        id: 'search-input',
    });
    const btn = el('button', { class: 'btn btn-primary', onClick: doSearch }, 'Search');
    form.appendChild(input);
    form.appendChild(btn);
    container.appendChild(form);

    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') doSearch();
    });

    const results = el('div', { id: 'search-results' });
    container.appendChild(results);

    async function doSearch() {
        const q = qs('#search-input').value.trim();
        if (!q) return;
        const r = qs('#search-results');
        clear(r);
        r.appendChild(el('div', { class: 'loading-center' }, el('div', { class: 'spinner' })));

        try {
            const data = await api.get(`/search?q=${encodeURIComponent(q)}`);
            clear(r);
            const items = data.results || [];
            if (items.length === 0) {
                r.appendChild(el('div', { class: 'empty-state' },
                    el('div', { class: 'empty-state-text' }, 'No results found')));
                return;
            }
            const grid = el('div', { class: 'media-grid' });
            items.forEach(item => {
                grid.appendChild(mediaCard(item, {
                    onClick: (it) => { window.location.hash = `#/media-detail/${it.id}`; },
                }));
            });
            r.appendChild(grid);
        } catch (e) {
            clear(r);
            r.appendChild(el('div', { class: 'empty-state' },
                el('div', { class: 'empty-state-text' }, `Error: ${e.message}`)));
        }
    }
}
