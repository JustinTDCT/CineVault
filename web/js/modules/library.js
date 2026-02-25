import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import { mediaCard } from '../components/card.js';
import { VirtualGrid } from '../components/grid.js';

let grid = null;

export async function init(container, libraryId) {
    clear(container);
    if (grid) { grid.destroy(); grid = null; }

    const lib = await api.libraries.get(libraryId);
    const header = el('div', { class: 'page-title' },
        el('span', {}, lib.name),
        el('div', { style: { display: 'flex', gap: '8px' } },
            el('button', { class: 'btn btn-sm btn-secondary', onClick: () => scanLibrary(libraryId) }, 'Scan'),
            sortDropdown()
        )
    );
    container.appendChild(header);

    const gridContainer = el('div', { style: { height: 'calc(100vh - 120px)' } });
    container.appendChild(gridContainer);

    const data = await api.media.list(libraryId, { limit: 200 });
    const items = data.items || [];

    if (items.length === 0) {
        gridContainer.appendChild(el('div', { class: 'empty-state' },
            el('div', { class: 'empty-state-icon' }, '\uD83D\uDCC2'),
            el('div', { class: 'empty-state-text' }, 'No items yet. Run a scan to discover media.')
        ));
        return;
    }

    grid = new VirtualGrid(gridContainer, {
        renderItem: (item) => mediaCard(item, {
            overlays: buildOverlays(item),
            onClick: (it) => { window.location.hash = `#/media-detail/${it.id}`; },
        }),
    });
    grid.setItems(items);
}

function buildOverlays(item) {
    const overlays = [];
    if (item.resolution) {
        overlays.push({ text: item.resolution, position: 'top-left' });
    }
    return overlays;
}

function sortDropdown() {
    const sel = el('select', { class: 'form-select', style: { width: 'auto', fontSize: '12px' } });
    ['Title', 'Year', 'Date Added'].forEach(label => {
        sel.appendChild(el('option', { value: label.toLowerCase().replace(' ', '_') }, label));
    });
    return sel;
}

async function scanLibrary(id) {
    try {
        await api.scanner.scan(id);
        import('../components/toast.js').then(t => t.success('Scan started'));
    } catch (e) {
        import('../components/toast.js').then(t => t.error(e.message));
    }
}
