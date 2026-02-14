
// ──── Multi-Select State Manager ────
const selectionState = {
    active: false,
    selectedIds: new Set(),
    lastClickedId: null,
    tagMode: 'add',     // add | remove | replace
    notesMode: 'append', // append | replace

    toggle(id, shiftKey) {
        if (shiftKey && this.lastClickedId && this.lastClickedId !== id) {
            this._rangeSelect(id);
        } else if (this.selectedIds.has(id)) {
            this.selectedIds.delete(id);
        } else {
            this.selectedIds.add(id);
        }
        this.lastClickedId = id;
        this._sync();
    },

    _rangeSelect(id) {
        const cards = Array.from(document.querySelectorAll('.media-card[data-media-id]'));
        const ids = cards.map(c => c.dataset.mediaId);
        const startIdx = ids.indexOf(this.lastClickedId);
        const endIdx = ids.indexOf(id);
        if (startIdx === -1 || endIdx === -1) return;
        const lo = Math.min(startIdx, endIdx);
        const hi = Math.max(startIdx, endIdx);
        for (let i = lo; i <= hi; i++) {
            this.selectedIds.add(ids[i]);
        }
    },

    selectAll() {
        document.querySelectorAll('.media-card[data-media-id]').forEach(c => {
            this.selectedIds.add(c.dataset.mediaId);
        });
        this._sync();
    },

    clear() {
        this.selectedIds.clear();
        this.lastClickedId = null;
        this._sync();
    },

    count() { return this.selectedIds.size; },

    _sync() {
        const n = this.selectedIds.size;
        this.active = n > 0;

        // Toggle body class
        document.body.classList.toggle('select-mode', this.active);

        // Update action bar
        const bar = document.getElementById('bulkActionBar');
        bar.classList.toggle('visible', this.active);
        document.getElementById('bulkCount').textContent = n + ' selected';

        // Update card visual state
        document.querySelectorAll('.media-card[data-media-id]').forEach(card => {
            card.classList.toggle('selected', this.selectedIds.has(card.dataset.mediaId));
        });
    }
};

function handleCardClick(id, event) {
    if (selectionState.active) {
        event.preventDefault();
        event.stopPropagation();
        selectionState.toggle(id, event.shiftKey);
    } else {
        loadMediaDetail(id);
    }
}

function handleCheckboxClick(id, event) {
    event.preventDefault();
    event.stopPropagation();
    selectionState.toggle(id, event.shiftKey);
}

function selectAllVisible() {
    selectionState.selectAll();
}

// ──── Bulk Edit Modal Logic ────
function toggleBulkField(rowId) {
    const row = document.getElementById(rowId);
    const check = row.querySelector('input[type="checkbox"]');
    const inputs = row.querySelectorAll('.bulk-field-body input, .bulk-field-body select, .bulk-field-body textarea');
    row.classList.toggle('active', check.checked);
    inputs.forEach(inp => inp.disabled = !check.checked);
}

function setBulkTagMode(mode) {
    selectionState.tagMode = mode;
    document.getElementById('bfTagModeAdd').classList.toggle('active', mode === 'add');
    document.getElementById('bfTagModeRemove').classList.toggle('active', mode === 'remove');
    document.getElementById('bfTagModeReplace').classList.toggle('active', mode === 'replace');
}

function setBulkNotesMode(mode) {
    selectionState.notesMode = mode;
    document.getElementById('bfNotesModeAppend').classList.toggle('active', mode === 'append');
    document.getElementById('bfNotesModeReplace').classList.toggle('active', mode === 'replace');
}

function openBulkEditModal() {
    if (selectionState.count() === 0) return;
    document.getElementById('bulkEditSubtitle').textContent =
        'Editing ' + selectionState.count() + ' items — only checked fields will be applied';
    // Reset fields
    ['bfRatingCheck','bfEditionCheck','bfTagsCheck','bfNotesCheck'].forEach(id => {
        document.getElementById(id).checked = false;
    });
    ['bfRating','bfEdition','bfTags','bfNotes'].forEach(id => {
        const row = document.getElementById(id);
        row.classList.remove('active');
        row.querySelectorAll('.bulk-field-body input, .bulk-field-body select, .bulk-field-body textarea')
            .forEach(inp => { inp.disabled = true; inp.value = ''; });
    });
    setBulkTagMode('add');
    setBulkNotesMode('append');
    document.getElementById('bulkEditOverlay').classList.add('active');
}

