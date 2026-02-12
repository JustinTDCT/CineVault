async function toggleWatchlist(itemId, btn) {
    const check = await api('GET', '/watchlist/' + itemId + '/check');
    if (check.success && check.data && check.data.in_watchlist) {
        const d = await api('DELETE', '/watchlist/' + itemId);
        if (d.success) { toast('Removed from Watchlist'); if (btn) btn.classList.remove('active'); }
    } else {
        const d = await api('POST', '/watchlist/' + itemId);
        if (d.success) { toast('Added to Watchlist'); if (btn) btn.classList.add('active'); }
    }
}

async function toggleFavorite(itemId, btn) {
    const d = await api('POST', '/favorites/' + itemId);
    if (d.success && d.data) {
        if (d.data.favorited) { toast('Added to Favorites'); if (btn) btn.classList.add('active'); }
        else { toast('Removed from Favorites'); if (btn) btn.classList.remove('active'); }
    }
}

async function rateMedia(mediaId, rating) {
    const d = await api('POST', '/media/' + mediaId + '/rating', { rating });
    if (d.success) toast('Rating saved');
}

async function deleteRating(mediaId) {
    const d = await api('DELETE', '/media/' + mediaId + '/rating');
    if (d.success) toast('Rating removed');
}

// Star rating widget builder
function buildStarRating(mediaId, currentRating) {
    const stars = [10,9,8,7,6,5,4,3,2,1].map(v => {
        const half = v % 2 !== 0;
        const label = half ? '&#9734;' : '&#11088;';
        const checked = currentRating && Math.round(currentRating*2)/2 >= v/2 ? ' checked' : '';
        return `<input type="radio" name="star-${mediaId}" id="star-${mediaId}-${v}" value="${v/2}"${checked} onchange="rateMedia('${mediaId}',${v/2})"><label for="star-${mediaId}-${v}" title="${v/2}">${v%2===0?'&#9733;':'&#9734;'}</label>`;
    });
    return `<div class="star-rating">${stars.join('')}</div>${currentRating ? '<span class="star-rating-value">'+currentRating.toFixed(1)+'</span>' : ''}`;
}

// Play queue (Up Next) — client-side with localStorage
const playQueue = {
    _key: 'cv_play_queue',
    _items: null,
    _currentIdx: 0,
    get items() { if (!this._items) { try { this._items = JSON.parse(localStorage.getItem(this._key)) || []; } catch { this._items = []; } } return this._items; },
    save() { localStorage.setItem(this._key, JSON.stringify(this._items)); },
    add(item) { this.items.push(item); this.save(); },
    addNext(item) { this.items.splice(this._currentIdx + 1, 0, item); this.save(); },
    remove(idx) { this.items.splice(idx, 1); this.save(); },
    clear() { this._items = []; this.save(); },
    reorder(fromIdx, toIdx) { const [item] = this.items.splice(fromIdx, 1); this.items.splice(toIdx, 0, item); this.save(); },
    next() { if (this._currentIdx < this.items.length - 1) { this._currentIdx++; return this.items[this._currentIdx]; } return null; },
    current() { return this.items[this._currentIdx] || null; }
};

function addToQueue(mediaId, title, posterPath) { playQueue.add({ id: mediaId, title, poster_path: posterPath }); toast('Added to queue'); }
function playNext(mediaId, title, posterPath) { playQueue.addNext({ id: mediaId, title, poster_path: posterPath }); toast('Playing next'); }

// Shuffle play helper
function shufflePlay(items) {
    playQueue.clear();
    const shuffled = [...items].sort(() => Math.random() - 0.5);
    shuffled.forEach(i => playQueue.add(i));
    if (shuffled.length > 0) playMedia(shuffled[0].id, shuffled[0].title);
}

// Saved filter presets
async function loadFilterPresets(libraryId) {
    const url = libraryId ? '/filters?library_id=' + libraryId : '/filters';
    const d = await api('GET', url);
    return (d.success && d.data) ? d.data : [];
}

async function saveFilterPreset(name, filters, libraryId) {
    const body = { name, filters };
    if (libraryId) body.library_id = libraryId;
    const d = await api('POST', '/filters', body);
    if (d.success) toast('Filter preset saved');
    return d;
}

async function deleteFilterPreset(id) {
    const d = await api('DELETE', '/filters/' + id);
    if (d.success) toast('Filter preset deleted');
    return d;
}

// ──── Keyboard Navigation for Media Grids ────
function enableGridKeyNav(grid) {
    if (!grid) return;
    const cards = grid.querySelectorAll('.media-card[tabindex]');
    if (cards.length === 0) return;

    grid.addEventListener('keydown', function(e) {
        const focused = document.activeElement;
        if (!focused || !focused.classList.contains('media-card')) return;

        const allCards = [...grid.querySelectorAll('.media-card[tabindex]')];
        const idx = allCards.indexOf(focused);
        if (idx === -1) return;

        // Calculate columns from grid
        const gridStyle = getComputedStyle(grid);
        const cols = gridStyle.gridTemplateColumns.split(' ').length;
        let target = -1;

        switch(e.key) {
            case 'ArrowRight': target = Math.min(idx + 1, allCards.length - 1); break;
            case 'ArrowLeft': target = Math.max(idx - 1, 0); break;
            case 'ArrowDown': target = Math.min(idx + cols, allCards.length - 1); break;
            case 'ArrowUp': target = Math.max(idx - cols, 0); break;
            case 'Enter':
            case ' ':
                e.preventDefault();
                focused.click();
                return;
            default: return;
        }

        if (target !== -1 && target !== idx) {
            e.preventDefault();
            allCards[target].focus();
        }
    });
}

