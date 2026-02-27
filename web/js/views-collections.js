// Collections views
async function loadCollectionsView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Collections</h2>
        <div><button class="btn-secondary" onclick="createCollectionTemplates()" title="Create smart collection presets">+ Templates</button>
        <button class="btn-primary" onclick="showCreateCollection()">+ New</button></div></div>
        <div id="collList"><div class="spinner"></div></div>`;
    const data = await api('GET', '/collections');
    const div = document.getElementById('collList');
    const collections = (data.success && data.data) ? data.data : [];
    if (collections.length > 0) {
        // Show top-level collections (no parent) first
        const topLevel = collections.filter(c => !c.parent_collection_id);
        const nested = collections.filter(c => c.parent_collection_id);
        const renderCollCard = c => {
            const childBadge = c.child_count > 0 ? `<span class="tag tag-blue">${c.child_count} sub</span>` : '';
            const typeBadge = c.collection_type === 'smart' ? '<span class="tag tag-purple">Smart</span>' : '';
            return `<div class="group-card"><div style="display:flex;justify-content:space-between;align-items:flex-start;">
                <div style="cursor:pointer;flex:1;" onclick="navigate('collection','${c.id}')">
                    <h4>${c.name}</h4>
                    ${c.description ? '<p style="color:var(--text-muted);font-size:var(--text-sm);">' + c.description + '</p>' : ''}
                    <span class="tag tag-green">${c.item_count || 0} items</span>
                    <span class="tag tag-cyan">${c.visibility}</span>
                    ${typeBadge}${childBadge}
                </div>
                <button class="btn-danger btn-small" onclick="event.stopPropagation();deleteCollection('${c.id}')">Delete</button>
            </div></div>`;
        };
        let html = topLevel.map(renderCollCard).join('');
        if (nested.length > 0) {
            html += '<h3 style="margin-top:20px;color:var(--text-secondary);">Nested Collections</h3>';
            html += nested.map(renderCollCard).join('');
        }
        div.innerHTML = html;
    } else {
        div.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#11088;</div><div class="empty-state-title">No collections</div><p>Create a collection or use templates to get started</p></div>';
    }
}

let _createCollectionLibId = null;

// ──── Reusable Rule Picker ────
const _rulePickerState = {};
let _rulePickerDebounce = null;

function initRulePicker(fieldId, options, searchFn) {
    _rulePickerState[fieldId] = { selected: [], options: options || [], searchFn: searchFn || null };
    const wrap = document.getElementById(fieldId);
    if (!wrap) return;
    _renderPickerDropdown(fieldId);
    _renderPickerChips(fieldId);
}

function _renderPickerDropdown(fieldId) {
    const wrap = document.getElementById(fieldId);
    const input = wrap.querySelector('.rule-picker-input');
    const dd = wrap.querySelector('.rule-picker-dropdown');
    if (!input || !dd) return;
    const st = _rulePickerState[fieldId];
    const q = input.value.toLowerCase();
    const filtered = st.options.filter(o => {
        if (st.selected.includes(o)) return false;
        return !q || o.toLowerCase().includes(q);
    });
    if (filtered.length === 0) {
        dd.innerHTML = '<div class="rule-picker-empty">' + (q ? 'No matches' : 'No options available') + '</div>';
    } else {
        dd.innerHTML = filtered.map(o =>
            `<div class="rule-picker-opt" onmousedown="rulePickerSelect('${fieldId}',this)" data-val="${escFilterHtml(o)}">${escFilterHtml(o)}</div>`
        ).join('');
    }
}

function rulePickerSelect(fieldId, el) {
    const val = el.dataset.val;
    const st = _rulePickerState[fieldId];
    if (!st.selected.includes(val)) {
        st.selected.push(val);
    }
    const wrap = document.getElementById(fieldId);
    const input = wrap.querySelector('.rule-picker-input');
    input.value = '';
    _renderPickerChips(fieldId);
    _renderPickerDropdown(fieldId);
}

function rulePickerRemove(fieldId, val) {
    const st = _rulePickerState[fieldId];
    st.selected = st.selected.filter(v => v !== val);
    _renderPickerChips(fieldId);
    _renderPickerDropdown(fieldId);
}

function _renderPickerChips(fieldId) {
    const wrap = document.getElementById(fieldId);
    const chipsEl = wrap.querySelector('.rule-picker-chips');
    if (!chipsEl) return;
    const st = _rulePickerState[fieldId];
    chipsEl.innerHTML = st.selected.map(v =>
        `<div class="rule-picker-chip">${escFilterHtml(v)}<span class="rule-picker-chip-x" onclick="rulePickerRemove('${fieldId}','${v.replace(/'/g,"\\'")}')">&times;</span></div>`
    ).join('');
}

