// Library grid view
function getFilterDefs() {
    if (_gridState.mediaType === 'music') {
        return ALL_FILTER_DEFS.filter(d => MUSIC_FILTER_KEYS.has(d.key));
    }
    return ALL_FILTER_DEFS;
}

// Keep FILTER_DEFS as a getter for backward compatibility
const FILTER_DEFS = ALL_FILTER_DEFS;

// ──── Virtual Scroll Engine ────
const _vs = {
    active: false,
    items: new Map(),
    totalItems: 0,
    colCount: 0,
    rowHeight: 0,
    cardGap: 22,
    bufferRows: 4,
    renderedRange: null,
    fetchBatch: 200,
    fetching: new Set(),
    container: null,
    grid: null,
    gridOffset: 0,
    measured: false,
    scrollHandler: null,
    resizeHandler: null,
    rafId: null,
};

function vsTeardown() {
    if (_vs.scrollHandler && _vs.container) {
        _vs.container.removeEventListener('scroll', _vs.scrollHandler);
    }
    if (_vs.resizeHandler) {
        window.removeEventListener('resize', _vs.resizeHandler);
    }
    if (_vs.rafId) cancelAnimationFrame(_vs.rafId);
    _vs.active = false;
    _vs.items.clear();
    _vs.totalItems = 0;
    _vs.colCount = 0;
    _vs.rowHeight = 0;
    _vs.measured = false;
    _vs.renderedRange = null;
    _vs.fetching.clear();
    _vs.container = null;
    _vs.grid = null;
    _vs.gridOffset = 0;
    _vs.scrollHandler = null;
    _vs.resizeHandler = null;
    _vs.rafId = null;
}

function vsMeasureGrid() {
    if (!_vs.grid) return;
    const style = getComputedStyle(_vs.grid);
    _vs.colCount = style.gridTemplateColumns.split(' ').length;
    const card = _vs.grid.querySelector('.media-card');
    if (card) {
        _vs.rowHeight = card.offsetHeight + _vs.cardGap;
        _vs.measured = true;
    }
    const containerRect = _vs.container.getBoundingClientRect();
    const gridRect = _vs.grid.getBoundingClientRect();
    _vs.gridOffset = gridRect.top - containerRect.top + _vs.container.scrollTop;
}

async function vsFetchRange(start, count) {
    const end = Math.min(start + count, _vs.totalItems);
    let needFetch = false;
    for (let i = start; i < end; i++) {
        if (!_vs.items.has(i)) { needFetch = true; break; }
    }
    if (!needFetch) return;

    const fetchKey = start + ':' + count;
    if (_vs.fetching.has(fetchKey)) return;
    _vs.fetching.add(fetchKey);
    try {
        const m = await api('GET', '/libraries/' + _gridState.libraryId +
            '/media?limit=' + count + '&offset=' + start + buildFilterQS());
        const items = (m.success && m.data && m.data.items) ? m.data.items : [];
        for (let i = 0; i < items.length; i++) _vs.items.set(start + i, items[i]);
        if (m.data && m.data.total !== undefined) {
            _vs.totalItems = m.data.total;
            _gridState.total = m.data.total;
        }
    } finally {
        _vs.fetching.delete(fetchKey);
    }
}

function vsRender(force) {
    if (!_vs.active || !_vs.measured || !_vs.grid || !_vs.container) return;
    if (_vs.rowHeight <= 0 || _vs.colCount <= 0) return;

    const scrollTop = _vs.container.scrollTop;
    const viewportH = _vs.container.clientHeight;
    const rowPitch = _vs.rowHeight;
    const totalRows = Math.ceil(_vs.totalItems / _vs.colCount);
    if (totalRows <= 0) return;
    const adjScroll = Math.max(0, scrollTop - _vs.gridOffset);

    const startRow = Math.max(0, Math.floor(adjScroll / rowPitch) - _vs.bufferRows);
    const endRow = Math.min(totalRows - 1,
        Math.ceil((adjScroll + viewportH) / rowPitch) + _vs.bufferRows);

    if (!force && _vs.renderedRange &&
        _vs.renderedRange.startRow === startRow &&
        _vs.renderedRange.endRow === endRow) return;

    _vs.renderedRange = { startRow, endRow };

    const startIdx = startRow * _vs.colCount;
    const endIdx = Math.min((endRow + 1) * _vs.colCount, _vs.totalItems);

    // Prefetch visible range + one batch ahead and behind
    const prefetchStart = Math.max(0, startIdx - _vs.fetchBatch);
    const prefetchEnd = Math.min(_vs.totalItems, endIdx + _vs.fetchBatch);
    for (let off = prefetchStart; off < prefetchEnd; off += _vs.fetchBatch) {
        const batchStart = Math.floor(off / _vs.fetchBatch) * _vs.fetchBatch;
        let missing = false;
        const batchEnd = Math.min(batchStart + _vs.fetchBatch, _vs.totalItems);
        for (let i = batchStart; i < batchEnd; i++) {
            if (!_vs.items.has(i)) { missing = true; break; }
        }
        if (missing) {
            vsFetchRange(batchStart, _vs.fetchBatch).then(() => {
                if (!_vs.active) return;
                // Schedule a single debounced re-render instead of forcing immediately
                if (_vs.rafId) return;
                _vs.rafId = requestAnimationFrame(() => {
                    _vs.rafId = null;
                    vsRender(true);
                });
            });
        }
    }

    const cards = [];
    for (let i = startIdx; i < endIdx; i++) {
        const item = _vs.items.get(i);
        if (item) {
            cards.push(renderMediaCard(item));
        } else {
            cards.push('<div class="skeleton-card"><div class="skeleton skeleton-poster"></div>' +
                '<div class="skeleton skeleton-title"></div><div class="skeleton skeleton-meta"></div></div>');
        }
    }

    const topH = startRow * rowPitch;
    const bottomH = Math.max(0, (totalRows - endRow - 1) * rowPitch);

    _vs.grid.style.paddingTop = topH + 'px';
    _vs.grid.style.paddingBottom = bottomH + 'px';
    _vs.grid.innerHTML = cards.join('');
}

