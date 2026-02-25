import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';

export function fileBrowser(container, onSelect) {
    let currentPath = '/';
    let selectedPaths = new Set();

    const pathDisplay = el('div', { class: 'form-label', style: { marginBottom: '8px' } });
    const list = el('div', { class: 'file-browser' });
    container.appendChild(pathDisplay);
    container.appendChild(list);

    async function load(path) {
        currentPath = path;
        pathDisplay.textContent = `Current: ${path}`;
        clear(list);

        try {
            const data = await api.libraries.browse(path);
            if (path !== '/') {
                const parent = path.split('/').slice(0, -1).join('/') || '/';
                list.appendChild(el('div', {
                    class: 'file-entry',
                    onClick: () => load(parent),
                }, el('span', { class: 'file-icon' }, '\u2190'), ' ..'));
            }
            if (data.folders) {
                data.folders.forEach(f => {
                    const entry = el('div', {
                        class: `file-entry${selectedPaths.has(f.path) ? ' selected' : ''}`,
                        onClick: (e) => {
                            if (e.detail === 2) {
                                load(f.path);
                            } else {
                                if (selectedPaths.has(f.path)) {
                                    selectedPaths.delete(f.path);
                                } else {
                                    selectedPaths.add(f.path);
                                }
                                entry.classList.toggle('selected');
                                onSelect([...selectedPaths]);
                            }
                        },
                    },
                        el('span', { class: 'file-icon' }, '\uD83D\uDCC1'),
                        ` ${f.name}`
                    );
                    list.appendChild(entry);
                });
            }
        } catch {
            list.appendChild(el('div', { style: { padding: '12px', color: 'var(--text-muted)' } }, 'Cannot read directory'));
        }
    }

    load(currentPath);

    return {
        getSelected: () => [...selectedPaths],
        setSelected: (paths) => { selectedPaths = new Set(paths); load(currentPath); },
    };
}