function rulePickerInputHandler(fieldId) {
    const st = _rulePickerState[fieldId];
    if (st.searchFn) {
        clearTimeout(_rulePickerDebounce);
        _rulePickerDebounce = setTimeout(async () => {
            const wrap = document.getElementById(fieldId);
            const input = wrap.querySelector('.rule-picker-input');
            const q = input.value.trim();
            if (q.length < 2) { st.options = []; _renderPickerDropdown(fieldId); return; }
            const results = await st.searchFn(q);
            st.options = results;
            _renderPickerDropdown(fieldId);
        }, 300);
    } else {
        _renderPickerDropdown(fieldId);
    }
}

function rulePickerFocus(fieldId) {
    const wrap = document.getElementById(fieldId);
    const dd = wrap.querySelector('.rule-picker-dropdown');
    _renderPickerDropdown(fieldId);
    dd.classList.add('open');
}

function rulePickerBlur(fieldId) {
    setTimeout(() => {
        const wrap = document.getElementById(fieldId);
        if (!wrap) return;
        const dd = wrap.querySelector('.rule-picker-dropdown');
        if (dd) dd.classList.remove('open');
    }, 200);
}

function rulePickerKeydown(fieldId, e) {
    if (e.key === 'Enter') {
        e.preventDefault();
        const wrap = document.getElementById(fieldId);
        const input = wrap.querySelector('.rule-picker-input');
        const val = input.value.trim();
        if (!val) return;
        const st = _rulePickerState[fieldId];
        if (!st.selected.includes(val)) { st.selected.push(val); }
        input.value = '';
        _renderPickerChips(fieldId);
        _renderPickerDropdown(fieldId);
    }
}

function buildPickerHtml(fieldId, placeholder, isSearch) {
    return `<div class="rule-picker" id="${fieldId}">
        <input type="text" class="rule-picker-input" placeholder="${placeholder}"
            onfocus="rulePickerFocus('${fieldId}')" onblur="rulePickerBlur('${fieldId}')"
            oninput="rulePickerInputHandler('${fieldId}')"
            onkeydown="rulePickerKeydown('${fieldId}', event)">
        <div class="rule-picker-dropdown"></div>
        <div class="rule-picker-chips"></div>
    </div>`;
}