function vsUpdateAlpha() {
    if (!_vs.active || !_vs.measured) return;
    const jump = document.getElementById('alphaJump');
    if (!jump) return;

    const adjScroll = Math.max(0, _vs.container.scrollTop - _vs.gridOffset);
    const visibleIdx = Math.floor(adjScroll / _vs.rowHeight) * _vs.colCount;

    const li = _gridState.letterIndex;
    let activeLetter = '#';
    for (let i = li.length - 1; i >= 0; i--) {
        if (li[i].offset <= visibleIdx) { activeLetter = li[i].letter; break; }
    }
    jump.querySelectorAll('.alpha-jump-letter').forEach(el => {
        el.classList.toggle('active', el.dataset.letter === activeLetter);
    });
}

async function vsInit(restoreScrollTop) {
    const grid = document.getElementById('libGrid');
    const container = document.getElementById('mainContent');
    if (!grid || !container) return;

    _vs.grid = grid;
    _vs.container = container;
    _vs.totalItems = _gridState.letterIndex.reduce((s, e) => s + e.count, 0);
    _vs.active = true;
    _vs.items.clear();
    _vs.measured = false;
    _vs.renderedRange = null;

    if (_vs.totalItems === 0) {
        const type = (allLibraries.find(l => l.id === _gridState.libraryId) || {}).media_type || '';
        grid.innerHTML = '<div class="empty-state" style="grid-column:1/-1;">' +
            '<div class="empty-state-icon">' + mediaIcon(type) + '</div>' +
            '<div class="empty-state-title">No items in this library</div>' +
            '<p>Scan the library to populate it with media</p></div>';
        return;
    }

    // Fetch first batch and render measurement cards
    await vsFetchRange(0, _vs.fetchBatch);
    const measuredItems = [];
    for (let i = 0; i < Math.min(_vs.colCount || 10, _vs.totalItems); i++) {
        if (_vs.items.has(i)) measuredItems.push(_vs.items.get(i));
    }
    grid.innerHTML = measuredItems.map(renderMediaCard).join('');

    // Force layout then measure
    vsMeasureGrid();
    if (!_vs.measured) return;

    // If restoring scroll, prefetch that range too
    if (restoreScrollTop > 0) {
        const adjScroll = Math.max(0, restoreScrollTop - _vs.gridOffset);
        const targetRow = Math.floor(adjScroll / _vs.rowHeight);
        const targetIdx = targetRow * _vs.colCount;
        const fetchStart = Math.floor(targetIdx / _vs.fetchBatch) * _vs.fetchBatch;
        await vsFetchRange(fetchStart, _vs.fetchBatch);
    }

    vsRender(true);

    if (restoreScrollTop > 0) {
        container.style.scrollBehavior = 'auto';
        container.scrollTop = restoreScrollTop;
        container.style.scrollBehavior = '';
        vsRender(true);
    }

    // Scroll handler with single-RAF debounce
    _vs.scrollHandler = () => {
        if (_vs.rafId) return;
        _vs.rafId = requestAnimationFrame(() => {
            _vs.rafId = null;
            vsRender();
            vsUpdateAlpha();
        });
    };
    container.addEventListener('scroll', _vs.scrollHandler, { passive: true });

    _vs.resizeHandler = () => {
        const oldCols = _vs.colCount;
        vsMeasureGrid();
        if (_vs.colCount !== oldCols) vsRender(true);
    };
    window.addEventListener('resize', _vs.resizeHandler);

    vsUpdateAlpha();
    enableGridKeyNav(grid);
}

function teardownGrid() {
    vsTeardown();
    if (_gridState.observer) { _gridState.observer.disconnect(); _gridState.observer = null; }
    if (_gridState.topObserver) { _gridState.topObserver.disconnect(); _gridState.topObserver = null; }
    _gridState = { libraryId: null, offset: 0, startOffset: 0, total: 0, loading: false, done: false, observer: null, topObserver: null, letterIndex: [], filters: {}, mediaType: '' };
}

// Build query string from current filters
function buildFilterQS() {
    const f = _gridState.filters || {};
    const params = [];
    const keys = ['genre','folder','content_rating','edition','source','dynamic_range',
        'codec','hdr_format','resolution','audio_codec','bitrate_range','country',
        'duration_range','watch_status','added_days','year_from','year_to','min_rating',
        'sort','order'];
    for (const k of keys) {
        if (f[k]) params.push(k + '=' + encodeURIComponent(f[k]));
    }
    return params.length > 0 ? '&' + params.join('&') : '';
}

