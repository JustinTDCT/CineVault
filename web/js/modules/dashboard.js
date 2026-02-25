import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';

export async function init(container) {
    clear(container);
    container.appendChild(el('div', { class: 'page-title' }, 'Dashboard'));

    const stats = el('div', { class: 'stats-grid' });
    container.appendChild(stats);

    try {
        const libs = await api.libraries.list();
        stats.appendChild(statCard(libs.length, 'Libraries'));

        const recent = el('div', {});
        recent.appendChild(el('h3', { style: { marginBottom: '16px' } }, 'Your Libraries'));

        if (libs.length === 0) {
            recent.appendChild(el('div', { class: 'empty-state' },
                el('div', { class: 'empty-state-icon' }, '\uD83C\uDFAC'),
                el('div', { class: 'empty-state-text' }, 'No libraries yet. Add one in Settings.')
            ));
        } else {
            const grid = el('div', { class: 'card-grid' });
            libs.forEach(lib => {
                const card = el('div', { class: 'card', style: { cursor: 'pointer' } },
                    el('div', { style: { fontWeight: '700', marginBottom: '4px' } }, lib.name),
                    el('div', { style: { fontSize: '13px', color: 'var(--text-muted)' } }, lib.library_type)
                );
                card.addEventListener('click', () => {
                    window.location.hash = `#/library/${lib.id}`;
                });
                grid.appendChild(card);
            });
            recent.appendChild(grid);
        }
        container.appendChild(recent);
    } catch (e) {
        container.appendChild(el('div', { class: 'empty-state' },
            el('div', { class: 'empty-state-text' }, 'Failed to load dashboard')
        ));
    }
}

function statCard(value, label) {
    return el('div', { class: 'stat-card' },
        el('div', { class: 'stat-value' }, String(value)),
        el('div', { class: 'stat-label' }, label)
    );
}
