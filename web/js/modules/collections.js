import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as modal from '../components/modal.js';
import * as toast from '../components/toast.js';

export async function init(container) {
    clear(container);
    const header = el('div', { class: 'page-title' },
        el('span', {}, 'Collections'),
        el('button', { class: 'btn btn-primary btn-sm', onClick: showCreateModal }, '+ New')
    );
    container.appendChild(header);

    try {
        const cols = await api.get('/collections');
        if (!cols || cols.length === 0) {
            container.appendChild(el('div', { class: 'empty-state' },
                el('div', { class: 'empty-state-icon' }, '\uD83D\uDCDA'),
                el('div', { class: 'empty-state-text' }, 'No collections yet')
            ));
            return;
        }
        const grid = el('div', { class: 'card-grid' });
        cols.forEach(col => {
            grid.appendChild(el('div', { class: 'card', style: { cursor: 'pointer' } },
                el('div', { style: { fontWeight: '700' } }, col.name),
                el('div', { style: { fontSize: '13px', color: 'var(--text-muted)', marginTop: '4px' } },
                    col.description || '')
            ));
        });
        container.appendChild(grid);
    } catch (e) {
        container.appendChild(el('div', { class: 'empty-state' },
            el('div', { class: 'empty-state-text' }, `Error: ${e.message}`)));
    }
}

function showCreateModal() {
    modal.open('New Collection', (body) => {
        body.appendChild(el('div', { class: 'form-group' },
            el('label', { class: 'form-label' }, 'Name'),
            el('input', { class: 'form-input', id: 'col-name' })
        ));
        body.appendChild(el('div', { class: 'form-group' },
            el('label', { class: 'form-label' }, 'Description'),
            el('textarea', { class: 'form-textarea', id: 'col-desc' })
        ));
    }, {
        footer: (footer) => {
            footer.appendChild(el('button', { class: 'btn btn-secondary', onClick: modal.close }, 'Cancel'));
            footer.appendChild(el('button', { class: 'btn btn-primary', onClick: async () => {
                const name = document.getElementById('col-name').value;
                const desc = document.getElementById('col-desc').value;
                if (!name) return;
                try {
                    await api.post('/collections', { name, description: desc || null });
                    modal.close();
                    toast.success('Collection created');
                    init(document.getElementById('page-container'));
                } catch (e) { toast.error(e.message); }
            }}, 'Create'));
        }
    });
}