async function loadMoreMedia() {
    if (_gridState.loading || _gridState.done) return;
    _gridState.loading = true;
    const m = await api('GET', '/libraries/' + _gridState.libraryId + '/media?limit=200&offset=' + _gridState.offset + buildFilterQS());
    const items = (m.success && m.data && m.data.items) ? m.data.items : [];
    _gridState.total = m.data ? m.data.total : _gridState.total;

    const grid = document.getElementById('libGrid');
    if (!grid) { _gridState.loading = false; return; }

    // Remove sentinel before appending
    const sentinel = document.getElementById('gridSentinel');
    if (sentinel) sentinel.remove();

    if (items.length === 0 && _gridState.offset === 0) {
        const type = (allLibraries.find(l => l.id === _gridState.libraryId) || {}).media_type || '';
        grid.innerHTML = `<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">${mediaIcon(type)}</div><div class="empty-state-title">No items in this library</div><p>Scan the library to populate it with media</p></div>`;
        _gridState.done = true;
        _gridState.loading = false;
        return;
    }

    grid.insertAdjacentHTML('beforeend', items.map(renderMediaCard).join(''));
    _gridState.offset += items.length;

    // Check if we've loaded everything
    if (_gridState.offset >= _gridState.total || items.length < 200) {
        _gridState.done = true;
        updateAlphaCount();
    } else {
        // Add sentinel for infinite scroll
        grid.insertAdjacentHTML('beforeend', '<div id="gridSentinel" class="load-more-sentinel"><div class="spinner"></div></div>');
        const newSentinel = document.getElementById('gridSentinel');
        if (newSentinel && _gridState.observer) _gridState.observer.observe(newSentinel);
    }
    _gridState.loading = false;
    updateAlphaCount();
    // Enable keyboard navigation on library grid
    enableGridKeyNav(grid);
}

// ──── Live scan: append newly added items to the grid ────
let _appendingNewItems = false;
async function appendNewScanItems(libraryId, newCount) {
    if (_appendingNewItems || !libraryId || newCount <= 0) return;
    if (_gridState.libraryId !== libraryId) return;

    _appendingNewItems = true;
    try {
        if (_vs.active) {
            // Invalidate VS cache: bump total and clear cached ranges near the end
            _vs.totalItems += newCount;
            _gridState.total = _vs.totalItems;
            const countEl = document.getElementById('libItemCount');
            if (countEl) countEl.textContent = _vs.totalItems.toLocaleString() + ' items';
            // Clear cached items near the end so they re-fetch with new additions
            const clearFrom = Math.max(0, _vs.totalItems - newCount - _vs.fetchBatch);
            for (let i = clearFrom; i < _vs.totalItems; i++) _vs.items.delete(i);
            vsRender(true);
            return;
        }

        const grid = document.getElementById('libGrid');
        if (!grid) return;
        const m = await api('GET', '/libraries/' + libraryId + '/media?sort=added_at&order=desc&limit=' + Math.min(newCount, 50));
        const items = (m.success && m.data && m.data.items) ? m.data.items : [];
        if (items.length === 0) return;
        const newItems = items.filter(item => !grid.querySelector('[data-media-id="' + item.id + '"]'));
        if (newItems.length === 0) return;
        grid.insertAdjacentHTML('beforeend', newItems.map(renderMediaCard).join(''));
        _gridState.total += newItems.length;
        _gridState.offset += newItems.length;
        const countEl = document.getElementById('libItemCount');
        if (countEl) countEl.textContent = _gridState.total.toLocaleString() + ' items';
    } catch (e) {
        // Silently ignore fetch errors during scan
    } finally {
        _appendingNewItems = false;
    }
}

function setupScrollObserver() {
    _gridState.observer = new IntersectionObserver((entries) => {
        if (entries[0].isIntersecting) loadMoreMedia();
    }, { root: document.getElementById('mainContent'), rootMargin: '800px' });
    const sentinel = document.getElementById('gridSentinel');
    if (sentinel) _gridState.observer.observe(sentinel);
}

function setupTopScrollObserver() {
    _gridState.topObserver = new IntersectionObserver((entries) => {
        if (entries[0].isIntersecting) loadPreviousMedia();
    }, { root: document.getElementById('mainContent'), rootMargin: '400px' });
    const sentinel = document.getElementById('gridTopSentinel');
    if (sentinel) _gridState.topObserver.observe(sentinel);
}

async function loadPreviousMedia() {
    if (_gridState.loading || _gridState.startOffset <= 0) return;
    _gridState.loading = true;

    const batchSize = 200;
    const newStart = Math.max(0, _gridState.startOffset - batchSize);
    const limit = _gridState.startOffset - newStart;

    const m = await api('GET', '/libraries/' + _gridState.libraryId + '/media?limit=' + limit + '&offset=' + newStart + buildFilterQS());
    const items = (m.success && m.data && m.data.items) ? m.data.items : [];

    const grid = document.getElementById('libGrid');
    if (!grid || items.length === 0) { _gridState.loading = false; return; }

    const container = document.getElementById('mainContent');

    // Save scroll reference — first real card currently in view
    const firstCard = grid.querySelector('.media-card');
    const scrollBefore = container ? container.scrollTop : 0;
    const firstCardTopBefore = firstCard ? firstCard.getBoundingClientRect().top : 0;

    // Remove existing top sentinel
    const topSentinel = document.getElementById('gridTopSentinel');
    if (topSentinel) topSentinel.remove();

    // Prepend items
    grid.insertAdjacentHTML('afterbegin', items.map(renderMediaCard).join(''));

    // Restore scroll position so view doesn't jump
    if (firstCard && container) {
        const firstCardTopAfter = firstCard.getBoundingClientRect().top;
        container.scrollTop = scrollBefore + (firstCardTopAfter - firstCardTopBefore);
    }

    _gridState.startOffset = newStart;

    // Re-add top sentinel if still more to load backward
    if (_gridState.startOffset > 0) {
        grid.insertAdjacentHTML('afterbegin', '<div id="gridTopSentinel" class="load-more-sentinel"></div>');
        const newTopSentinel = document.getElementById('gridTopSentinel');
        if (newTopSentinel && _gridState.topObserver) _gridState.topObserver.observe(newTopSentinel);
    }

    _gridState.loading = false;
    updateAlphaCount();
}