function showCreateCollection(parentId, libraryId) {
    _createCollectionLibId = libraryId || null;
    _smartPickersInitialized = false;
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">New Collection</h2></div>
    <div style="max-width:700px;">
        <div class="form-group"><label>Collection Type</label>
            <div style="display:flex;gap:10px;">
                <button class="btn-primary" id="collTypeManualBtn" onclick="setCollType('manual')">Manual</button>
                <button class="btn-secondary" id="collTypeSmartBtn" onclick="setCollType('smart')">Smart</button>
            </div>
        </div>
        <input type="hidden" id="collType" value="manual">
        <input type="hidden" id="collParentId" value="${parentId || ''}">
        <div class="form-group"><label>Name</label><input type="text" id="collName" placeholder="e.g. My Favorites"></div>
        <div class="form-group"><label>Description</label><input type="text" id="collDesc" placeholder="Optional description"></div>
        <div class="form-group"><label>Visibility</label>
            <select id="collVis"><option value="private">Private</option><option value="shared">Shared</option><option value="public">Public</option></select>
        </div>
        <div class="form-group"><label>Sort Mode</label>
            <select id="collSortMode">
                <option value="custom">Custom Order</option>
                <option value="title">Title</option>
                <option value="year">Year</option>
                <option value="rating">Rating</option>
                <option value="added">Recently Added</option>
                <option value="duration">Duration</option>
            </select>
        </div>
        <div id="smartRulesSection" style="display:none;">
            <h3 style="margin:20px 0 12px;color:var(--text-primary);">Smart Collection Rules</h3>
            <div class="smart-rule-builder">
                <div class="rule-group"><label>Genres</label>
                    ${buildPickerHtml('ruleGenres', 'Search genres...')}
                </div>
                <div class="rule-group"><label>Exclude Genres</label>
                    ${buildPickerHtml('ruleExcludeGenres', 'Search genres to exclude...')}
                </div>
                <div class="rule-group"><label>Moods</label>
                    ${buildPickerHtml('ruleMoods', 'Search moods...')}
                </div>
                <div class="rule-group"><label>Performers</label>
                    ${buildPickerHtml('rulePerformers', 'Type to search performers...', true)}
                </div>
                <div class="rule-group"><label>Studios</label>
                    ${buildPickerHtml('ruleStudios', 'Search studios...')}
                </div>
                <div class="rule-group"><label>Keywords</label>
                    ${buildPickerHtml('ruleKeywords', 'Type keyword and press Enter')}
                </div>
                <div class="rule-group"><label>Year Range</label>
                    <div class="rule-row">
                        <input type="number" id="ruleYearFrom" placeholder="From">
                        <span style="color:var(--text-muted);">to</span>
                        <input type="number" id="ruleYearTo" placeholder="To">
                    </div>
                </div>
                <div class="rule-group"><label>Min Rating (0-10)</label>
                    <input type="number" id="ruleMinRating" placeholder="e.g. 7.0" step="0.1" min="0" max="10">
                </div>
                <div class="rule-group"><label>Duration Range (minutes)</label>
                    <div class="rule-row">
                        <input type="number" id="ruleMinDuration" placeholder="Min">
                        <span style="color:var(--text-muted);">to</span>
                        <input type="number" id="ruleMaxDuration" placeholder="Max">
                    </div>
                </div>
                <div class="rule-group"><label>Added Within (days)</label>
                    <input type="number" id="ruleAddedWithin" placeholder="e.g. 30">
                </div>
                <div class="rule-group"><label>Content Rating</label>
                    ${buildPickerHtml('ruleContentRating', 'Select content rating...')}
                </div>
                <div class="rule-group"><label>Sort By</label>
                    <select id="ruleSortBy">
                        <option value="">Default (Rating)</option>
                        <option value="title">Title</option>
                        <option value="year">Year</option>
                        <option value="rating">Rating</option>
                        <option value="added">Recently Added</option>
                        <option value="duration">Duration</option>
                        <option value="random">Random</option>
                    </select>
                </div>
                <div class="rule-group"><label>Sort Order</label>
                    <select id="ruleSortOrder">
                        <option value="desc">Descending</option>
                        <option value="asc">Ascending</option>
                    </select>
                </div>
                <div class="rule-group"><label>Max Results</label>
                    <input type="number" id="ruleMaxResults" placeholder="100" min="1" max="500">
                </div>
            </div>
            <button class="btn-secondary" onclick="previewSmartRules()">Preview Matches</button>
            <div id="smartPreviewResult"></div>
        </div>
        <div style="margin-top:20px;">
            <button class="btn-primary" onclick="createCollection()">Create Collection</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="cancelCreateCollection()">Cancel</button>
        </div>
    </div>`;
}

let _smartPickersInitialized = false;
function setCollType(type) {
    document.getElementById('collType').value = type;
    document.getElementById('smartRulesSection').style.display = type === 'smart' ? 'block' : 'none';
    document.getElementById('collTypeManualBtn').className = type === 'manual' ? 'btn-primary' : 'btn-secondary';
    document.getElementById('collTypeSmartBtn').className = type === 'smart' ? 'btn-primary' : 'btn-secondary';
    if (type === 'smart' && !_smartPickersInitialized) {
        _smartPickersInitialized = true;
        initSmartPickers();
    }
}

async function initSmartPickers() {
    let genres = [], contentRatings = [], moods = [], studios = [];
    // Fetch library-scoped filter options (genres, content ratings)
    if (_createCollectionLibId) {
        const filterData = await api('GET', '/libraries/' + _createCollectionLibId + '/filters');
        if (filterData.success && filterData.data) {
            genres = filterData.data.genres || [];
            contentRatings = filterData.data.content_ratings || [];
        }
    }
    // Fetch moods from tags
    const moodData = await api('GET', '/tags?category=mood');
    if (moodData.success && moodData.data) {
        moods = moodData.data.map(t => t.name);
    }
    // Fetch studios
    const studioData = await api('GET', '/studios?limit=500');
    if (studioData.success && studioData.data) {
        studios = studioData.data.map(s => s.name);
    }
    // Initialize pickers with fetched data
    initRulePicker('ruleGenres', genres);
    initRulePicker('ruleExcludeGenres', genres);
    initRulePicker('ruleMoods', moods);
    initRulePicker('ruleStudios', studios);
    initRulePicker('ruleContentRating', contentRatings);
    initRulePicker('ruleKeywords', []);  // free-text only
    // Performers: search-as-you-type
    initRulePicker('rulePerformers', [], async (q) => {
        const res = await api('GET', '/performers?q=' + encodeURIComponent(q) + '&limit=50');
        return (res.success && res.data) ? res.data.map(p => p.name) : [];
    });
}

function buildSmartRules() {
    const pickerList = id => (_rulePickerState[id] && _rulePickerState[id].selected.length > 0) ? [..._rulePickerState[id].selected] : [];
    const parseNum = id => { const v = document.getElementById(id).value.trim(); return v ? Number(v) : null; };
    const rules = {};
    const genres = pickerList('ruleGenres'); if (genres.length) rules.genres = genres;
    const excludeGenres = pickerList('ruleExcludeGenres'); if (excludeGenres.length) rules.exclude_genres = excludeGenres;
    const moods = pickerList('ruleMoods'); if (moods.length) rules.moods = moods;
    const performers = pickerList('rulePerformers'); if (performers.length) rules.performers = performers;
    const studios = pickerList('ruleStudios'); if (studios.length) rules.studios = studios;
    const keywords = pickerList('ruleKeywords'); if (keywords.length) rules.keywords = keywords;
    const contentRating = pickerList('ruleContentRating'); if (contentRating.length) rules.content_rating = contentRating;
    const yf = parseNum('ruleYearFrom'); if (yf !== null) rules.year_from = yf;
    const yt = parseNum('ruleYearTo'); if (yt !== null) rules.year_to = yt;
    const mr = parseNum('ruleMinRating'); if (mr !== null) rules.min_rating = mr;
    const minD = parseNum('ruleMinDuration'); if (minD !== null) rules.min_duration = minD;
    const maxD = parseNum('ruleMaxDuration'); if (maxD !== null) rules.max_duration = maxD;
    const aw = parseNum('ruleAddedWithin'); if (aw !== null) rules.added_within = aw;
    const sb = document.getElementById('ruleSortBy').value; if (sb) rules.sort_by = sb;
    const so = document.getElementById('ruleSortOrder').value; if (so) rules.sort_order = so;
    const maxR = parseNum('ruleMaxResults'); if (maxR !== null) rules.max_results = maxR;
    return rules;
}

async function previewSmartRules() {
    const rules = buildSmartRules();
    if (Object.keys(rules).length === 0) { toast('Add at least one rule', 'error'); return; }
    // Create a temporary smart collection to preview
    const tempRules = JSON.stringify(rules);
    const res = await api('POST', '/collections', { name: '__preview_temp_' + Date.now(), collection_type: 'smart', rules: tempRules, visibility: 'private' });
    if (!res.success) { document.getElementById('smartPreviewResult').innerHTML = '<div class="smart-preview-count">Error creating preview</div>'; return; }
    const tempId = res.data.id;
    const evalRes = await api('GET', '/collections/' + tempId + '/evaluate');
    await api('DELETE', '/collections/' + tempId);
    const count = (evalRes.success && evalRes.data) ? evalRes.data.length : 0;
    document.getElementById('smartPreviewResult').innerHTML = `<div class="smart-preview-count">${count} matching item${count !== 1 ? 's' : ''} found</div>`;
}

async function createCollection() {
    const name = document.getElementById('collName').value;
    const desc = document.getElementById('collDesc').value || null;
    const vis = document.getElementById('collVis').value;
    const sortMode = document.getElementById('collSortMode').value;
    const collType = document.getElementById('collType').value;
    const parentId = document.getElementById('collParentId').value || null;
    if (!name) { toast('Name required', 'error'); return; }

    const body = { name: name, description: desc, visibility: vis, collection_type: collType, item_sort_mode: sortMode };
    if (parentId) body.parent_collection_id = parentId;
    if (_createCollectionLibId) body.library_id = _createCollectionLibId;

    if (collType === 'smart') {
        const rules = buildSmartRules();
        if (Object.keys(rules).length === 0) { toast('Smart collections need at least one rule', 'error'); return; }
        body.rules = JSON.stringify(rules);
    }

    const r = await api('POST', '/collections', body);
    if (r.success) {
        toast('Collection created!');
        if (_createCollectionLibId) {
            // Return to library view with collections tab active
            await loadLibraryView(_createCollectionLibId);
            _createCollectionLibId = null;
            showLibraryCollections();
        } else {
            loadCollectionsView();
        }
    }
    else toast(r.error, 'error');
}

async function cancelCreateCollection() {
    if (_createCollectionLibId) {
        await loadLibraryView(_createCollectionLibId);
        _createCollectionLibId = null;
        showLibraryCollections();
    } else {
        loadCollectionsView();
    }
}

async function deleteCollection(id) {
    if (!confirm('Delete this collection?')) return;
    const d = await api('DELETE', '/collections/' + id);
    if (d.success) { toast('Deleted'); loadCollectionsView(); }
    else toast(d.error, 'error');
}

async function editCollection(id) {
    const res = await api('GET', '/collections/' + id);
    if (!res.success) { toast('Failed to load collection', 'error'); return; }
    const coll = res.data;
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Collection</h2></div>
        <div style="max-width:560px;">
            <div class="form-group"><label>Name</label><input type="text" id="editCollName" value="${coll.name || ''}"></div>
            <div class="form-group"><label>Description</label><textarea id="editCollDesc" rows="3">${coll.description || ''}</textarea></div>
            <div class="form-group"><label>Visibility</label>
                <select id="editCollVisibility">
                    <option value="public" ${coll.visibility==='public'?'selected':''}>Public</option>
                    <option value="private" ${coll.visibility==='private'?'selected':''}>Private</option>
                    <option value="shared" ${coll.visibility==='shared'?'selected':''}>Shared</option>
                </select>
            </div>
            <div class="form-group"><label>Sort Mode</label>
                <select id="editCollSort">
                    <option value="title" ${coll.sort_mode==='title'?'selected':''}>Title</option>
                    <option value="year" ${coll.sort_mode==='year'?'selected':''}>Year</option>
                    <option value="added_at" ${coll.sort_mode==='added_at'?'selected':''}>Date Added</option>
                    <option value="custom" ${coll.sort_mode==='custom'?'selected':''}>Custom Order</option>
                </select>
            </div>
            <div class="form-group"><label>Poster URL</label><input type="text" id="editCollPoster" value="${coll.poster_path || ''}" placeholder="Optional poster image"></div>
            <button class="btn-primary" onclick="saveCollectionEdit('${id}')">Save Changes</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadCollectionDetailView('${id}')">Cancel</button>
        </div>`;
}