function closeBulkEditModal() {
    document.getElementById('bulkEditOverlay').classList.remove('active');
}

async function saveBulkEdit() {
    const ids = Array.from(selectionState.selectedIds);
    if (ids.length === 0) return;
    const fields = {};

    if (document.getElementById('bfRatingCheck').checked) {
        const v = document.getElementById('bfRatingVal').value;
        if (v !== '') fields.rating = parseFloat(v);
    }
    if (document.getElementById('bfEditionCheck').checked) {
        fields.edition_type = document.getElementById('bfEditionVal').value;
    }
    if (document.getElementById('bfTagsCheck').checked) {
        fields.custom_tags = document.getElementById('bfTagsVal').value;
        fields.tag_mode = selectionState.tagMode;
    }
    if (document.getElementById('bfNotesCheck').checked) {
        fields.custom_notes = document.getElementById('bfNotesVal').value;
        fields.notes_mode = selectionState.notesMode;
    }

    if (Object.keys(fields).length === 0) {
        toast('No fields selected to apply', 'error');
        return;
    }

    const res = await api('PUT', '/media/bulk', { ids, fields });
    if (res.success) {
        toast('Updated ' + ids.length + ' items');
        closeBulkEditModal();
        selectionState.clear();
    } else {
        toast(res.error || 'Bulk update failed', 'error');
    }
}

// ──── Bulk Actions ────
async function bulkMarkPlayed() {
    const ids = Array.from(selectionState.selectedIds);
    if (ids.length === 0) return;
    const res = await api('POST', '/media/bulk-action', { ids, action: 'mark_played' });
    if (res.success) { toast(ids.length + ' items marked as played'); selectionState.clear(); }
    else toast(res.error || 'Failed', 'error');
}

async function bulkMarkUnplayed() {
    const ids = Array.from(selectionState.selectedIds);
    if (ids.length === 0) return;
    const res = await api('POST', '/media/bulk-action', { ids, action: 'mark_unplayed' });
    if (res.success) { toast(ids.length + ' items marked as unplayed'); selectionState.clear(); }
    else toast(res.error || 'Failed', 'error');
}

async function bulkRefreshMetadata() {
    const ids = Array.from(selectionState.selectedIds);
    if (ids.length === 0) return;
    const res = await api('POST', '/media/bulk-action', { ids, action: 'refresh_metadata' });
    if (res.success) { toast('Metadata refresh queued for ' + ids.length + ' items'); selectionState.clear(); }
    else toast(res.error || 'Failed', 'error');
}

async function bulkDelete() {
    const ids = Array.from(selectionState.selectedIds);
    if (ids.length === 0) return;
    if (!confirm('Delete ' + ids.length + ' media items? This cannot be undone.')) return;
    const res = await api('POST', '/media/bulk-action', { ids, action: 'delete' });
    if (res.success) {
        toast(ids.length + ' items deleted');
        selectionState.clear();
        // Reload current view
        const mc = document.getElementById('mainContent');
        document.querySelectorAll('.media-card').forEach(c => {
            if (ids.includes(c.dataset.mediaId)) c.remove();
        });
    } else toast(res.error || 'Delete failed', 'error');
}

// ──── Collection Picker ────
let _pickerSelectedCollections = new Set();

async function openBulkAddToCollection() {
    if (selectionState.count() === 0) return;
    _pickerSelectedCollections.clear();
    document.getElementById('pickerNewCollName').value = '';
    const list = document.getElementById('collectionPickerList');
    list.innerHTML = '<div class="spinner"></div> Loading...';
    document.getElementById('collectionPickerOverlay').classList.add('active');
    await _refreshPickerList();
}

async function _refreshPickerList() {
    const list = document.getElementById('collectionPickerList');
    const res = await api('GET', '/collections');
    const collections = (res.success && res.data) ? res.data.filter(c => c.collection_type !== 'smart') : [];
    if (collections.length === 0) {
        list.innerHTML = '<div style="color:var(--text-tertiary);padding:12px;">No manual collections yet. Create one above.</div>';
        return;
    }
    list.innerHTML = collections.map(c => `
        <div class="collection-picker-item${_pickerSelectedCollections.has(c.id) ? ' selected' : ''}" onclick="togglePickerCollection('${c.id}', this)">
            <input type="checkbox"${_pickerSelectedCollections.has(c.id) ? ' checked' : ''} onclick="event.stopPropagation();">
            <span>${c.name}</span>
        </div>
    `).join('');
}