function buildAlphaJump(letterIndex) {
    const allLetters = ['#','A','B','C','D','E','F','G','H','I','J','K','L','M','N','O','P','Q','R','S','T','U','V','W','X','Y','Z'];
    const indexMap = {};
    letterIndex.forEach(e => { indexMap[e.letter] = e; });
    return `<div class="alpha-jump" id="alphaJump">${allLetters.map(l => {
        const entry = indexMap[l];
        if (entry) {
            return `<div class="alpha-jump-letter" data-letter="${l}" data-offset="${entry.offset}" onclick="jumpToLetter(this)">${l}</div>`;
        }
        return `<div class="alpha-jump-letter disabled">${l}</div>`;
    }).join('')}</div>`;
}

function updateAlphaCount() {
    // Highlight the current visible letter based on scroll position
    const grid = document.getElementById('libGrid') || document.getElementById('typeGrid');
    const jump = document.getElementById('alphaJump');
    if (!grid || !jump) return;
    const cards = grid.querySelectorAll('.media-card');
    if (cards.length === 0) return;

    // Find the first visible card
    const container = document.getElementById('mainContent');
    const containerTop = container.getBoundingClientRect().top;
    let activeLetter = '';
    for (const card of cards) {
        const rect = card.getBoundingClientRect();
        if (rect.top >= containerTop - 50) {
            const title = card.querySelector('.media-title');
            if (title) {
                const sortTitle = card.dataset.sortTitle || title.textContent;
                activeLetter = sortableTitleLetter(sortTitle);
            }
            break;
        }
    }
    jump.querySelectorAll('.alpha-jump-letter').forEach(el => {
        el.classList.toggle('active', el.dataset.letter === activeLetter);
    });
}

// Strip leading articles (The, A, An) for alpha-jump matching
function sortableTitleLetter(text) {
    const t = text.trim();
    const stripped = t.replace(/^(The|A|An)\s+/i, '');
    const first = stripped.charAt(0).toUpperCase();
    return (first >= 'A' && first <= 'Z') ? first : '#';
}

// Build a client-side alpha-jump for views that load all items at once
function buildClientAlphaJump(items, titleField) {
    titleField = titleField || 'title';
    const allLetters = ['#','A','B','C','D','E','F','G','H','I','J','K','L','M','N','O','P','Q','R','S','T','U','V','W','X','Y','Z'];
    const letterSet = new Set();
    items.forEach(item => {
        const title = item[titleField] || '';
        letterSet.add(sortableTitleLetter(title));
    });
    return `<div class="alpha-jump" id="alphaJump">${allLetters.map(l => {
        if (letterSet.has(l)) {
            return `<div class="alpha-jump-letter" data-letter="${l}" onclick="jumpToClientLetter(this)">${l}</div>`;
        }
        return `<div class="alpha-jump-letter disabled">${l}</div>`;
    }).join('')}</div>`;
}

// Client-side letter jump — scrolls to first card matching the letter
function jumpToClientLetter(el) {
    const letter = el.dataset.letter;
    const grid = document.getElementById('libGrid') || document.getElementById('typeGrid');
    if (!grid) return;

    const cards = grid.querySelectorAll('.media-card');
    for (const card of cards) {
        const title = card.querySelector('.media-title');
        if (title) {
            const sortTitle = card.dataset.sortTitle || title.textContent;
            if (sortableTitleLetter(sortTitle) === letter) {
                card.scrollIntoView({ behavior: 'smooth', block: 'start' });
                break;
            }
        }
    }

    document.querySelectorAll('.alpha-jump-letter').forEach(l => l.classList.remove('active'));
    el.classList.add('active');
}

// Update active letter on scroll for client-side grids
function updateClientAlphaCount() {
    const grid = document.getElementById('libGrid') || document.getElementById('typeGrid');
    const jump = document.getElementById('alphaJump');
    if (!grid || !jump) return;
    const cards = grid.querySelectorAll('.media-card');
    if (cards.length === 0) return;

    const container = document.getElementById('mainContent');
    const containerTop = container.getBoundingClientRect().top;
    let activeLetter = '';
    for (const card of cards) {
        const rect = card.getBoundingClientRect();
        if (rect.top >= containerTop - 50) {
            const title = card.querySelector('.media-title');
            if (title) {
                const sortTitle = card.dataset.sortTitle || title.textContent;
                activeLetter = sortableTitleLetter(sortTitle);
            }
            break;
        }
    }
    jump.querySelectorAll('.alpha-jump-letter').forEach(el => {
        el.classList.toggle('active', el.dataset.letter === activeLetter);
    });
}