async function saveCollectionEdit(id) {
    const name = document.getElementById('editCollName').value.trim();
    if (!name) { toast('Name is required', 'error'); return; }
    const d = await api('PUT', '/collections/' + id, {
        name,
        description: document.getElementById('editCollDesc').value.trim() || null,
        visibility: document.getElementById('editCollVisibility').value,
        sort_mode: document.getElementById('editCollSort').value,
        poster_path: document.getElementById('editCollPoster').value.trim() || null
    });
    if (d.success) { toast('Collection updated!'); loadCollectionDetailView(id); }
    else toast(d.error, 'error');
}

async function createCollectionTemplates(libraryId) {
    const r = await api('POST', '/collections/templates');
    if (r.success) {
        toast(`${r.data.count} template collection(s) created`);
        if (libraryId) {
            showLibraryCollections();
        } else {
            loadCollectionsView();
        }
    }
    else toast(r.error || 'Failed to create templates', 'error');
}

// ──── Add to Collection Picker ────
async function showAddToCollectionPicker(mediaId) {
    const collRes = await api('GET', '/collections');
    const manualColls = (collRes.success && collRes.data) ? collRes.data.filter(c => c.collection_type === 'manual') : [];
    const options = manualColls.map(c => `<option value="${c.id}">${c.name} (${c.item_count || 0} items)</option>`).join('');
    // Show inline picker below detail actions
    const existing = document.getElementById('collPickerInline');
    if (existing) existing.remove();
    const picker = document.createElement('div');
    picker.id = 'collPickerInline';
    picker.style.cssText = 'display:flex;flex-direction:column;gap:8px;margin-top:10px;padding:12px;background:rgba(0,0,0,0.3);border:1px solid var(--accent-border);border-radius:10px;';
    picker.innerHTML = `
        <div style="display:flex;gap:8px;align-items:center;">
            <input type="text" id="collPickerNewName" placeholder="New collection name..." style="flex:1;">
            <button class="btn-primary btn-small" onclick="detailCreateAndAdd('${mediaId}')">Create & Add</button>
        </div>
        ${manualColls.length > 0 ? `<div style="display:flex;gap:8px;align-items:center;">
            <select id="collPickerSelect" style="flex:1;">${options}</select>
            <button class="btn-primary btn-small" onclick="addToCollection('${mediaId}')">Add</button>
        </div>` : '<div style="color:var(--text-tertiary);font-size:0.82rem;">No existing collections — create one above</div>'}
        <div style="display:flex;justify-content:flex-end;">
            <button class="btn-secondary btn-small" onclick="this.closest(\'#collPickerInline\').remove()">Cancel</button>
        </div>`;
    const actions = document.querySelector('.detail-actions');
    if (actions) actions.parentElement.insertBefore(picker, actions.nextSibling);
}