async function pickerCreateCollection() {
    const input = document.getElementById('pickerNewCollName');
    const name = input.value.trim();
    if (!name) { toast('Enter a collection name', 'error'); return; }
    const body = { name, collection_type: 'manual', visibility: 'private' };
    // Attach library context so the collection appears in library-scoped views
    if (_gridState.libraryId) {
        body.library_id = _gridState.libraryId;
    }
    const res = await api('POST', '/collections', body);
    if (res.success) {
        toast('Collection "' + name + '" created');
        input.value = '';
        // Auto-select the newly created collection
        if (res.data && res.data.id) {
            _pickerSelectedCollections.add(res.data.id);
        }
        await _refreshPickerList();
    } else {
        toast(res.error || 'Failed to create collection', 'error');
    }
}

function togglePickerCollection(id, el) {
    const cb = el.querySelector('input[type="checkbox"]');
    if (_pickerSelectedCollections.has(id)) {
        _pickerSelectedCollections.delete(id);
        cb.checked = false;
        el.classList.remove('selected');
    } else {
        _pickerSelectedCollections.add(id);
        cb.checked = true;
        el.classList.add('selected');
    }
}

function closeCollectionPicker() {
    document.getElementById('collectionPickerOverlay').classList.remove('active');
}

async function saveToCollections() {
    const mediaIds = Array.from(selectionState.selectedIds);
    const collIds = Array.from(_pickerSelectedCollections);
    if (collIds.length === 0) { toast('Select at least one collection', 'error'); return; }

    let added = 0;
    for (const collId of collIds) {
        const items = mediaIds.map((mid, i) => ({ media_item_id: mid, position: i }));
        const res = await api('POST', '/collections/' + collId + '/items/bulk', items);
        if (res.success) added++;
    }
    toast('Added ' + mediaIds.length + ' items to ' + added + ' collection' + (added !== 1 ? 's' : ''));
    closeCollectionPicker();
    selectionState.clear();
}

// ──── Task Activity Tracker ────
function handleTaskUpdate(data) {
    const taskId = data.task_id;
    if (!taskId) return;
    if (data.status === 'complete' || data.status === 'failed') {
        delete activeTasks[taskId];
        // Keep indicator visible briefly after last task completes
        if (Object.keys(activeTasks).length === 0) {
            clearTimeout(taskFadeTimer);
            taskFadeTimer = setTimeout(() => updateTaskIndicator(), 2000);
            // Show 100% momentarily
            const ring = document.getElementById('taskRingFill');
            if (ring) ring.setAttribute('stroke-dashoffset', '0');
            return;
        }
    } else {
        clearTimeout(taskFadeTimer);
        activeTasks[taskId] = {
            type: data.task_type || 'unknown',
            description: data.description || data.task_type || 'Task',
            progress: data.progress || 0,
            status: data.status || 'running'
        };
    }
    updateTaskIndicator();
}

function updateTaskIndicator() {
    const indicator = document.getElementById('taskIndicator');
    const tasks = Object.values(activeTasks);
    const count = tasks.length;
    if (count === 0) {
        indicator.style.display = 'none';
        indicator.classList.remove('active');
        return;
    }
    indicator.style.display = '';
    indicator.classList.add('active');
    // Aggregate progress (average across all active tasks)
    const avgProgress = tasks.reduce((sum, t) => sum + (t.progress || 0), 0) / count;
    const ring = document.getElementById('taskRingFill');
    if (ring) ring.setAttribute('stroke-dashoffset', String(100 - avgProgress));
    // Task count
    const countEl = document.getElementById('taskCount');
    if (countEl) countEl.textContent = count;
    // Tooltip
    const tooltip = document.getElementById('taskTooltip');
    if (tooltip) {
        tooltip.innerHTML = '<div class="task-tooltip-header">Active Tasks</div>' +
            tasks.map(t => `<div class="task-tooltip-item">
                <div class="task-tooltip-name">${t.description}</div>
                <div class="task-tooltip-progress">
                    <div class="task-tooltip-bar"><div class="task-tooltip-bar-fill" style="width:${Math.round(t.progress)}%"></div></div>
                    <span class="task-tooltip-pct">${Math.round(t.progress)}%</span>
                </div>
            </div>`).join('');
    }
}

