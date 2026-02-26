// Edit modal and metadata editing
function toggleKnownEdition(el) {
    const idx = el.getAttribute('data-idx');
    const detail = document.getElementById('knownEdDetail_' + idx);
    if (!detail) return;
    if (detail.style.display === 'none') {
        detail.style.display = 'block';
        el.textContent = 'Hide details';
    } else {
        detail.style.display = 'none';
        el.textContent = 'Show details';
    }
}

// ──── Edit Media ────
let _editMediaItem = null;
let _editArtworkLoaded = false;
const VIDEO_TYPES_TABBED = ['movies', 'adult_movies', 'home_videos', 'other_videos'];

async function openEditModal(id) {
    const data = await api('GET', '/media/' + id);
    if (!data.success) { toast('Failed to load media', 'error'); return; }
    const m = data.data;
    _editMediaItem = m;
    _editArtworkLoaded = false;

    document.getElementById('editMediaId').value = m.id;
    document.getElementById('editTitle').value = m.title || '';
    document.getElementById('editSortTitle').value = m.sort_title || '';
    document.getElementById('editOriginalTitle').value = m.original_title || '';
    document.getElementById('editDescription').value = m.description || '';
    document.getElementById('editYear').value = m.year || '';
    document.getElementById('editReleaseDate').value = m.release_date ? m.release_date.substring(0,10) : '';
    document.getElementById('editRating').value = m.rating != null ? m.rating : '';
    document.getElementById('editEditionType').value = m.edition_type || 'Theatrical';
    document.getElementById('editContentRating').value = m.content_rating || '';
    document.getElementById('editOriginalLanguage').value = m.original_language || '';
    document.getElementById('editTagline').value = m.tagline || '';
    document.getElementById('editCountry').value = m.country || '';
    document.getElementById('editTrailerUrl').value = m.trailer_url || '';
    document.getElementById('editGenres').value = (m.genres || []).join(', ');
    document.getElementById('editStudio').value = m.studio || '';
    document.getElementById('editPosterPath').value = m.poster_path || '';
    document.getElementById('editBackdropPath').value = m.backdrop_path || '';
    document.getElementById('editCustomNotes').value = m.custom_notes || '';
    const ctRaw = m.custom_tags ? (typeof m.custom_tags === 'string' ? JSON.parse(m.custom_tags || '{}') : m.custom_tags) : {};
    const ctArr = ctRaw.tags || [];
    document.getElementById('editCustomTags').value = ctArr.join(', ');

    // Cast display
    document.getElementById('editCastPanel').style.display = 'none';
    document.getElementById('editCastToggle').classList.remove('open');
    loadEditCast(m.id);

    // Parent Movie (movies and adult_movies only)
    const parentSearch = document.getElementById('parentMovieSearch');
    const parentIdField = document.getElementById('parentMovieId');
    const parentInfo = document.getElementById('parentCurrentInfo');
    parentSearch.value = '';
    parentIdField.value = '';
    parentInfo.style.display = 'none';
    parentInfo.innerHTML = '';
    document.getElementById('parentSearchResults').classList.remove('active');
    const isMovieType = m.media_type === 'movies' || m.media_type === 'adult_movies';
    if (isMovieType) {
        const edRes = await api('GET', '/media/' + id + '/editions');
        if (edRes.success && edRes.data.has_editions) {
            const eds = edRes.data.editions || [];
            const parent = eds.find(e => e.is_default);
            if (parent && parent.media_item_id !== m.id) {
                parentInfo.style.display = '';
                parentInfo.innerHTML = `<div class="parent-current"><span class="pc-title">&#128279; Parent: ${parent.title} (${parent.edition_type})</span><button class="pc-remove" onclick="removeEditionParent('${m.id}')">Remove</button></div>`;
            } else if (parent && parent.media_item_id === m.id && eds.length > 1) {
                parentInfo.style.display = '';
                parentInfo.innerHTML = `<div class="parent-current"><span class="pc-title">&#11088; This is the parent movie (${eds.length} editions)</span></div>`;
            }
        }
    }

    // Series info (movies and adult_movies only)
    const seriesGroup = document.getElementById('editSeriesGroup');
    const seriesInfo = document.getElementById('editSeriesInfo');
    seriesInfo.innerHTML = '';
    if (isMovieType) {
        seriesGroup.style.display = '';
        loadEditSeriesInfo(m.id);
    } else {
        seriesGroup.style.display = 'none';
    }

    // Lock status & per-field locks
    const lockedFields = m.locked_fields || [];
    renderFieldLocks(lockedFields);
    const lockDiv = document.getElementById('editLockStatus');
    const resetBtn = document.getElementById('editResetBtn');
    if (m.metadata_locked || lockedFields.includes('*')) {
        lockDiv.innerHTML = '<span class="lock-badge locked">&#128274; All fields locked — auto-match will not overwrite your edits</span>';
        resetBtn.style.display = 'inline-flex';
    } else if (lockedFields.length > 0) {
        lockDiv.innerHTML = '<span class="lock-badge locked">&#128274; ' + lockedFields.length + ' field' + (lockedFields.length > 1 ? 's' : '') + ' locked</span>';
        resetBtn.style.display = 'inline-flex';
    } else {
        lockDiv.innerHTML = '<span class="lock-badge unlocked">&#128275; No fields locked — auto-match may update all fields</span>';
        resetBtn.style.display = 'none';
    }

    // Tab mode for video types (not music_videos)
    const modal = document.getElementById('editModalContainer');
    const useTabs = VIDEO_TYPES_TABBED.includes(m.media_type);
    if (useTabs) {
        modal.classList.add('tabbed');
        document.getElementById('editTabEditionsBtn').style.display = isMovieType ? '' : 'none';
        switchEditTab('basic');
    } else {
        modal.classList.remove('tabbed');
        document.querySelectorAll('.edit-tab-content').forEach(el => el.classList.add('active'));
    }

    // Reset artwork tab
    document.getElementById('artworkTabContent').innerHTML =
        '<div class="artwork-tab-loading" id="artworkTabLoading"><div class="artwork-tab-spinner"></div><span>Loading artwork...</span></div>';

    document.getElementById('editMediaOverlay').classList.add('active');
}