async function detailCreateAndAdd(mediaId) {
    const input = document.getElementById('collPickerNewName');
    const name = input.value.trim();
    if (!name) { toast('Enter a collection name', 'error'); return; }
    // Get library_id from the media item so the collection is scoped to its library
    const body = { name, collection_type: 'manual', visibility: 'private' };
    const mediaRes = await api('GET', '/media/' + mediaId);
    if (mediaRes.success && mediaRes.data && mediaRes.data.library_id) {
        body.library_id = mediaRes.data.library_id;
    }
    const res = await api('POST', '/collections', body);
    if (!res.success) { toast(res.error || 'Failed to create collection', 'error'); return; }
    const collId = res.data && res.data.id;
    if (collId) {
        const addRes = await api('POST', '/collections/' + collId + '/items', { media_item_id: mediaId });
        if (addRes.success) {
            toast('Created "' + name + '" and added item');
            const picker = document.getElementById('collPickerInline');
            if (picker) picker.remove();
        } else {
            toast('Collection created but failed to add: ' + (addRes.error || ''), 'error');
        }
    }
}

async function addToCollection(mediaId) {
    const sel = document.getElementById('collPickerSelect');
    if (!sel) return;
    const collId = sel.value;
    const d = await api('POST', '/collections/' + collId + '/items', { media_item_id: mediaId });
    if (d.success) {
        toast('Added to collection!');
        const picker = document.getElementById('collPickerInline');
        if (picker) picker.remove();
    } else {
        toast(d.error || 'Failed to add', 'error');
    }
}