// ──── WebSocket ────
function connectWS() {
    if (ws) ws.close();
    const token = localStorage.getItem('token');
    if (!token) return;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${proto}//${location.host}/api/v1/ws?token=${token}`);
    const badge = document.getElementById('wsStatus');
    ws.onopen = () => { badge.style.display=''; badge.className='ws-status tag tag-green'; badge.textContent='Live'; };
    ws.onclose = () => { badge.className='ws-status tag tag-red'; badge.textContent='Offline'; setTimeout(connectWS, 5000); };
    ws.onmessage = (e) => {
        try {
            const msg = JSON.parse(e.data);
            handleWSEvent(msg);
        } catch {}
    };
}

// ──── Live scan tracking for incremental grid updates ────
const _scanTracker = {};  // { [libraryId]: { lastAdded: 0, lastFetchTime: 0 } }

function handleWSEvent(msg) {
    switch(msg.event) {
        case 'scan:start': {
            toast('Scanning: ' + (msg.data?.name || ''), 'success');
            const libId = msg.data?.library_id;
            if (libId) {
                _scanTracker[libId] = { lastAdded: 0, lastFetchTime: 0 };
                const prog = document.getElementById('scan-progress-' + libId);
                if (prog) prog.classList.add('active');
                const btn = document.getElementById('scan-btn-' + libId);
                if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>Scanning...'; }
            }
            break;
        }
        case 'scan:progress': {
            const libId = msg.data?.library_id;
            if (!libId) break;
            const current = msg.data.current || 0;
            const total = msg.data.total || 0;
            const filesAdded = msg.data.files_added || 0;
            const filename = msg.data.filename || '';
            const pct = total > 0 ? Math.round((current / total) * 100) : 0;
            const fill = document.getElementById('scan-fill-' + libId);
            if (fill) fill.style.width = pct + '%';
            const fileEl = document.getElementById('scan-file-' + libId);
            if (fileEl) fileEl.textContent = filename;
            const countEl = document.getElementById('scan-count-' + libId);
            if (countEl) countEl.textContent = current + ' / ' + total + ' (' + pct + '%)';
            const prog = document.getElementById('scan-progress-' + libId);
            if (prog && !prog.classList.contains('active')) prog.classList.add('active');

            // Live grid update: if user is viewing this library and new items were added
            if (typeof _gridState !== 'undefined' && _gridState.libraryId === libId && filesAdded > 0) {
                const tracker = _scanTracker[libId] || { lastAdded: 0, lastFetchTime: 0 };
                const now = Date.now();
                if (filesAdded > tracker.lastAdded && (now - tracker.lastFetchTime) >= 2000) {
                    tracker.lastFetchTime = now;
                    const prevAdded = tracker.lastAdded;
                    tracker.lastAdded = filesAdded;
                    _scanTracker[libId] = tracker;
                    if (typeof appendNewScanItems === 'function') {
                        appendNewScanItems(libId, filesAdded - prevAdded);
                    }
                } else if (filesAdded > tracker.lastAdded) {
                    tracker.lastAdded = filesAdded;
                    _scanTracker[libId] = tracker;
                }
            }
            break;
        }
        case 'scan:complete': {
            const r = msg.data?.result;
            toast(`Scan complete: ${r?.files_added||0} added, ${r?.files_found||0} found`);
            loadSidebarCounts();
            const libId = msg.data?.library_id;
            if (libId) {
                delete _scanTracker[libId];
                const fill = document.getElementById('scan-fill-' + libId);
                if (fill) fill.style.width = '100%';
                const btn = document.getElementById('scan-btn-' + libId);
                if (btn) { btn.disabled = false; btn.innerHTML = '&#128269; Scan'; }
                setTimeout(() => {
                    const prog = document.getElementById('scan-progress-' + libId);
                    if (prog) prog.classList.remove('active');
                    if (fill) fill.style.width = '0%';
                }, 3000);
                loadLibrariesView();
                // Reload the grid if the user is viewing this library for final sort/count
                if (typeof _gridState !== 'undefined' && _gridState.libraryId === libId) {
                    if (typeof reloadLibraryGrid === 'function') reloadLibraryGrid();
                }
            }
            break;
        }
        case 'task:update':
            handleTaskUpdate(msg.data);
            break;
        case 'job:progress': break;
        default:
            // Handle sync events (P12-01)
            if (msg.event && msg.event.startsWith('sync:')) {
                handleSyncWSMessage(msg.event, msg.data);
            }
            break;
    }
}