async function jumpToLetter(el) {
    const targetOffset = parseInt(el.dataset.offset);
    if (!_vs.active || !_vs.measured || !_vs.container || _vs.colCount <= 0) return;

    const targetRow = Math.floor(targetOffset / _vs.colCount);
    const totalRows = Math.ceil(_vs.totalItems / _vs.colCount);
    const rowPitch = _vs.rowHeight;
    const targetScroll = _vs.gridOffset + targetRow * rowPitch;

    // Pre-set padding so the grid height supports the target scroll position
    const viewportH = _vs.container.clientHeight;
    const adjScroll = Math.max(0, targetScroll - _vs.gridOffset);
    const startRow = Math.max(0, Math.floor(adjScroll / rowPitch) - _vs.bufferRows);
    const endRow = Math.min(totalRows - 1,
        Math.ceil((adjScroll + viewportH) / rowPitch) + _vs.bufferRows);
    _vs.grid.style.paddingTop = (startRow * rowPitch) + 'px';
    _vs.grid.style.paddingBottom = Math.max(0, (totalRows - endRow - 1) * rowPitch) + 'px';

    // Instant jump — bypass CSS scroll-behavior: smooth
    _vs.container.style.scrollBehavior = 'auto';
    _vs.container.scrollTop = targetScroll;
    _vs.container.style.scrollBehavior = '';

    // Prefetch the target range if not cached
    const fetchStart = Math.floor(targetOffset / _vs.fetchBatch) * _vs.fetchBatch;
    vsFetchRange(fetchStart, _vs.fetchBatch).then(() => {
        if (!_vs.active) return;
        if (_vs.rafId) return;
        _vs.rafId = requestAnimationFrame(() => {
            _vs.rafId = null;
            vsRender(true);
        });
    });

    vsRender(true);
    document.querySelectorAll('.alpha-jump-letter').forEach(l => l.classList.remove('active'));
    el.classList.add('active');
}

async function loadMediaTypeView(mediaType) {
    teardownGrid();
    const mc = document.getElementById('mainContent');
    const label = MEDIA_LABELS[mediaType] || mediaType;

    // Find all libraries of this type
    const libs = await api('GET', '/libraries');
    const matchingLibs = (libs.success && libs.data) ? libs.data.filter(l => l.media_type === mediaType) : [];

    if (matchingLibs.length === 1) {
        // Single library — use paginated view
        return loadLibraryView(matchingLibs[0].id);
    }

    // Multiple libraries — load all (with pagination for each)
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">${label}</h2></div>
        <div class="media-grid-wrapper"><div class="media-grid" id="typeGrid"><div class="spinner"></div> Loading...</div></div>`;
    let items = [];
    for (const lib of matchingLibs) {
        let off = 0;
        while (true) {
            const m = await api('GET', '/libraries/'+lib.id+'/media?limit=500&offset='+off);
            if (m.success && m.data && m.data.items && m.data.items.length > 0) {
                items = items.concat(m.data.items);
                off += m.data.items.length;
                if (off >= m.data.total) break;
            } else break;
        }
    }
    const grid = document.getElementById('typeGrid');
    if (items.length > 0) {
        grid.innerHTML = items.map(renderMediaCard).join('');
        const wrapper = grid.closest('.media-grid-wrapper');
        if (wrapper) wrapper.insertAdjacentHTML('beforeend', buildClientAlphaJump(items, 'title'));
        const container = document.getElementById('mainContent');
        if (container) container.addEventListener('scroll', () => { requestAnimationFrame(updateClientAlphaCount); });
    } else {
        grid.innerHTML = `<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">${mediaIcon(mediaType)}</div><div class="empty-state-title">No ${label.toLowerCase()} yet</div><p>Add a ${label.toLowerCase()} library and scan it</p></div>`;
    }
}