function switchEditTab(tabName) {
    document.querySelectorAll('.edit-tab').forEach(t => t.classList.toggle('active', t.dataset.tab === tabName));
    document.querySelectorAll('.edit-tab-content').forEach(c => c.classList.toggle('active', c.dataset.tabContent === tabName));
    if (tabName === 'artwork' && !_editArtworkLoaded) {
        loadArtworkTab();
    }
}

async function loadArtworkTab() {
    if (!_editMediaItem) return;
    _editArtworkLoaded = true;
    const container = document.getElementById('artworkTabContent');
    const mediaId = _editMediaItem.id;
    const currentPoster = _editMediaItem.poster_path || '';
    const currentBackdrop = _editMediaItem.backdrop_path || '';

    container.innerHTML = '<div class="artwork-tab-loading"><div class="artwork-tab-spinner"></div><span>Loading artwork...</span></div>';

    const res = await api('GET', `/media/${mediaId}/artwork`);
    let posters = [];
    let backdrops = [];
    if (res.success && res.data) {
        posters = res.data.posters || [];
        backdrops = res.data.backdrops || [];
    }

    let html = '';

    // Poster section
    html += '<div class="artwork-section">';
    html += '<div class="artwork-section-header"><h3>Poster</h3>';
    html += '<span class="artwork-section-count">' + (posters.length + (currentPoster ? 1 : 0)) + ' available</span></div>';
    if (currentPoster || posters.length > 0) {
        html += '<div class="artwork-inline-grid">';
        if (currentPoster) {
            const alreadyInList = posters.some(u => u === currentPoster);
            html += buildArtworkThumb(currentPoster, 'poster', true, !alreadyInList);
            posters.forEach(url => {
                if (url !== currentPoster) {
                    html += buildArtworkThumb(url, 'poster', false, false);
                }
            });
        } else {
            posters.forEach((url, i) => {
                html += buildArtworkThumb(url, 'poster', i === 0, false);
            });
        }
        html += '</div>';
    } else {
        html += '<div class="artwork-tab-empty">No poster images available. Use Identify to fetch metadata.</div>';
    }
    html += '<div class="artwork-manual-url">';
    html += '<input type="text" id="manualPosterUrl" placeholder="Or paste a custom poster URL...">';
    html += '<button class="btn-secondary" onclick="applyManualArtwork(\'poster\')">Apply</button>';
    html += '</div></div>';

    // Backdrop section
    html += '<div class="artwork-section">';
    html += '<div class="artwork-section-header"><h3>Background</h3>';
    html += '<span class="artwork-section-count">' + (backdrops.length + (currentBackdrop ? 1 : 0)) + ' available</span></div>';
    if (currentBackdrop || backdrops.length > 0) {
        html += '<div class="artwork-inline-grid backdrop-grid">';
        if (currentBackdrop) {
            const alreadyInList = backdrops.some(u => u === currentBackdrop);
            html += buildArtworkThumb(currentBackdrop, 'backdrop', true, !alreadyInList);
            backdrops.forEach(url => {
                if (url !== currentBackdrop) {
                    html += buildArtworkThumb(url, 'backdrop', false, false);
                }
            });
        } else {
            backdrops.forEach((url, i) => {
                html += buildArtworkThumb(url, 'backdrop', i === 0, false);
            });
        }
        html += '</div>';
    } else {
        html += '<div class="artwork-tab-empty">No background images available. Use Identify to fetch metadata.</div>';
    }
    html += '<div class="artwork-manual-url">';
    html += '<input type="text" id="manualBackdropUrl" placeholder="Or paste a custom background URL...">';
    html += '<button class="btn-secondary" onclick="applyManualArtwork(\'backdrop\')">Apply</button>';
    html += '</div></div>';

    container.innerHTML = html;

    // When no current backdrop/poster exists the first item is shown as visually
    // selected, so register it in the hidden inputs so Save actually picks it up.
    if (!currentBackdrop && backdrops.length > 0) {
        const el = document.getElementById('editBackdropPath');
        if (el && !el.value) el.value = backdrops[0];
    }
    if (!currentPoster && posters.length > 0) {
        const el = document.getElementById('editPosterPath');
        if (el && !el.value) el.value = posters[0];
    }
}

