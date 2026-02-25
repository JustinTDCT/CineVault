import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as toast from '../components/toast.js';
import { toggle } from '../components/slider-toggle.js';
import { select } from '../components/dropdown.js';
import { fileBrowser } from '../components/file-browser.js';
import * as modal from '../components/modal.js';

let libTypes = [];

export async function init(container, param) {
    clear(container);

    const isNew = !param || param === 'new';
    let lib = defaultLibrary();

    try {
        libTypes = await api.libraries.types();
    } catch {
        libTypes = [];
    }

    if (!isNew) {
        try {
            lib = await api.libraries.get(param);
        } catch (e) {
            container.appendChild(el('div', { class: 'empty-state' },
                el('div', { class: 'empty-state-icon' }, '\u26A0\uFE0F'),
                el('div', { class: 'empty-state-text' }, 'Library not found')));
            return;
        }
    }

    const title = el('div', { class: 'page-title' },
        el('span', {}, isNew ? 'Create Library' : 'Edit Library'),
        el('div', { style: { display: 'flex', gap: '8px' } },
            ...(!isNew ? [el('button', {
                class: 'btn btn-danger btn-sm',
                onClick: () => confirmDelete(lib),
            }, 'Delete')] : []),
            el('button', {
                class: 'btn btn-secondary btn-sm',
                onClick: () => { window.location.hash = '#/settings'; },
            }, 'Back')
        )
    );
    container.appendChild(title);

    const form = el('div', { class: 'lib-edit-layout' });
    container.appendChild(form);

    renderForm(form, lib, isNew);
}

function defaultLibrary() {
    return {
        name: '',
        library_type: 'movies',
        folders: [],
        include_in_homepage: true,
        include_in_search: true,
        retrieve_metadata: true,
        import_nfo: false,
        export_nfo: false,
        normalize_audio: false,
        timeline_scrubbing: false,
        preview_videos: false,
        intro_detection: false,
        credits_detection: false,
        recap_detection: false,
    };
}