async function loadLibraryView(libraryId) {
    teardownGrid();
    const mc = document.getElementById('mainContent');
    const lib = allLibraries.find(l => l.id === libraryId);
    const label = lib ? lib.name : 'Library';
    const type = lib ? lib.media_type : '';

    // TV show library with season grouping → show TV shows instead of flat episodes
    if (lib && lib.media_type === 'tv_shows' && lib.season_grouping) {
        mc.innerHTML = `<div class="section-header"><h2 class="section-title">${label}</h2><span class="tag tag-cyan" style="margin-left:12px;">TV Shows</span></div>
            <div class="media-grid-wrapper"><div class="media-grid" id="libGrid"><div class="spinner"></div> Loading...</div></div>`;
        const data = await api('GET', '/libraries/' + libraryId + '/shows');
        const shows = (data.success && data.data) ? data.data : [];
        const grid = document.getElementById('libGrid');
        if (shows.length > 0) {
            grid.innerHTML = shows.map(renderShowCard).join('');
            const wrapper = grid.closest('.media-grid-wrapper');
            if (wrapper) wrapper.insertAdjacentHTML('beforeend', buildClientAlphaJump(shows, 'title'));
            const container = document.getElementById('mainContent');
            if (container) container.addEventListener('scroll', () => { requestAnimationFrame(updateClientAlphaCount); });
        } else {
            grid.innerHTML = `<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">&#128250;</div><div class="empty-state-title">No TV shows yet</div><p>Scan the library to detect shows</p></div>`;
        }
        const tvRestore = _pendingScrollRestore;
        _pendingScrollRestore = null;
        if (tvRestore && tvRestore.scrollTop > 0) {
            requestAnimationFrame(() => {
                const c = document.getElementById('mainContent');
                if (c) c.scrollTop = tvRestore.scrollTop;
            });
        }
        return;
    }

    _gridState.libraryId = libraryId;
    _gridState.mediaType = type;

    // Fetch filter options and letter index in parallel
    const [filterData, idxData] = await Promise.all([
        api('GET', '/libraries/' + libraryId + '/filters'),
        api('GET', '/libraries/' + libraryId + '/media/index' + (buildFilterQS() ? '?' + buildFilterQS().substring(1) : ''))
    ]);
    const filterOpts = (filterData.success && filterData.data) ? filterData.data : {};
    const letterIndex = (idxData.success && idxData.data) ? idxData.data : [];
    _gridState.letterIndex = letterIndex;

    const totalCount = letterIndex.reduce((s, e) => s + e.count, 0);

    const isMusic = type === 'music';
    const extraAreas = isMusic
        ? `<div id="artistsArea" style="display:none;"></div>
           <div id="albumsArea" style="display:none;"></div>
           <div id="genresArea" style="display:none;"></div>`
        : `<div id="collectionsArea" style="display:none;"></div>
           <div id="seriesArea" style="display:none;"></div>`;

    mc.innerHTML = `<div class="section-header"><h2 class="section-title">${label}</h2><span class="tag tag-cyan" style="margin-left:12px;">${MEDIA_LABELS[type]||type}</span><span class="tag" id="libItemCount" style="margin-left:8px;">${totalCount.toLocaleString()} items</span></div>
        ${buildFilterToolbar(filterOpts)}
        <div class="media-grid-wrapper" id="mediaGridWrapper">
            <div class="media-grid" id="libGrid"></div>
            ${buildAlphaJump(letterIndex)}
        </div>
        ${extraAreas}`;

    renderFilterChips();
    loadFilterPresetsIntoDropdown();

    if (isMusic) {
        showMusicAlbums();
        return;
    }

    const restoreState = _pendingScrollRestore;
    _pendingScrollRestore = null;

    const restoreScroll = (restoreState && restoreState.libraryId === libraryId)
        ? restoreState.scrollTop : 0;
    await vsInit(restoreScroll);
}

// ──── Filter Toolbar ────
function escFilterHtml(s) { return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }

function getFilterValues(def) {
    if (def.options) return def.options;
    if (def.apiKey && _filterOpts[def.apiKey] && _filterOpts[def.apiKey].length > 0) {
        return _filterOpts[def.apiKey].map(v => ({
            v: v,
            l: def.displayMap ? (def.displayMap[v] || v.toUpperCase()) :
               def.displayFn ? def.displayFn(v) : v
        }));
    }
    return [];
}

function getFilterLabel(key, value) {
    const def = FILTER_DEFS.find(d => d.key === key);
    if (!def) return value;
    const vals = getFilterValues(def);
    const opt = vals.find(o => o.v === value);
    return opt ? opt.l : value;
}

function buildFilterToolbar(opts) {
    _filterOpts = opts;
    const isMusic = _gridState.mediaType === 'music';

    const sortOptions = isMusic
        ? `<option value="">Title</option>
           <option value="artist">Artist</option>
           <option value="album">Album</option>
           <option value="year">Year</option>
           <option value="track_number">Track #</option>
           <option value="duration">Length</option>
           <option value="added_at">Date Added</option>`
        : `<option value="">Title</option>
           <option value="year">Year</option>
           <option value="resolution">Resolution</option>
           <option value="duration">Length</option>
           <option value="bitrate">Bitrate</option>
           <option value="rt_rating">Rotten Tomatoes</option>
           <option value="rating">TMDB Rating</option>
           <option value="audience_score">Audience Score</option>
           <option value="added_at">Date Added</option>`;

    const yearRatingSection = isMusic
        ? `<span class="ft-label">Year</span>
           <input type="number" id="ftYearFrom" placeholder="From" min="1900" max="2099" onchange="applyLibFilter()">
           <input type="number" id="ftYearTo" placeholder="To" min="1900" max="2099" onchange="applyLibFilter()">`
        : `<span class="ft-label">Year</span>
           <input type="number" id="ftYearFrom" placeholder="From" min="1900" max="2099" onchange="applyLibFilter()">
           <input type="number" id="ftYearTo" placeholder="To" min="1900" max="2099" onchange="applyLibFilter()">
           <span class="ft-label">Rating</span>
           <input type="number" id="ftMinRating" placeholder="0" min="0" max="10" step="0.5" onchange="applyLibFilter()">`;

    const viewButtons = isMusic
        ? `<button class="ft-btn" id="ftGridBtn" onclick="showLibraryGrid()" title="All Tracks">&#9638; Grid</button>
           <button class="ft-btn" id="ftArtistBtn" onclick="showMusicArtists()" title="View by Artist">&#127908; Artists</button>
           <button class="ft-btn active" id="ftAlbumBtn" onclick="showMusicAlbums()" title="View by Album">&#128191; Albums</button>
           <button class="ft-btn" id="ftGenreBtn" onclick="showMusicGenres()" title="View by Genre">&#127926; Genres</button>`
        : `<button class="ft-btn" id="ftGridBtn" onclick="showLibraryGrid()" title="Grid view">&#9638; Grid</button>
           <button class="ft-btn" id="ftCollBtn" onclick="showLibraryCollections()" title="Collections view">&#128218; Collections</button>
           <button class="ft-btn" id="ftSeriesBtn" onclick="showLibrarySeries()" title="Series view">&#127910; Series</button>`;

    return `<div class="filter-toolbar" id="filterToolbar">
        <div class="ft-add-wrapper">
            <button class="ft-add-btn" onclick="toggleFilterPicker(event)">&#43; Filter</button>
            <div class="ft-picker" id="ftPicker">
                <div class="ft-picker-cats" id="ftPickerCats"></div>
                <div class="ft-picker-vals" id="ftPickerVals"></div>
            </div>
        </div>
        <div class="ft-chips" id="ftChips"></div>
        <div class="ft-sep"></div>
        ${yearRatingSection}
        <div class="ft-sep"></div>
        <span class="ft-label">Sort</span>
        <select id="ftSort" onchange="applyLibFilter()">
            ${sortOptions}
        </select>
        <select id="ftOrder" onchange="applyLibFilter()">
            <option value="asc">A&#8594;Z / Low&#8594;High</option>
            <option value="desc">Z&#8594;A / High&#8594;Low</option>
        </select>
        <div class="ft-sep"></div>
        ${viewButtons}
        <div class="ft-sep"></div>
        <select class="ft-preset-select" id="ftPresetSelect" onchange="applyFilterPresetFromDropdown(this.value)">
            <option value="">Presets...</option>
        </select>
        <button class="ft-btn" onclick="promptSaveFilterPreset()" title="Save current filters as preset">&#128190; Save</button>
        <button class="ft-reset" onclick="resetLibFilters()" title="Reset all filters">&#10005; Reset</button>
    </div>`;
}