async function removeFromCollection(collId, itemId) {
    const d = await api('DELETE', '/collections/' + collId + '/items/' + itemId);
    if (d.success) { toast('Removed from collection'); loadCollectionDetailView(collId); }
    else toast(d.error || 'Failed to remove', 'error');
}

function formatRuntime(seconds) {
    if (!seconds) return '0m';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return h > 0 ? h + 'h ' + m + 'm' : m + 'm';
}

async function loadCollectionDetailView(collId) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div>';

    const [collRes, statsRes, childRes] = await Promise.all([
        api('GET', '/collections/' + collId),
        api('GET', '/collections/' + collId + '/stats'),
        api('GET', '/collections/' + collId + '/children')
    ]);

    if (!collRes.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Collection not found</div></div>'; return; }
    const coll = collRes.data;
    const stats = (statsRes.success && statsRes.data) ? statsRes.data : null;
    const children = (childRes.success && childRes.data) ? childRes.data : [];

    // Build breadcrumb
    let breadcrumb = `<div class="collection-breadcrumb">
        <a onclick="navigate('collections')">Collections</a>
        <span class="cb-sep">&#8250;</span>
        <span>${coll.name}</span>
    </div>`;

    // Header
    const poster = coll.poster_path
        ? `<img src="${posterSrc(coll.poster_path, coll.updated_at)}" alt="">`
        : '&#128218;';
    const typeBadge = coll.collection_type === 'smart' ? '<span class="tag tag-purple">Smart</span>' : '<span class="tag tag-blue">Manual</span>';

    let headerHTML = `<div class="collection-detail-header">
        <div class="collection-detail-poster">${poster}</div>
        <div class="collection-detail-info">
            <h1>${coll.name}</h1>
            ${coll.description ? '<div class="cd-desc">' + coll.description + '</div>' : ''}
            <div class="cd-badges">
                ${typeBadge}
                <span class="tag tag-cyan">${coll.visibility}</span>
                <span class="tag tag-green">${coll.item_count || 0} items</span>
            </div>
            <div style="display:flex;gap:8px;margin-top:12px;flex-wrap:wrap;">
                <button class="btn-secondary btn-small" onclick="editCollection('${coll.id}')">&#9998; Edit</button>
                <button class="btn-secondary btn-small" onclick="openCollectionArtworkPicker('${coll.id}','poster')">&#128444; Poster</button>
                <button class="btn-secondary btn-small" onclick="openCollectionArtworkPicker('${coll.id}','backdrop')">&#127756; Backdrop</button>
                <button class="btn-secondary btn-small" onclick="showCreateCollection('${coll.id}')">+ Sub-collection</button>
                <button class="btn-danger btn-small" onclick="deleteCollection('${coll.id}');navigate('collections');">Delete</button>
            </div>
        </div>
    </div>`;

    // Stats bar
    let statsHTML = '';
    if (stats && stats.total_items > 0) {
        const genreChips = (stats.genres || []).slice(0, 6).map(g => `<span class="tag">${g.name} (${g.count})</span>`).join('');
        statsHTML = `<div class="collection-stats-bar">
            <div class="collection-stat"><span class="cs-value">${stats.total_items}</span><span class="cs-label">Items</span></div>
            <div class="collection-stat"><span class="cs-value">${formatRuntime(stats.total_runtime_seconds)}</span><span class="cs-label">Runtime</span></div>
            <div class="collection-stat"><span class="cs-value">${stats.avg_rating ? stats.avg_rating.toFixed(1) : '-'}</span><span class="cs-label">Avg Rating</span></div>
            ${genreChips ? '<div class="collection-stat-genres">' + genreChips + '</div>' : ''}
        </div>`;
    }

    // Sub-collections
    let childHTML = '';
    if (children.length > 0) {
        childHTML = `<div class="sub-collections-section"><h3>Sub-collections</h3>
            <div class="collection-grid">${children.map(c => {
                const cp = c.poster_path ? `<img src="${posterSrc(c.poster_path, c.updated_at)}" alt="">` : '&#128218;';
                return `<div class="collection-card" onclick="navigate('collection','${c.id}')">
                    <div class="cc-poster">${cp}</div>
                    <div class="cc-info">
                        <div class="cc-name">${c.name}</div>
                        <div class="cc-meta">${c.item_count || 0} item${(c.item_count||0) !== 1 ? 's' : ''}</div>
                    </div>
                </div>`;
            }).join('')}</div></div>`;
    }

    // Sort bar
    const sortOptions = ['custom','title','year','rating','added','duration'];
    const sortLabels = { custom:'Custom', title:'Title', year:'Year', rating:'Rating', added:'Added', duration:'Duration' };
    let sortHTML = `<div class="collection-sort-bar">
        <div class="csb-left"><span style="color:var(--text-muted);font-size:var(--text-sm);">Sort:</span>
            <select id="collDetailSort" onchange="changeCollectionSort('${coll.id}', this.value)">
                ${sortOptions.map(o => `<option value="${o}" ${coll.item_sort_mode === o ? 'selected' : ''}>${sortLabels[o]}</option>`).join('')}
            </select>
        </div>
        <div class="csb-right">
            <button class="btn-secondary btn-small" onclick="navigate('collections')">&#8592; Back</button>
        </div>
    </div>`;

    // Items grid
    let itemsHTML = '';
    if (coll.collection_type === 'smart') {
        // Evaluate smart collection
        const evalRes = await api('GET', '/collections/' + collId + '/evaluate');
        const items = (evalRes.success && evalRes.data) ? evalRes.data : [];
        if (items.length > 0) {
            itemsHTML = '<div class="media-grid">' + items.map(renderMediaCard).join('') + '</div>';
        } else {
            itemsHTML = '<div class="empty-state"><div class="empty-state-icon">&#128269;</div><div class="empty-state-title">No matches</div><p>No items match the current smart rules</p></div>';
        }
    } else {
        // Manual collection items with joined metadata
        const items = coll.items || [];
        if (items.length > 0) {
            itemsHTML = '<div class="media-grid">' + items.map(ci => {
                // Render collection item as a media card using joined metadata
                const dur = ci.duration_seconds ? Math.floor(ci.duration_seconds / 60) + 'min' : '';
                const year = ci.year || '';
                const res = ci.resolution || '';
                const meta = [year, dur, res].filter(Boolean).join(' \u00b7 ');
                const itemId = ci.media_item_id || ci.tv_show_id || ci.album_id || ci.book_id || '';
                const clickAction = ci.media_item_id ? `loadMediaDetail('${ci.media_item_id}')` : (ci.tv_show_id ? `loadShowView('${ci.tv_show_id}')` : '');
                return `<div class="media-card" tabindex="0" onclick="${clickAction}">
                    <div class="media-poster" style="position:relative;">
                        ${ci.poster_path ? '<img src="' + posterSrc(ci.poster_path, coll.updated_at) + '" alt="" loading="lazy">' : '&#127916;'}
                        ${renderOverlayBadges(ci)}
                        <button class="cw-remove" onclick="event.stopPropagation();removeFromCollection('${collId}','${ci.id}')" title="Remove from collection">&#10005;</button>
                        <div class="media-card-hover-info">
                            <div class="hover-title">${ci.title || 'Untitled'}</div>
                            <div class="hover-meta">${ci.rating ? '<span class="hover-rating-badge">' + ratingIcon('tmdb', ci.rating) + ' ' + ci.rating.toFixed(1) + '</span>' : ''}<span>${meta}</span></div>
                        </div>
                        <div class="play-overlay"><div class="play-button">&#9654;</div></div>
                    </div>
                    <div class="media-info"><div class="media-title">${ci.title || 'Untitled'}</div><div class="media-meta">${meta}</div></div>
                </div>`;
            }).join('') + '</div>';
        } else {
            itemsHTML = '<div class="empty-state"><div class="empty-state-icon">&#128218;</div><div class="empty-state-title">Empty collection</div><p>Add items from media detail pages using the + Collection button</p></div>';
        }
    }

    mc.innerHTML = breadcrumb + headerHTML + statsHTML + childHTML + sortHTML + itemsHTML;
}

async function changeCollectionSort(collId, sortMode) {
    await api('PUT', '/collections/' + collId, { item_sort_mode: sortMode });
    loadCollectionDetailView(collId);
}

// ──── Performers ────