function renderForm(container, lib, isNew) {
    const state = { ...lib };

    const leftCol = el('div', { class: 'lib-edit-main' });
    const rightCol = el('div', { class: 'lib-edit-side' });

    // --- Name ---
    const nameGroup = el('div', { class: 'form-group' },
        el('label', { class: 'form-label' }, 'Library Name'),
        el('input', {
            class: 'form-input',
            type: 'text',
            value: state.name,
            placeholder: 'e.g. Movies, Kids Movies...',
            onInput: (e) => { state.name = e.target.value; },
        })
    );
    leftCol.appendChild(nameGroup);

    // --- Type ---
    const typeInfo = getTypeInfo(state.library_type);
    const typeOpts = libTypes.map(t => ({ value: t.value, label: t.label }));
    if (!typeOpts.length) {
        typeOpts.push({ value: 'movies', label: 'Movies' });
    }

    const typeGroup = el('div', { class: 'form-group' });
    typeGroup.appendChild(el('label', { class: 'form-label' }, 'Library Type'));
    if (isNew) {
        typeGroup.appendChild(select(typeOpts, state.library_type, (v) => {
            state.library_type = v;
            clear(container);
            renderForm(container, state, isNew);
        }));
    } else {
        const label = typeOpts.find(t => t.value === state.library_type)?.label || state.library_type;
        typeGroup.appendChild(el('div', {
            class: 'form-value-display',
        }, label));
    }
    leftCol.appendChild(typeGroup);

    // --- Folders ---
    const foldersCard = el('div', { class: 'card', style: { marginTop: 'var(--sp-4)' } });
    foldersCard.appendChild(el('h4', { style: { marginBottom: 'var(--sp-3)' } }, 'Media Folders'));

    if (state.folders && state.folders.length > 0) {
        const tagList = el('div', { class: 'folder-tags' });
        state.folders.forEach((f, idx) => {
            const tag = el('div', { class: 'folder-tag' },
                el('span', {}, f),
                el('button', {
                    class: 'folder-tag-remove',
                    onClick: () => {
                        state.folders.splice(idx, 1);
                        clear(container);
                        renderForm(container, state, isNew);
                    },
                }, '\u00d7')
            );
            tagList.appendChild(tag);
        });
        foldersCard.appendChild(tagList);
    }

    const browserWrap = el('div', { style: { marginTop: 'var(--sp-3)' } });
    const fb = fileBrowser(browserWrap, (selected) => {
        // no-op, user will click Add
    });

    const addFolderBtn = el('button', {
        class: 'btn btn-secondary btn-sm',
        style: { marginTop: 'var(--sp-3)' },
        onClick: () => {
            const selected = fb.getSelected();
            if (selected.length === 0) {
                toast.error('Select at least one folder');
                return;
            }
            selected.forEach(p => {
                if (!state.folders.includes(p)) state.folders.push(p);
            });
            clear(container);
            renderForm(container, state, isNew);
        },
    }, 'Add Selected Folders');

    foldersCard.appendChild(browserWrap);
    foldersCard.appendChild(addFolderBtn);
    leftCol.appendChild(foldersCard);

    // --- General Settings ---
    const generalCard = el('div', { class: 'card' });
    generalCard.appendChild(el('h4', { style: { marginBottom: 'var(--sp-3)' } }, 'General Settings'));
    generalCard.appendChild(toggle('Include in Homepage', state.include_in_homepage, (v) => { state.include_in_homepage = v; }));
    generalCard.appendChild(toggle('Include in Search', state.include_in_search, (v) => { state.include_in_search = v; }));
    generalCard.appendChild(toggle('Retrieve Metadata', state.retrieve_metadata, (v) => { state.retrieve_metadata = v; }));
    generalCard.appendChild(toggle('Import NFO', state.import_nfo, (v) => { state.import_nfo = v; }));
    generalCard.appendChild(toggle('Export NFO', state.export_nfo, (v) => { state.export_nfo = v; }));
    rightCol.appendChild(generalCard);

    // --- Audio Settings (only for types with audio) ---
    if (typeInfo.has_audio) {
        const audioCard = el('div', { class: 'card' });
        audioCard.appendChild(el('h4', { style: { marginBottom: 'var(--sp-3)' } }, 'Audio'));
        audioCard.appendChild(toggle('Normalize Audio', state.normalize_audio, (v) => { state.normalize_audio = v; }));
        rightCol.appendChild(audioCard);
    }

    // --- Video Settings (only for video types) ---
    if (typeInfo.is_video) {
        const videoCard = el('div', { class: 'card' });
        videoCard.appendChild(el('h4', { style: { marginBottom: 'var(--sp-3)' } }, 'Video'));
        videoCard.appendChild(toggle('Timeline Scrubbing', state.timeline_scrubbing, (v) => { state.timeline_scrubbing = v; }));
        videoCard.appendChild(toggle('Preview Videos', state.preview_videos, (v) => { state.preview_videos = v; }));
        videoCard.appendChild(toggle('Intro Detection', state.intro_detection, (v) => { state.intro_detection = v; }));
        videoCard.appendChild(toggle('Credits Detection', state.credits_detection, (v) => { state.credits_detection = v; }));
        videoCard.appendChild(toggle('Recap Detection', state.recap_detection, (v) => { state.recap_detection = v; }));
        rightCol.appendChild(videoCard);
    }

    // --- Save Button ---
    const actions = el('div', { class: 'lib-edit-actions' },
        el('button', {
            class: 'btn btn-primary',
            onClick: () => saveLibrary(state, isNew),
        }, isNew ? 'Create Library' : 'Save Changes')
    );
    rightCol.appendChild(actions);

    container.appendChild(leftCol);
    container.appendChild(rightCol);
}

function getTypeInfo(typeValue) {
    const found = libTypes.find(t => t.value === typeValue);
    if (found) return found;
    return { is_video: true, has_audio: true, has_metadata: true };
}

async function saveLibrary(state, isNew) {
    if (!state.name.trim()) {
        toast.error('Library name is required');
        return;
    }
    if (!state.folders || state.folders.length === 0) {
        toast.error('At least one folder is required');
        return;
    }

    try {
        if (isNew) {
            await api.libraries.create(state);
            toast.success('Library created');
        } else {
            await api.libraries.update(state.id, state);
            toast.success('Library updated');
        }
        window.location.hash = '#/settings';
    } catch (e) {
        toast.error(`Failed to save: ${e.message}`);
    }
}

function confirmDelete(lib) {
    modal.confirm(`Delete "${lib.name}"? This will remove the library and all associated data. This cannot be undone.`, async () => {
        try {
            await api.libraries.del(lib.id);
            toast.success('Library deleted');
            window.location.hash = '#/settings';
        } catch (e) {
            toast.error(`Failed to delete: ${e.message}`);
        }
    });
}