function toggleFilterPicker(e) {
    e.stopPropagation();
    const picker = document.getElementById('ftPicker');
    if (picker.classList.contains('open')) { closeFilterPicker(); return; }
    const catsEl = document.getElementById('ftPickerCats');
    const valsEl = document.getElementById('ftPickerVals');
    let catHtml = '';
    for (const def of getFilterDefs()) {
        if ((_gridState.filters || {})[def.key]) continue;
        const vals = getFilterValues(def);
        if (!vals || vals.length === 0) continue;
        catHtml += `<div class="ft-picker-cat" data-key="${def.key}" onclick="showFilterValues('${def.key}')" onmouseenter="showFilterValues('${def.key}')">${escFilterHtml(def.label)}<span class="ft-picker-cat-count">${vals.length}</span></div>`;
    }
    catsEl.innerHTML = catHtml || '<div class="ft-picker-empty">No more filters available</div>';
    valsEl.innerHTML = '<div class="ft-picker-hint">Hover a category to see options</div>';
    picker.classList.add('open');
    setTimeout(() => document.addEventListener('click', _closePickerOnOutside), 0);
}

function _closePickerOnOutside(e) {
    const wrapper = document.querySelector('.ft-add-wrapper');
    if (wrapper && !wrapper.contains(e.target)) closeFilterPicker();
}

function closeFilterPicker() {
    const picker = document.getElementById('ftPicker');
    if (picker) picker.classList.remove('open');
    document.removeEventListener('click', _closePickerOnOutside);
}

function showFilterValues(key) {
    const def = FILTER_DEFS.find(d => d.key === key);
    if (!def) return;
    _pickerVals = getFilterValues(def);
    const valsEl = document.getElementById('ftPickerVals');
    const searchBox = _pickerVals.length > 10
        ? `<div class="ft-picker-search"><input type="text" placeholder="Search ${def.label.toLowerCase()}..." oninput="filterPickerVals(this.value)" onclick="event.stopPropagation()"></div>` : '';
    valsEl.innerHTML = searchBox + `<div class="ft-picker-val-list" id="ftPickerValList">${_pickerVals.map((opt, i) =>
        `<div class="ft-picker-val" onclick="pickFilterValue('${key}',${i})">${escFilterHtml(opt.l)}</div>`
    ).join('')}</div>`;
    document.querySelectorAll('.ft-picker-cat').forEach(el => {
        el.classList.toggle('active', el.dataset.key === key);
    });
}

function filterPickerVals(q) {
    const list = document.getElementById('ftPickerValList');
    if (!list) return;
    const lq = q.toLowerCase();
    list.querySelectorAll('.ft-picker-val').forEach(el => {
        el.style.display = el.textContent.toLowerCase().includes(lq) ? '' : 'none';
    });
}

function pickFilterValue(key, idx) {
    const value = _pickerVals[idx].v;
    if (!_gridState.filters) _gridState.filters = {};
    _gridState.filters[key] = value;
    closeFilterPicker();
    renderFilterChips();
    reloadLibraryGrid();
}

function removeFilter(key) {
    if (_gridState.filters) delete _gridState.filters[key];
    renderFilterChips();
    reloadLibraryGrid();
}

function renderFilterChips() {
    const chipsEl = document.getElementById('ftChips');
    if (!chipsEl) return;
    const f = _gridState.filters || {};
    let html = '';
    for (const def of getFilterDefs()) {
        if (!f[def.key]) continue;
        const label = getFilterLabel(def.key, f[def.key]);
        html += `<div class="ft-chip"><span class="ft-chip-cat">${escFilterHtml(def.label)}:</span> ${escFilterHtml(label)}<span class="ft-chip-x" onclick="removeFilter('${def.key}')" title="Remove filter">&times;</span></div>`;
    }
    chipsEl.innerHTML = html;
}