function buildArtworkThumb(url, type, isSelected, isCurrent) {
    const cls = type === 'backdrop' ? 'artwork-inline-thumb backdrop-thumb' : 'artwork-inline-thumb';
    const selCls = isSelected ? ' selected' : '';
    const safeUrl = url.replace(/'/g, "\\'");
    const source = extractArtworkSource(url);
    return `<div class="${cls}${selCls}" onclick="selectInlineArtwork(this,'${type}','${safeUrl}')">
        <img src="${url}" loading="lazy" alt="${type}">
        ${isCurrent ? '<span class="artwork-current-badge">Current</span>' : ''}
        <span class="artwork-check">&#10003;</span>
        <span class="artwork-source-badge">${source}</span>
    </div>`;
}

function selectInlineArtwork(el, type, url) {
    const grid = el.parentElement;
    grid.querySelectorAll('.artwork-inline-thumb').forEach(t => t.classList.remove('selected'));
    el.classList.add('selected');
    if (type === 'poster') {
        document.getElementById('editPosterPath').value = url;
    } else {
        document.getElementById('editBackdropPath').value = url;
    }
}

function applyManualArtwork(type) {
    const inputId = type === 'poster' ? 'manualPosterUrl' : 'manualBackdropUrl';
    const url = document.getElementById(inputId).value.trim();
    if (!url) { toast('Enter a URL first', 'error'); return; }
    if (type === 'poster') {
        document.getElementById('editPosterPath').value = url;
    } else {
        document.getElementById('editBackdropPath').value = url;
    }
    const grid = document.getElementById(inputId).closest('.artwork-section').querySelector('.artwork-inline-grid');
    if (grid) grid.querySelectorAll('.artwork-inline-thumb').forEach(t => t.classList.remove('selected'));
    toast(`Custom ${type} URL applied`);
}

function closeEditModal() {
    document.getElementById('editMediaOverlay').classList.remove('active');
}

async function saveMediaEdit() {
    const id = document.getElementById('editMediaId').value;
    const title = document.getElementById('editTitle').value.trim();
    if (!title) { toast('Title is required', 'error'); return; }
    const sortTitle = document.getElementById('editSortTitle').value.trim() || null;
    const originalTitle = document.getElementById('editOriginalTitle').value.trim() || null;
    const description = document.getElementById('editDescription').value.trim() || null;
    const yearVal = document.getElementById('editYear').value;
    const year = yearVal ? parseInt(yearVal) : null;
    const releaseDate = document.getElementById('editReleaseDate').value || null;
    const ratingVal = document.getElementById('editRating').value;
    const rating = ratingVal !== '' ? parseFloat(ratingVal) : null;

    const editionType = document.getElementById('editEditionType').value;
    const posterPath = document.getElementById('editPosterPath').value.trim() || null;
    const backdropPath = document.getElementById('editBackdropPath').value.trim() || null;
    const customNotes = document.getElementById('editCustomNotes').value.trim() || null;
    const ctInput = document.getElementById('editCustomTags').value.trim();
    const ctTags = ctInput ? ctInput.split(',').map(t => t.trim()).filter(Boolean) : [];
    const customTags = JSON.stringify({ tags: ctTags });

    const contentRating = document.getElementById('editContentRating').value || null;
    const originalLanguage = document.getElementById('editOriginalLanguage').value.trim() || null;
    const tagline = document.getElementById('editTagline').value.trim() || null;
    const country = document.getElementById('editCountry').value.trim() || null;
    const trailerUrl = document.getElementById('editTrailerUrl').value.trim() || null;
    const genresInput = document.getElementById('editGenres').value.trim();
    const genres = genresInput ? genresInput.split(',').map(g => g.trim()).filter(Boolean) : null;
    const studio = document.getElementById('editStudio').value.trim() || null;

    // Download artwork locally when a new external URL was selected from the picker
    const origPoster = (_editMediaItem && _editMediaItem.poster_path) || '';
    const origBackdrop = (_editMediaItem && _editMediaItem.backdrop_path) || '';
    let finalPoster = posterPath;
    let finalBackdrop = backdropPath;
    if (posterPath && posterPath !== origPoster && posterPath.startsWith('http')) {
        const artRes = await api('PUT', '/media/' + id + '/artwork', { type: 'poster', url: posterPath });
        if (artRes.success && artRes.data && artRes.data.path) finalPoster = artRes.data.path;
    }
    if (backdropPath && backdropPath !== origBackdrop && backdropPath.startsWith('http')) {
        const artRes = await api('PUT', '/media/' + id + '/artwork', { type: 'backdrop', url: backdropPath });
        if (artRes.success && artRes.data && artRes.data.path) finalBackdrop = artRes.data.path;
    }

    const d = await api('PUT', '/media/' + id, {
        title, sort_title: sortTitle, original_title: originalTitle,
        description, year, release_date: releaseDate, rating,
        edition_type: editionType, poster_path: finalPoster, backdrop_path: finalBackdrop,
        custom_notes: customNotes, custom_tags: customTags,
        content_rating: contentRating, original_language: originalLanguage,
        tagline, country, trailer_url: trailerUrl, genres, studio
    });
    if (d.success) {
        // Set edition parent if one was selected
        const parentId = document.getElementById('parentMovieId').value;
        if (parentId) {
            await api('POST', '/media/' + id + '/edition-parent', {
                parent_id: parentId,
                edition_type: editionType
            });
        }
        toast('Changes saved! Metadata locked.');
        closeEditModal();
        // If triggered from duplicate review, mark as addressed
        if (window._dupResolveAfterEdit) {
            const r = window._dupResolveAfterEdit;
            await api('POST', '/duplicates/resolve', { media_id: r.itemId, partner_id: r.partnerId, action: 'edit' });
            window._dupResolveAfterEdit = null;
            loadDuplicatesView();
            loadSidebarCounts();
        } else {
            loadMediaDetail(id);
        }
    } else {
        toast(d.error || 'Save failed', 'error');
    }
}

async function resetMediaMeta() {
    const id = document.getElementById('editMediaId').value;
    if (!confirm('Reset all metadata locks? The next auto-match or scan will be able to overwrite your edits.')) return;
    // Clear per-field locks as well
    await api('PUT', '/media/' + id + '/locked-fields', { locked_fields: [] });
    const d = await api('POST', '/media/' + id + '/reset');
    if (d.success) {
        toast('All metadata locks removed');
        closeEditModal();
        loadMediaDetail(id);
    } else {
        toast(d.error || 'Reset failed', 'error');
    }
}

// ──── Per-Field Locks ────
const LOCKABLE_FIELDS = [
    { key: 'title', label: 'Title' },
    { key: 'description', label: 'Description' },
    { key: 'year', label: 'Year' },
    { key: 'rating', label: 'Rating' },
    { key: 'poster_path', label: 'Poster' },
    { key: 'content_rating', label: 'Content Rating' },
    { key: 'tagline', label: 'Tagline' },
    { key: 'original_language', label: 'Language' },
    { key: 'country', label: 'Country' },
    { key: 'trailer_url', label: 'Trailer' },
    { key: 'logo_path', label: 'Logo' },
    { key: 'backdrop_path', label: 'Backdrop' },
    { key: 'imdb_rating', label: 'IMDb Rating' },
    { key: 'rt_rating', label: 'RT Score' },
    { key: 'audience_score', label: 'Audience Score' },
    { key: 'genres', label: 'Genres' },
    { key: 'cast', label: 'Cast & Crew' },
    { key: 'source_type', label: 'Source Type' },
    { key: 'hdr_format', label: 'HDR Format' },
    { key: 'custom_notes', label: 'Custom Notes' },
    { key: 'custom_tags', label: 'Custom Tags' },
];
let _currentLockedFields = [];

function renderFieldLocks(lockedFields) {
    _currentLockedFields = lockedFields || [];
    const grid = document.getElementById('fieldLocksGrid');
    const hasWildcard = _currentLockedFields.includes('*');
    grid.innerHTML = LOCKABLE_FIELDS.map(f => {
        const isLocked = hasWildcard || _currentLockedFields.includes(f.key);
        return `<div class="field-lock-item">
            <label onclick="toggleFieldLock('${f.key}')">${f.label}</label>
            <span class="field-lock-icon ${isLocked ? 'locked' : 'unlocked'}" onclick="toggleFieldLock('${f.key}')"
                title="${isLocked ? 'Locked — click to unlock' : 'Unlocked — click to lock'}">
                ${isLocked ? '&#128274;' : '&#128275;'}
            </span>
        </div>`;
    }).join('');
}

async function toggleFieldLock(fieldKey) {
    const id = document.getElementById('editMediaId').value;
    // Remove wildcard if present (switching to per-field mode)
    let fields = _currentLockedFields.filter(f => f !== '*');
    const idx = fields.indexOf(fieldKey);
    if (idx >= 0) {
        fields.splice(idx, 1);
    } else {
        fields.push(fieldKey);
    }
    // Save to API
    const d = await api('PUT', '/media/' + id + '/locked-fields', { locked_fields: fields });
    if (d.success) {
        _currentLockedFields = fields;
        renderFieldLocks(fields);
        // Update lock status display
        updateLockStatusDisplay(fields);
    } else {
        toast(d.error || 'Failed to update field lock', 'error');
    }
}

function updateLockStatusDisplay(fields) {
    const lockDiv = document.getElementById('editLockStatus');
    const resetBtn = document.getElementById('editResetBtn');
    if (fields.includes('*') || fields.length === LOCKABLE_FIELDS.length) {
        lockDiv.innerHTML = '<span class="lock-badge locked">&#128274; All fields locked</span>';
        resetBtn.style.display = 'inline-flex';
    } else if (fields.length > 0) {
        lockDiv.innerHTML = '<span class="lock-badge locked">&#128274; ' + fields.length + ' field' + (fields.length > 1 ? 's' : '') + ' locked</span>';
        resetBtn.style.display = 'inline-flex';
    } else {
        lockDiv.innerHTML = '<span class="lock-badge unlocked">&#128275; No fields locked — auto-match may update all fields</span>';
        resetBtn.style.display = 'none';
    }
}

// ──── Edit Modal Cast Display ────
function toggleEditCast() {
    const panel = document.getElementById('editCastPanel');
    const toggle = document.getElementById('editCastToggle');
    const isOpen = panel.style.display !== 'none';
    panel.style.display = isOpen ? 'none' : '';
    toggle.classList.toggle('open', !isOpen);
}

async function loadEditCast(mediaId) {
    const panel = document.getElementById('editCastPanel');
    const castRes = await api('GET', '/media/' + mediaId + '/cast');
    if (castRes.success && castRes.data && castRes.data.length > 0) {
        const castItems = castRes.data.map(c => {
            const subtitle = c.role === 'actor' ? (c.character_name || 'Actor') : c.role;
            return `<div style="display:flex;align-items:center;gap:10px;padding:6px 0;border-bottom:1px solid rgba(255,255,255,0.05);">
                <div style="width:32px;height:32px;border-radius:50%;background:rgba(0,217,255,0.15);display:flex;align-items:center;justify-content:center;font-size:0.7rem;flex-shrink:0;">
                    ${c.photo_path ? '<img src="'+c.photo_path+'" style="width:32px;height:32px;border-radius:50%;object-fit:cover;">' : '&#128100;'}
                </div>
                <div style="flex:1;min-width:0;">
                    <div style="font-size:0.82rem;color:#e5e5e5;">${c.name}</div>
                    <div style="font-size:0.72rem;color:#5a6a7f;">${subtitle}</div>
                </div>
            </div>`;
        }).join('');
        panel.innerHTML = `<div style="max-height:200px;overflow-y:auto;padding:8px 0;">${castItems}</div>
            <p style="font-size:0.72rem;color:#6b7b8d;margin-top:6px;">Cast is populated via metadata matching. Use Identify to refresh.</p>`;
    } else {
        panel.innerHTML = '<p style="color:#5a6a7f;font-size:0.82rem;padding:8px 0;">No cast data. Use Identify to populate from metadata sources.</p>';
    }
}

// ──── Parent Movie Search ────
let _parentSearchTimer = null;
function searchParentMovie(query) {
    clearTimeout(_parentSearchTimer);
    const results = document.getElementById('parentSearchResults');
    if (query.length < 2) { results.classList.remove('active'); return; }
    _parentSearchTimer = setTimeout(async () => {
        const d = await api('GET', '/media/search?q=' + encodeURIComponent(query));
        if (!d.success || !d.data || d.data.length === 0) { results.classList.remove('active'); return; }
        const editId = document.getElementById('editMediaId').value;
        const items = d.data.filter(i => i.id !== editId && (i.media_type === 'movies' || i.media_type === 'adult_movies'));
        if (items.length === 0) { results.classList.remove('active'); return; }
        results.innerHTML = items.slice(0, 10).map(i =>
            `<div class="parent-search-result" onclick="selectParentMovie('${i.id}','${i.title.replace(/'/g,"\\'")}',${i.year||'null'})">${i.title}<span class="psr-year">${i.year||''}</span></div>`
        ).join('');
        results.classList.add('active');
    }, 300);
}

function selectParentMovie(id, title, year) {
    document.getElementById('parentMovieId').value = id;
    document.getElementById('parentMovieSearch').value = '';
    document.getElementById('parentSearchResults').classList.remove('active');
    const info = document.getElementById('parentCurrentInfo');
    info.style.display = '';
    info.innerHTML = `<div class="parent-current"><span class="pc-title">&#128279; Parent: ${title}${year ? ' ('+year+')' : ''}</span><button class="pc-remove" onclick="clearParentSelection()">Remove</button></div>`;
}

function clearParentSelection() {
    document.getElementById('parentMovieId').value = '';
    document.getElementById('parentCurrentInfo').style.display = 'none';
    document.getElementById('parentCurrentInfo').innerHTML = '';
}

async function removeEditionParent(mediaId) {
    if (!confirm('Remove this movie from its edition group?')) return;
    const d = await api('DELETE', '/media/' + mediaId + '/edition-parent');
    if (d.success) {
        toast('Removed from edition group');
        closeEditModal();
        loadMediaDetail(mediaId);
    } else {
        toast(d.error || 'Failed to remove', 'error');
    }
}

// Close parent search on click outside
document.addEventListener('click', function(e) {
    const wrap = document.querySelector('.parent-search-wrap');
    if (wrap && !wrap.contains(e.target)) {
        document.getElementById('parentSearchResults').classList.remove('active');
    }
});

// Close edit modal on Escape
document.getElementById('editMediaOverlay').addEventListener('click', function(e) {
    if (e.target === this) closeEditModal();
});

// ──── Movie Series ────
let _seriesLibraryList = [];