function applyLibFilter() {
    const f = {};
    // Preserve chip-based filters
    const pickerKeys = new Set(FILTER_DEFS.map(d => d.key));
    const existing = _gridState.filters || {};
    for (const k of pickerKeys) { if (existing[k]) f[k] = existing[k]; }
    // Read DOM inputs
    const yearFrom = document.getElementById('ftYearFrom');
    const yearTo = document.getElementById('ftYearTo');
    const minRating = document.getElementById('ftMinRating');
    const sort = document.getElementById('ftSort');
    const order = document.getElementById('ftOrder');
    if (yearFrom && yearFrom.value) f.year_from = yearFrom.value;
    if (yearTo && yearTo.value) f.year_to = yearTo.value;
    if (minRating && minRating.value) f.min_rating = minRating.value;
    if (sort && sort.value) f.sort = sort.value;
    if (order && order.value !== 'asc') f.order = order.value;
    _gridState.filters = f;
    reloadLibraryGrid();
}

function resetLibFilters() {
    ['ftSort','ftOrder'].forEach(id => { const el = document.getElementById(id); if (el) el.selectedIndex = 0; });
    ['ftYearFrom','ftYearTo','ftMinRating'].forEach(id => { const el = document.getElementById(id); if (el) el.value = ''; });
    _gridState.filters = {};
    renderFilterChips();
    reloadLibraryGrid();
}

// ──── Filter Presets (F5) ────
async function loadFilterPresetsIntoDropdown() {
    const presets = await loadFilterPresets(_gridState.libraryId);
    const sel = document.getElementById('ftPresetSelect');
    if (!sel) return;
    let opts = '<option value="">Presets...</option>';
    presets.forEach(p => {
        opts += `<option value="${p.id}">${p.name}</option>`;
    });
    opts += '<option value="__manage__" style="color:#ff6b6b;">Manage Presets...</option>';
    sel.innerHTML = opts;
}

async function applyFilterPresetFromDropdown(presetId) {
    if (!presetId) return;
    if (presetId === '__manage__') { showManagePresets(); document.getElementById('ftPresetSelect').value = ''; return; }
    const presets = await loadFilterPresets(_gridState.libraryId);
    const preset = presets.find(p => p.id === presetId);
    if (preset && preset.filters) {
        _gridState.filters = preset.filters;
        renderFilterChips();
        reloadLibraryGrid();
        toast('Preset "' + preset.name + '" applied');
    }
    document.getElementById('ftPresetSelect').value = '';
}

async function promptSaveFilterPreset() {
    const name = prompt('Preset name:');
    if (!name) return;
    const filters = { ..._gridState.filters };
    const ftSort = document.getElementById('ftSort');
    const ftOrder = document.getElementById('ftOrder');
    const ftYearFrom = document.getElementById('ftYearFrom');
    const ftYearTo = document.getElementById('ftYearTo');
    const ftMinRating = document.getElementById('ftMinRating');
    if (ftSort && ftSort.value) filters._sort = ftSort.value;
    if (ftOrder && ftOrder.value) filters._order = ftOrder.value;
    if (ftYearFrom && ftYearFrom.value) filters._yearFrom = ftYearFrom.value;
    if (ftYearTo && ftYearTo.value) filters._yearTo = ftYearTo.value;
    if (ftMinRating && ftMinRating.value) filters._minRating = ftMinRating.value;
    await saveFilterPreset(name, filters, _gridState.libraryId);
    loadFilterPresetsIntoDropdown();
}

async function showManagePresets() {
    const presets = await loadFilterPresets(_gridState.libraryId);
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Filter Presets</h2></div>
        <div id="presetList">${presets.length ? presets.map(p => `
            <div class="job-item"><strong>${p.name}</strong><span style="color:#5a6a7f;margin-left:auto;font-size:0.78rem;">${p.library_id ? 'Library-specific' : 'Global'}</span><button class="btn-danger btn-small" style="margin-left:12px;" onclick="deletePresetAndReload('${p.id}')">Delete</button></div>`).join('') : '<p style="color:#5a6a7f;">No saved presets</p>'}</div>
        <button class="btn-secondary" style="margin-top:16px;" onclick="navigate('home')">&#8592; Back</button>`;
}

async function deletePresetAndReload(id) {
    await deleteFilterPreset(id);
    showManagePresets();
}

async function reloadLibraryGrid() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    vsTeardown();
    const filters = _gridState.filters;
    _gridState.offset = 0;
    _gridState.total = 0;
    _gridState.loading = false;
    _gridState.done = false;
    _gridState.filters = filters;

    const qs = buildFilterQS();
    const idxData = await api('GET', '/libraries/' + libId + '/media/index' + (qs ? '?' + qs.substring(1) : ''));
    const letterIndex = (idxData.success && idxData.data) ? idxData.data : [];
    _gridState.letterIndex = letterIndex;

    const totalCount = letterIndex.reduce((s, e) => s + e.count, 0);
    const countEl = document.getElementById('libItemCount');
    if (countEl) countEl.textContent = totalCount.toLocaleString() + ' items';

    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'flex';
    ['collectionsArea','seriesArea','artistsArea','albumsArea','genresArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    });

    const grid = document.getElementById('libGrid');
    if (grid) grid.innerHTML = '';

    const existingJump = document.querySelector('.alpha-jump');
    if (existingJump) existingJump.outerHTML = buildAlphaJump(letterIndex);

    ['ftGridBtn','ftCollBtn','ftSeriesBtn','ftArtistBtn','ftAlbumBtn','ftGenreBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftGridBtn');
    });

    await vsInit(0);
}

function showLibraryGrid() {
