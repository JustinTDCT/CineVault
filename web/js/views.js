// ──── Utilities ────
function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

// ──── Navigation ────
let _currentNav = { view: 'home', extra: null };
let _detailReturnNav = null;

function navigate(view, extra) {
    _currentNav = { view, extra: extra || null };
    selectionState.clear();
    closeUserDropdown();
    document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
    if (view === 'library' && extra) {
        const navItem = document.querySelector(`.nav-item[data-view="library"][data-id="${extra}"]`);
        if (navItem) navItem.classList.add('active');
    } else {
        const navItem = document.querySelector(`.nav-item[data-view="${view}"]`);
        if (navItem) navItem.classList.add('active');
    }
    switch(view) {
        case 'home': loadHomeView(); break;
        case 'libraries': loadLibrariesView(); break;
        case 'library': loadLibraryView(extra); break;
        case 'media': loadMediaTypeView(extra); break;
        case 'collections': loadCollectionsView(); break;
        case 'collection': loadCollectionDetailView(extra); break;
        case 'performers': loadPerformersView(); break;
        case 'tags': loadTagsView(); break;
        case 'studios': loadStudiosView(); break;
        case 'duplicates': loadDuplicatesView(); break;
        case 'settings': window.location.href = 'settings.html'; break;
        case 'admin': window.location.href = 'settings.html#admin'; break;
        case 'analytics': window.location.href = 'settings.html#analytics'; break;
        case 'profile': loadProfileView(); break;
        case 'stats': loadStatsView(); break;
        case 'genre': loadGenreHubView(extra); break;
        case 'decade': loadDecadeHubView(extra); break;
        case 'wrapped': loadWrappedView(extra); break;
        case 'requests': loadContentRequestsView(); break;
        case 'livetv': loadLiveTVView(); break;
    }
}

// Navigate back from media detail to the originating view.
// Falls back to the item's library if no prior context exists.
function navigateBack(libraryId) {
    if (_detailReturnNav && _detailReturnNav.view) {
        const r = _detailReturnNav;
        // Special cases that don't go through navigate()
        if (r.view === '__series') {
            loadSeriesDetail(r.extra);
            return;
        }
        navigate(r.view, r.extra);
    } else {
        navigate('library', libraryId);
    }
}

document.querySelectorAll('.nav-item[data-view]').forEach(item => {
    item.addEventListener('click', () => navigate(item.dataset.view, item.dataset.type));
});

let searchTimeout;
document.getElementById('searchInput').addEventListener('input', (e) => {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => { if (e.target.value.length >= 2) loadSearchView(e.target.value); }, 400);
});

// ──── Overlay Badge Helpers ────
const AUDIO_LABEL_MAP = {
    'truehd atmos': 'Atmos', 'truehd_atmos': 'Atmos', 'eac3 atmos': 'Atmos', 'eac3_atmos': 'Atmos',
    'dolby atmos': 'Atmos', 'atmos': 'Atmos',
    'truehd': 'TrueHD', 'dts-hd ma': 'DTS-HD MA', 'dts-hd': 'DTS-HD', 'dtshd': 'DTS-HD',
    'dts-x': 'DTS:X', 'dtsx': 'DTS:X', 'dts:x': 'DTS:X',
    'dts': 'DTS', 'eac3': 'EAC3', 'ac3': 'AC3', 'aac': 'AAC',
    'flac': 'FLAC', 'pcm': 'PCM', 'opus': 'Opus', 'vorbis': 'Vorbis', 'mp3': 'MP3',
    'pcm_s16le': 'PCM', 'pcm_s24le': 'PCM', 'pcm_s32le': 'PCM',
};
const EDITION_LABEL_MAP = {
    'directors_cut': "Director's Cut", 'directors cut': "Director's Cut",
    'extended': 'Extended', 'theatrical': 'Theatrical', 'unrated': 'Unrated',
    'imax': 'IMAX', 'remastered': 'Remastered', 'criterion': 'Criterion',
    'uncut': 'Uncut', 'special': 'Special', '3d': '3D',
};
const SOURCE_LABEL_MAP = {
    'remux': 'REMUX', 'bluray': 'Blu-Ray', 'blu-ray': 'Blu-Ray',
    'web-dl': 'WEB-DL', 'webdl': 'WEB-DL', 'webrip': 'WEBRip', 'web': 'WEB',
    'hdtv': 'HDTV', 'dvd': 'DVD', 'dvdrip': 'DVDRip', 'bdrip': 'BDRip',
    'brrip': 'BRRip', 'sdtv': 'SDTV', 'cam': 'CAM',
};

function buildResolutionLabel(item) {
    let res = (item.resolution || '').toUpperCase();
    if (!res) return '';
    // Normalize common forms
    if (res === '2160P' || res === '2160' || res === 'UHD') res = '4K';
    const hdr = item.hdr_format || item.dynamic_range || '';
    if (!hdr || hdr === 'SDR' || hdr === 'sdr') return res;
    const hdrUpper = hdr.toUpperCase();
    if (hdrUpper.includes('DOLBY VISION') || hdrUpper === 'DV' || hdrUpper.includes('DOLBY_VISION')) return res + ' DV';
    if (hdrUpper.includes('HDR10+') || hdrUpper === 'HDR10PLUS') return res + ' HDR10+';
    if (hdrUpper.includes('HDR10') || hdrUpper.includes('HDR')) return res + ' HDR';
    if (hdrUpper.includes('HLG')) return res + ' HLG';
    return res + ' ' + hdr;
}

function mapAudioLabel(item) {
    const raw = (item.audio_codec || '').toLowerCase().trim();
    if (!raw) return '';
    // Try exact match first, then partial
    if (AUDIO_LABEL_MAP[raw]) return AUDIO_LABEL_MAP[raw];
    for (const [key, label] of Object.entries(AUDIO_LABEL_MAP)) {
        if (raw.includes(key)) return label;
    }
    // Append channel info if available
    const base = raw.toUpperCase();
    if (item.audio_channels && item.audio_channels > 2) return base + ' ' + item.audio_channels + '.1';
    return base;
}

function mapEditionLabel(item) {
    const raw = (item.edition_type || '').toLowerCase().trim();
    if (!raw || raw === 'standard' || raw === 'unknown') return '';
    if (EDITION_LABEL_MAP[raw]) return EDITION_LABEL_MAP[raw];
    // Try partial match
    for (const [key, label] of Object.entries(EDITION_LABEL_MAP)) {
        if (raw.includes(key)) return label;
    }
    // Capitalize first letter as fallback
    return raw.charAt(0).toUpperCase() + raw.slice(1);
}

function mapSourceLabel(item) {
    const raw = (item.source_type || '').toLowerCase().trim();
    if (!raw) return '';
    if (SOURCE_LABEL_MAP[raw]) return SOURCE_LABEL_MAP[raw];
    for (const [key, label] of Object.entries(SOURCE_LABEL_MAP)) {
        if (raw.includes(key)) return label;
    }
    return raw.toUpperCase();
}

function renderOverlayBadges(item) {
    const p = overlayPrefs;
    const isMusic = item.media_type === 'music';
    let trBadges = '', tlBadges = '', blBadges = '', brBadges = '';

    if (isMusic) {
        if (p.audio_codec) {
            const audioLabel = mapAudioLabel(item);
            if (audioLabel) trBadges += `<span class="overlay-badge overlay-badge-audio">${audioLabel}</span>`;
        }
        let html = '';
        if (trBadges) html += `<div class="overlay-zone-tr">${trBadges}</div>`;
        return html;
    }

    // TOP-RIGHT: Resolution + HDR, Audio Codec
    if (p.resolution_hdr) {
        const resLabel = buildResolutionLabel(item);
        if (resLabel) {
            const isHDR = resLabel.includes('HDR') || resLabel.includes('DV') || resLabel.includes('HLG');
            trBadges += `<span class="overlay-badge ${isHDR ? 'overlay-badge-res-hdr' : 'overlay-badge-res'}">${resLabel}</span>`;
        }
    }
    if (p.audio_codec) {
        const audioLabel = mapAudioLabel(item);
        if (audioLabel) trBadges += `<span class="overlay-badge overlay-badge-audio">${audioLabel}</span>`;
    }

    // TOP-LEFT: Content Rating + Edition Type (single row)
    if (p.content_rating && item.content_rating) {
        tlBadges += `<span class="overlay-badge overlay-badge-content-rating">${item.content_rating}</span>`;
    }
    if (p.edition_type) {
        if (item.edition_count && item.edition_count > 1) {
            tlBadges += `<span class="overlay-badge overlay-badge-edition">${item.edition_count} Editions</span>`;
        } else {
            const edLabel = mapEditionLabel(item);
            if (edLabel) tlBadges += `<span class="overlay-badge overlay-badge-edition">${edLabel}</span>`;
        }
    }

    // BOTTOM-LEFT: Ratings
    if (p.ratings) {
        if (item.imdb_rating) blBadges += `<span class="overlay-badge overlay-badge-imdb">IMDb ${item.imdb_rating.toFixed(1)}</span>`;
        if (item.rt_rating) blBadges += `<span class="overlay-badge overlay-badge-rt">RT ${item.rt_rating}%</span>`;
        if (item.audience_score) blBadges += `<span class="overlay-badge overlay-badge-tmdb">AS ${item.audience_score}%</span>`;
    }

    // BOTTOM-RIGHT: Source Type + Multi-Part indicator
    if (p.source_type) {
        const srcLabel = mapSourceLabel(item);
        if (srcLabel) brBadges += `<span class="overlay-badge overlay-badge-source">${srcLabel}</span>`;
    }
    if (item.sister_part_count > 1) {
        brBadges += `<span class="overlay-badge overlay-badge-multipart">${item.sister_part_count} Parts</span>`;
    }

    let html = '';
    if (trBadges) html += `<div class="overlay-zone-tr">${trBadges}</div>`;
    if (tlBadges) html += `<div class="overlay-zone-tl">${tlBadges}</div>`;
    if (blBadges) html += `<div class="overlay-zone-bl">${blBadges}</div>`;
    if (brBadges) html += `<div class="overlay-zone-br">${brBadges}</div>`;
    return html;
}

// ──── Media Card (with expanded hover info) ────
function renderMediaCard(item) {
    const isMusic = item.media_type === 'music';
    const displayDur = item.sister_total_duration > 0 ? item.sister_total_duration : item.duration_seconds;
    const dur = displayDur ? Math.floor(displayDur/60)+'min' : '';
    const year = item.year || '';
    const res = item.resolution || '';
    let meta, hoverMeta, ratingBadge;
    if (isMusic) {
        const artist = item.artist_name || '';
        const album = item.album_title || '';
        meta = [artist, dur].filter(Boolean).join(' \u00b7 ');
        hoverMeta = [artist, album, year].filter(Boolean).join(' \u00b7 ');
        ratingBadge = '';
    } else {
        meta = [year, dur, res].filter(Boolean).join(' \u00b7 ');
        hoverMeta = [year, res, item.codec].filter(Boolean).join(' \u00b7 ');
        ratingBadge = item.rating ? `<span class="hover-rating-badge">&#11088; ${item.rating.toFixed(1)}</span>` : '';
    }
    const displayTitle = item.sister_group_name || item.title;
    const isSelected = selectionState.selectedIds.has(item.id);
    const sortAttr = item.sort_title ? ' data-sort-title="'+item.sort_title.replace(/"/g,'&amp;quot;')+'"' : '';
    const previewAttr = item.preview_path ? ' data-preview="'+item.preview_path+'"' : '';
    return `<div class="media-card${isSelected ? ' selected' : ''}" tabindex="0" data-media-id="${item.id}"${sortAttr}${previewAttr} onclick="handleCardClick('${item.id}', event)">
        <div class="media-poster" style="position:relative;">
            <div class="select-checkbox" onclick="handleCheckboxClick('${item.id}', event)">
                <svg viewBox="0 0 24 24"><polyline points="20 6 9 17 4 12"></polyline></svg>
            </div>
            ${item.poster_path ? '<img src="'+posterSrc(item.poster_path, item.updated_at)+'" alt="" loading="lazy">' : mediaIcon(item.media_type)}
            ${renderOverlayBadges(item)}
            <div class="media-card-hover-info">
                <div class="hover-title">${displayTitle}</div>
                <div class="hover-meta">${ratingBadge}<span>${hoverMeta}</span></div>
            </div>
            <div class="play-overlay"><div class="play-button" onclick="event.stopPropagation();playMedia('${item.id}','${(displayTitle||'').replace(/'/g,"\\'")}')">&#9654;</div></div>
        </div>
        <div class="media-info"><div class="media-title">${displayTitle}</div><div class="media-meta">${meta}</div></div>
    </div>`;
}

// StashApp-style video preview on hover
// After 3s hovering the card (excluding the play button), a muted looping MP4
// video fades in over the poster. Moving to the play button pauses/hides the
// preview so it stays click-accessible. Leaving the card cleans up entirely.
let _pvTimer = null;
let _pvCard = null;

function _pvShow(card) {
    const poster = card.querySelector('.media-poster');
    if (!poster || card.classList.contains('previewing')) return;
    let vid = card.querySelector('.preview-video');
    if (!vid) {
        vid = document.createElement('video');
        vid.className = 'preview-video';
        vid.src = card.dataset.preview;
        vid.muted = true;
        vid.loop = true;
        vid.playsInline = true;
        vid.setAttribute('playsinline', '');
        vid.preload = 'auto';
        poster.appendChild(vid);
    }
    vid.play().catch(() => {});
    card.classList.add('previewing');
}

function _pvHide(card) {
    const vid = card.querySelector('.preview-video');
    if (vid) {
        vid.pause();
        vid.removeAttribute('src');
        vid.load();
        vid.remove();
    }
    card.classList.remove('previewing');
}

function _pvStop() {
    clearTimeout(_pvTimer);
    _pvTimer = null;
    if (_pvCard) _pvHide(_pvCard);
    _pvCard = null;
}

function _pvStart(card) {
    if (_pvCard === card && _pvTimer) return;
    _pvStop();
    _pvCard = card;
    _pvTimer = setTimeout(() => { _pvTimer = null; _pvShow(card); }, 1500);
}

document.addEventListener('mouseenter', (e) => {
    if (!e.target || !e.target.classList) return;
    // Entering the play button: pause preview timer / hide active preview
    if (e.target.classList.contains('play-button')) {
        clearTimeout(_pvTimer);
        _pvTimer = null;
        if (_pvCard) _pvHide(_pvCard);
        return;
    }
    // Entering a card with preview data: start the 1.5s timer
    const card = e.target.closest ? e.target.closest('.media-card[data-preview]') : null;
    if (card) _pvStart(card);
}, true);

document.addEventListener('mouseleave', (e) => {
    if (!e.target || !e.target.classList) return;
    // Leaving the play button back to the card: restart timer
    if (e.target.classList.contains('play-button')) {
        const card = e.target.closest ? e.target.closest('.media-card[data-preview]') : null;
        if (card && card === _pvCard) {
            _pvTimer = setTimeout(() => { _pvTimer = null; _pvShow(card); }, 1500);
        }
        return;
    }
    // Leaving the card entirely: full cleanup
    const card = e.target.closest ? e.target.closest('.media-card[data-preview]') : null;
    if (card && card === _pvCard) _pvStop();
}, true);

// ──── Skeleton Generators ────
function skeletonCards(count) {
    return Array(count).fill('').map(() => `<div class="skeleton-card"><div class="skeleton skeleton-poster"></div><div class="skeleton skeleton-title"></div><div class="skeleton skeleton-meta"></div></div>`).join('');
}

// ──── Home View (with Hero Banner + Skeleton Loading) ────
async function loadHomeView() {
    const mc = document.getElementById('mainContent');
    // Show skeleton layout immediately
    mc.innerHTML = `
        <div class="skeleton skeleton-hero"></div>
        <div class="section-header"><h2 class="section-title">Continue Watching</h2></div>
        <div id="continueRow" class="continue-row skeleton-row">${skeletonCards(6)}</div>
        <div class="section-header"><h2 class="section-title">Recently Added</h2></div>
        <div class="media-grid" id="recentGrid">${skeletonCards(12)}</div>`;

    // Fetch all data concurrently
    const [cwResult, libsResult] = await Promise.allSettled([
        api('GET', '/watch/continue'),
        api('GET', '/libraries')
    ]);

    // Build hero banner + recently added from libraries
    let allItems = [];
    try {
        const libs = libsResult.status === 'fulfilled' ? libsResult.value : { success: false };
        if (libs.success && libs.data) {
            const homepageLibs = libs.data.filter(lib => lib.include_in_homepage !== false);
            const mediaPromises = homepageLibs.slice(0,5).map(lib => api('GET', '/libraries/'+lib.id+'/media'));
            const mediaResults = await Promise.allSettled(mediaPromises);
            for (const r of mediaResults) {
                if (r.status === 'fulfilled' && r.value.success && r.value.data && r.value.data.items)
                    allItems = allItems.concat(r.value.data.items);
            }
            allItems.sort((a,b) => new Date(b.added_at) - new Date(a.added_at));
        }
    } catch {}

    // Pick a hero item (random from top recently added items with posters)
    const heroPool = allItems.filter(i => i.poster_path).slice(0, 20);
    const heroItem = heroPool.length > 0 ? heroPool[Math.floor(Math.random() * Math.min(heroPool.length, 8))] : null;

    // Build hero banner HTML
    let heroHTML = '';
    if (heroItem) {
        const hMeta = [heroItem.year, heroItem.resolution, heroItem.codec].filter(Boolean).join(' \u00b7 ');
        const hDesc = heroItem.description ? heroItem.description.substring(0, 200) + (heroItem.description.length > 200 ? '...' : '') : '';
        heroHTML = `<div class="hero-banner">
            <div class="hero-banner-bg" style="background-image:url('${posterSrc(heroItem.poster_path, heroItem.updated_at)}')"></div>
            <div class="hero-banner-gradient"></div>
            <div class="hero-banner-content">
                <div class="hero-banner-title">${heroItem.title}</div>
                <div class="hero-banner-meta">
                    ${heroItem.rating ? '<span class="hero-rating">&#11088; '+heroItem.rating.toFixed(1)+'</span>' : ''}
                    <span>${hMeta}</span>
                </div>
                ${hDesc ? '<div class="hero-banner-desc">'+hDesc+'</div>' : ''}
                <div class="hero-banner-actions">
                    <button class="hero-btn-play" onclick="playMedia('${heroItem.id}','${(heroItem.title||'').replace(/'/g,"\\'")}')">&#9654; Play</button>
                    <button class="hero-btn-info" onclick="loadMediaDetail('${heroItem.id}')">&#9432; More Info</button>
                </div>
            </div>
        </div>`;
    }

    // Build continue watching with enhanced info
    let cwHTML = '';
    try {
        const cw = cwResult.status === 'fulfilled' ? cwResult.value : { success: false };
        if (cw.success && cw.data && cw.data.length > 0) {
            cwHTML = cw.data.map(wh => {
                const item = wh.media_item || {};
                const pct = wh.duration_seconds ? Math.round(wh.progress_seconds/wh.duration_seconds*100) : 0;
                const remainSec = wh.duration_seconds ? wh.duration_seconds - wh.progress_seconds : 0;
                const remainMin = Math.ceil(remainSec / 60);
                const timeLeft = remainMin > 0 ? remainMin + ' min left' : '';
                return `<div class="media-card" tabindex="0" onclick="playMedia('${item.id}','${(item.title||'').replace(/'/g,"\\'")}')">
                    <div class="media-poster" style="position:relative;">
                        ${item.poster_path ? '<img src="'+posterSrc(item.poster_path, item.updated_at)+'">' : mediaIcon(item.media_type||'movies')}
                        ${renderOverlayBadges(item)}
                        <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
                        ${timeLeft ? '<span class="cw-time-left">'+timeLeft+'</span>' : ''}
                        <button class="cw-remove" onclick="event.stopPropagation();removeContinue('${item.id}')" title="Remove">&#10005;</button>
                        <div class="play-overlay"><div class="play-button">&#9654;</div></div>
                    </div>
                    <div class="media-info"><div class="media-title">${item.title||'Unknown'}</div><div class="media-meta">${Math.floor(wh.progress_seconds/60)}/${wh.duration_seconds?Math.floor(wh.duration_seconds/60):'?'} min</div></div>
                </div>`;
            }).join('');
        } else cwHTML = '<div style="color:#5a6a7f;padding:20px;">No items in progress</div>';
    } catch { cwHTML = ''; }

    // Build recently added grid
    let recentHTML = '';
    if (allItems.length > 0) {
        recentHTML = allItems.slice(0,12).map(renderMediaCard).join('');
    } else {
        recentHTML = `<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">&#128253;</div><div class="empty-state-title">No media yet</div><p>Add libraries and scan them to populate your media</p><button class="btn-primary" style="margin-top:18px;" onclick="navigate('libraries')">Manage Libraries</button></div>`;
    }

    // Fetch engagement rows concurrently
    const [onDeckResult, watchlistResult, favoritesResult, trendingResult] = await Promise.allSettled([
        api('GET', '/watch/on-deck'),
        api('GET', '/watchlist'),
        api('GET', '/favorites'),
        api('GET', '/discover/trending')
    ]);

    // Build On Deck row
    let onDeckHTML = '';
    try {
        const od = onDeckResult.status === 'fulfilled' ? onDeckResult.value : { success: false };
        if (od.success && od.data && od.data.length > 0) {
            onDeckHTML = `<div class="engagement-row">
                <div class="engagement-row-header"><span class="engagement-row-title">On Deck</span><span class="engagement-row-count">${od.data.length} shows</span></div>
                <div class="continue-row">${od.data.map(d => `<div class="on-deck-card" onclick="playMedia('${d.episode_id}','${(d.episode_title||'').replace(/'/g,"\\'")}')">
                    <img class="on-deck-thumb" src="${posterSrc(d.episode_poster||d.show_poster,'')}" onerror="this.style.display='none'">
                    <div class="on-deck-info">
                        <div class="on-deck-show">${d.show_title}</div>
                        <div class="on-deck-ep">S${String(d.season_number||0).padStart(2,'0')}E${String(d.episode_number||0).padStart(2,'0')} · ${d.episode_title||'Episode'}</div>
                        ${d.duration ? '<div class="on-deck-meta">'+Math.round(d.duration/60)+' min</div>' : ''}
                    </div>
                </div>`).join('')}</div></div>`;
        }
    } catch {}

    // Build Watchlist row
    let watchlistHTML = '';
    try {
        const wl = watchlistResult.status === 'fulfilled' ? watchlistResult.value : { success: false };
        if (wl.success && wl.data && wl.data.length > 0) {
            watchlistHTML = `<div class="engagement-row">
                <div class="engagement-row-header"><span class="engagement-row-title">My Watchlist</span><span class="engagement-row-count">${wl.data.length} items</span></div>
                <div class="media-grid">${wl.data.slice(0,12).map(item => `<div class="media-card" onclick="loadMediaDetail('${item.media_item_id||item.tv_show_id||item.edition_group_id}')">
                    <div class="media-poster">${item.poster_path ? '<img src="'+posterSrc(item.poster_path,'')+'">' : '<span style="font-size:2rem;">&#128278;</span>'}</div>
                    <div class="media-info"><div class="media-title">${item.title}</div></div>
                </div>`).join('')}</div></div>`;
        }
    } catch {}

    // Build Favorites row
    let favoritesHTML = '';
    try {
        const fv = favoritesResult.status === 'fulfilled' ? favoritesResult.value : { success: false };
        if (fv.success && fv.data && fv.data.length > 0) {
            const mediaFavs = fv.data.filter(f => f.item_type === 'media' || f.item_type === 'show');
            if (mediaFavs.length > 0) {
                favoritesHTML = `<div class="engagement-row">
                <div class="engagement-row-header"><span class="engagement-row-title">Favorites</span><span class="engagement-row-count">${mediaFavs.length} items</span></div>
                <div class="media-grid">${mediaFavs.slice(0,12).map(item => `<div class="media-card" onclick="loadMediaDetail('${item.media_item_id||item.tv_show_id}')">
                    <div class="media-poster">${item.poster_path ? '<img src="'+posterSrc(item.poster_path,'')+'">' : '<span style="font-size:2rem;">&#10084;</span>'}</div>
                    <div class="media-info"><div class="media-title">${item.title}</div></div>
                </div>`).join('')}</div></div>`;
            }
        }
    } catch {}

    // Build Trending row
    let trendingHTML = '';
    try {
        const tr = trendingResult.status === 'fulfilled' ? trendingResult.value : { success: false };
        if (tr.success && tr.data && tr.data.length > 0) {
            trendingHTML = `<div class="engagement-row">
                <div class="engagement-row-header"><span class="engagement-row-title">&#128293; Trending This Week</span><span class="engagement-row-count">${tr.data.length} items</span></div>
                <div class="media-grid">${tr.data.slice(0,12).map(item => `<div class="media-card" onclick="loadMediaDetail('${item.id}')">
                    <div class="media-poster">${item.poster_path ? '<img src="'+posterSrc(item.poster_path, item.updated_at)+'">' : mediaIcon(item.media_type||'movies')}
                        <span class="trending-badge">${item.unique_viewers} viewer${item.unique_viewers !== 1 ? 's' : ''}</span>
                    </div>
                    <div class="media-info"><div class="media-title">${item.title}</div><div class="media-meta">${item.year||''} ${item.rating?'&#11088; '+item.rating.toFixed(1):''}</div></div>
                </div>`).join('')}</div></div>`;
        }
    } catch {}

    // Render everything at once (no flicker)
    mc.innerHTML = `
        ${heroHTML}
        <div class="section-header"><h2 class="section-title">Continue Watching</h2></div>
        <div id="continueRow" class="continue-row">${cwHTML}</div>
        ${onDeckHTML}
        ${trendingHTML}
        ${watchlistHTML}
        ${favoritesHTML}
        <div class="section-header"><h2 class="section-title">Recently Added</h2></div>
        <div class="media-grid" id="recentGrid">${recentHTML}</div>`;

    // Enable keyboard nav on the new grid
    enableGridKeyNav(document.getElementById('recentGrid'));
}

// ──── Remove from Continue Watching ────
async function removeContinue(mediaId) {
    const d = await api('DELETE', '/watch/continue/' + mediaId);
    if (d.success) { toast('Removed from Continue Watching'); loadHomeView(); }
    else toast(d.error || 'Failed to remove', 'error');
}

// ──── Media Detail ────
var _detailMediaId = null;

async function loadMediaDetail(id) {
    // Save return context so Back goes to where the user came from
    _detailReturnNav = { ..._currentNav };
    _detailMediaId = id;
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div> Loading...';
    const data = await api('GET', '/media/' + id);
    if (!data.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Media not found</div></div>'; return; }
    const m = data.data;
    const dur = m.duration_seconds ? formatDuration(m.duration_seconds) : '';
    const hdrTag = m.dynamic_range === 'HDR' ? (m.hdr_format || 'HDR') : '';
    const meta = [m.year, dur, m.resolution, m.codec, m.source_type, hdrTag].filter(Boolean).join(' \u00b7 ');

    // Build ratings row
    let ratingsHTML = '';
    const hasAnyRating = m.rating || m.imdb_rating || m.rt_rating != null || m.audience_score != null || m.metacritic_score;
    if (hasAnyRating) {
        ratingsHTML = '<div class="ratings-row">';
        if (m.rating) ratingsHTML += `<div class="rating-badge rating-tmdb"><span class="rating-icon">&#11088;</span><span class="rating-value">${m.rating.toFixed(1)}</span><span class="rating-label">TMDB</span></div>`;
        if (m.imdb_rating) ratingsHTML += `<div class="rating-badge rating-imdb"><span class="rating-icon">&#127902;</span><span class="rating-value">${m.imdb_rating.toFixed(1)}</span><span class="rating-label">IMDb</span></div>`;
        if (m.rt_rating != null) ratingsHTML += `<div class="rating-badge rating-rt"><span class="rating-icon">&#127813;</span><span class="rating-value">${m.rt_rating}%</span><span class="rating-label">Rotten Tomatoes</span></div>`;
        if (m.audience_score != null) ratingsHTML += `<div class="rating-badge rating-audience"><span class="rating-icon">&#128101;</span><span class="rating-value">${m.audience_score}%</span><span class="rating-label">Audience</span></div>`;
        if (m.metacritic_score) ratingsHTML += `<div class="rating-badge" style="border-color:rgba(255,204,0,0.3);"><span class="rating-icon">&#127942;</span><span class="rating-value">${m.metacritic_score}</span><span class="rating-label">Metacritic</span></div>`;
        ratingsHTML += '</div>';
    }

    // Multi-country content ratings display (filtered by user region if set)
    let countryRatingsHTML = '';
    if (m.content_ratings_json) {
        try {
            const cr = JSON.parse(m.content_ratings_json);
            let entries = Object.entries(cr);
            if (_userRegion) {
                entries = entries.filter(([country]) => country === _userRegion);
            }
            if (entries.length > 0) {
                countryRatingsHTML = '<div class="multi-rating-row">';
                entries.forEach(([country, rating]) => {
                    countryRatingsHTML += `<span class="rating-country-badge"><span class="country-code">${country}</span> ${rating}</span>`;
                });
                countryRatingsHTML += '</div>';
            }
        } catch(e) {}
    }

    const detailTitle = m.sister_group_name || m.title;

    mc.innerHTML = `
        <div class="detail-hero">
            <div class="detail-poster">${m.poster_path ? '<img src="'+posterSrc(m.poster_path, m.updated_at)+'">' : mediaIcon(m.media_type)}</div>
            <div class="detail-info">
                <h1>${detailTitle}</h1>
                ${m.sister_part_count > 1 ? '<span class="tag tag-multipart">'+m.sister_part_count+' Parts</span>' : ''}
                ${(m.edition_type && m.edition_type !== 'Theatrical' && m.edition_type !== 'Standard' && m.edition_type !== '' && m.edition_type !== 'unknown') ? '<span class="edition-appendix-badge">'+m.edition_type+' Edition</span>' : ''}
                <div class="meta-row">${meta}</div>
                ${m.description ? '<p class="description">'+m.description+'</p>' : ''}
                <div id="editionAppendixDetails" class="edition-appendix-details"></div>
                <div id="detailGenreTags" class="genre-tags"></div>
                ${ratingsHTML}
                ${countryRatingsHTML}
                <div class="detail-actions">
                    <button class="btn-primary" onclick="playMedia('${m.id}','${detailTitle.replace(/'/g,"\\'")}')">&#9654; Play${m.sister_part_count > 1 ? ' Part 1' : ''}</button>
                    ${m.trailer_url ? '<button class="btn-secondary" onclick="watchTrailer(\''+m.trailer_url.replace(/'/g,"\\'")+'\',\''+m.title.replace(/'/g,"\\'")+'\')">&#127909; Trailer</button>' : ''}
                    ${m.media_type === 'movie' ? '<button class="btn-secondary" onclick="playCinemaMode(\''+m.id+'\',\''+m.title.replace(/'/g,"\\'")+'\')">&#127910; Cinema</button>' : ''}
                    <button class="btn-secondary" onclick="createSyncSession('${m.id}')">&#128101; Watch Together</button>
                    <button class="btn-secondary" id="detailWatchlist" onclick="toggleWatchlist('${m.id}',this)">&#128278; Watchlist</button>
                    <button class="btn-secondary" id="detailFavorite" onclick="toggleFavorite('${m.id}',this)">&#10084; Favorite</button>
                    <button class="btn-secondary" onclick="addToQueue('${m.id}','${m.title.replace(/'/g,"\\'")}','${(m.poster_path||'').replace(/'/g,"\\'")}')">&#9654;&#9654; Queue</button>
                    <button class="btn-secondary" onclick="musicPlayer.enqueue([{id:'${m.id}',title:'${m.title.replace(/'/g,"\\'")}',artist:'${(m.artist||'').replace(/'/g,"\\'")}',duration_seconds:${m.duration_seconds||0}}]);musicPlayer.currentIndex=musicPlayer.queue.length-1;musicPlayer.playTrack();">&#127925; Music Play</button>
                    ${(m.media_type === 'comics' || m.media_type === 'ebooks' || (m.file_name && /\.(cbz|cbr|epub|pdf)$/i.test(m.file_name))) ? '<button class="btn-secondary" onclick="openReader(\''+m.id+'\')">&#128214; Read</button>' : ''}
                    <button class="btn-secondary" onclick="openEditModal('${m.id}')">&#9998; Edit</button>
                    <button class="btn-secondary" onclick="showAddToCollectionPicker('${m.id}')">&#128218; + Collection</button>
                </div>
                <div id="detailUserRating" style="margin-top:8px;"></div>
                <div style="margin-bottom:10px;">
                    ${m.metadata_locked ? '<span class="lock-badge locked">&#128274; Metadata Locked</span>' : ''}
                    <span class="tag tag-cyan">${MEDIA_LABELS[m.media_type]||m.media_type}</span>
                    ${m.edition_count > 1 ? '<span class="tag" style="background:rgba(168,85,247,0.3);color:#d4a5ff;">Multiple Editions</span>' : ''}
                    ${m.file_size ? '<span class="tag tag-purple">'+(m.file_size/1024/1024).toFixed(0)+' MB</span>' : ''}
                    ${m.source_type ? '<span class="tag tag-blue">'+m.source_type.toUpperCase()+'</span>' : ''}
                    ${m.dynamic_range === 'HDR' ? '<span class="tag tag-gold">'+(m.hdr_format || 'HDR')+'</span>' : ''}
                    ${m.audio_codec ? '<span class="tag tag-green">'+m.audio_codec+'</span>' : ''}
                    ${m.bitrate ? '<span class="tag tag-orange">'+(m.bitrate/1000).toFixed(0)+' kbps</span>' : ''}
                </div>
                <div class="detail-tabs">
                    <button class="detail-tab active" onclick="showDetailTab(this,'info','${m.id}')">Info</button>
                    ${m.sister_part_count > 1 ? '<button class="detail-tab" onclick="showDetailTab(this,\'parts\',\''+m.id+'\')">Parts</button>' : ''}
                    <button class="detail-tab" onclick="showDetailTab(this,'cast','${m.id}')">Cast</button>
                    <button class="detail-tab" onclick="showDetailTab(this,'tags-tab','${m.id}')">Tags</button>
                    ${m.edition_count > 1 ? '<button class="detail-tab" onclick="showDetailTab(this,\'editions\',\''+m.id+'\')">Editions</button>' : ''}
                    <button class="detail-tab" onclick="showDetailTab(this,'metadata','${m.id}')">Metadata</button>
                    <button class="detail-tab" onclick="showDetailTab(this,'chapters','${m.id}')">Chapters</button>
                    <button class="detail-tab" onclick="showDetailTab(this,'extras','${m.id}')">Extras</button>
                    <button class="detail-tab" onclick="showDetailTab(this,'segments','${m.id}')">Segments</button>
                    <button class="detail-tab" onclick="showDetailTab(this,'file','${m.id}')">File</button>
                </div>
                <div class="detail-tab-content" id="detailTabContent">
                    <table style="width:100%;font-size:0.85rem;">
                        <tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">File</td><td>${m.file_name}</td></tr>
                        <tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Path</td><td style="word-break:break-all;">${m.file_path}</td></tr>
                        ${m.width ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Resolution</td><td>'+m.width+'x'+m.height+'</td></tr>' : ''}
                        ${m.codec ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Video Codec</td><td>'+m.codec+'</td></tr>' : ''}
                        ${m.source_type ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Source</td><td>'+m.source_type+'</td></tr>' : ''}
                        ${m.dynamic_range && m.dynamic_range !== 'SDR' ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Dynamic Range</td><td>'+m.dynamic_range+(m.hdr_format ? ' ('+m.hdr_format+')' : '')+'</td></tr>' : ''}
                        ${m.custom_notes ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Notes</td><td>'+m.custom_notes+'</td></tr>' : ''}
                        ${m.mal_id ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">MyAnimeList</td><td><a href="https://myanimelist.net/anime/'+m.mal_id+'" target="_blank" style="color:#00D9FF;">MAL #'+m.mal_id+'</a></td></tr>' : ''}
                        ${m.anilist_id ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">AniList</td><td><a href="https://anilist.co/anime/'+m.anilist_id+'" target="_blank" style="color:#00D9FF;">AL #'+m.anilist_id+'</a></td></tr>' : ''}
                        ${m.anime_season ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Anime Season</td><td>'+m.anime_season+'</td></tr>' : ''}
                        ${m.sub_or_dub ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Sub / Dub</td><td>'+m.sub_or_dub+'</td></tr>' : ''}
                        <tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Added</td><td>${new Date(m.added_at).toLocaleString()}</td></tr>
                    </table>
                </div>
            </div>
        </div>
        <button class="btn-secondary" onclick="navigateBack('${m.library_id}')">&#8592; Back</button>`;

    // Load genre tags for this item
    loadMediaGenreTags(id);

    // Load engagement state (watchlist, favorite, user rating, community rating)
    (async () => {
        const [wlCheck, favCheck, ratingData, communityData] = await Promise.allSettled([
            api('GET', '/watchlist/' + id + '/check'),
            api('GET', '/favorites/' + id + '/check'),
            api('GET', '/media/' + id + '/rating'),
            api('GET', '/media/' + id + '/community-rating')
        ]);
        const wlBtn = document.getElementById('detailWatchlist');
        if (wlBtn && wlCheck.status === 'fulfilled' && wlCheck.value.success && wlCheck.value.data && wlCheck.value.data.in_watchlist) wlBtn.classList.add('active');
        const favBtn = document.getElementById('detailFavorite');
        if (favBtn && favCheck.status === 'fulfilled' && favCheck.value.success && favCheck.value.data && favCheck.value.data.favorited) favBtn.classList.add('active');
        const ratingEl = document.getElementById('detailUserRating');
        if (ratingEl) {
            const ur = ratingData.status === 'fulfilled' ? ratingData.value : null;
            const currentRating = (ur && ur.success && ur.data && ur.data.rating != null) ? ur.data.rating : null;
            let html = buildStarRating(id, currentRating);
            const cr = communityData.status === 'fulfilled' ? communityData.value : null;
            if (cr && cr.success && cr.data && cr.data.count > 0) {
                html += ` <span class="community-rating"><span class="avg">${cr.data.average.toFixed(1)}</span> (${cr.data.count} ${cr.data.count===1?'vote':'votes'})</span>`;
            }
            ratingEl.innerHTML = html;
        }
    })();

    // Load edition appendix details from cache server if this is a non-standard edition
    if (m.edition_type && m.edition_type !== 'Theatrical' && m.edition_type !== 'Standard' && m.edition_type !== '' && m.edition_type !== 'unknown') {
        loadEditionAppendix(id, m.edition_type);
    }
}

async function loadEditionAppendix(mediaId, editionType) {
    const container = document.getElementById('editionAppendixDetails');
    if (!container) return;
    try {
        const edRes = await api('GET', '/media/' + mediaId + '/editions');
        if (!edRes.success || !edRes.data.cache_editions || edRes.data.cache_editions.length === 0) return;
        // Find matching cache edition by type (case-insensitive)
        const et = editionType.toLowerCase().trim();
        const match = edRes.data.cache_editions.find(ce => {
            const ct = (ce.edition_type || '').toLowerCase().trim();
            return ct === et || ct.includes(et) || et.includes(ct);
        });
        if (!match) return;
        let parts = [];
        if (match.overview) parts.push('<p class="edition-appendix-overview">' + escapeHtml(match.overview) + '</p>');
        if (match.new_content_summary) parts.push('<p class="edition-appendix-content"><strong>New content:</strong> ' + escapeHtml(match.new_content_summary) + '</p>');
        if (parts.length > 0) container.innerHTML = parts.join('');
    } catch(e) {}
}

async function loadMediaGenreTags(mediaId) {
    const tagData = await api('GET', '/media/' + mediaId + '/tags');
    const container = document.getElementById('detailGenreTags');
    if (!container) return;
    if (tagData.success && tagData.data && tagData.data.length > 0) {
        const genreTags = tagData.data.filter(t => t.category === 'genre');
        const moodTags = tagData.data.filter(t => t.category === 'mood');
        let html = '';
        if (genreTags.length > 0) {
            html += genreTags.map(t => `<span class="genre-tag">${t.name}</span>`).join('');
        }
        if (moodTags.length > 0) {
            html += moodTags.map(t => `<span class="genre-tag mood-tag">${t.name}</span>`).join('');
        }
        container.innerHTML = html;
    }
}

async function showDetailTab(btn, tab, mediaId) {
    btn.parentElement.querySelectorAll('.detail-tab').forEach(t => t.classList.remove('active'));
    btn.classList.add('active');
    const tc = document.getElementById('detailTabContent');
    if (!tc) return;
    if (tab === 'tags-tab') {
        tc.innerHTML = '<div class="spinner"></div>';
        const [tagRes, allTagsRes] = await Promise.all([
            api('GET', '/media/' + mediaId + '/tags'),
            api('GET', '/tags')
        ]);
        const assigned = (tagRes.success && tagRes.data) ? tagRes.data : [];
        const allTags = (allTagsRes.success && allTagsRes.data) ? allTagsRes.data : [];
        const assignedIds = new Set(assigned.map(t => t.id));
        let html = '<div class="genre-tags" id="mediaTagsList">';
        if (assigned.length > 0) {
            html += assigned.map(t => `<span class="genre-tag">${t.name} <span style="cursor:pointer;margin-left:4px;opacity:0.6;" onclick="removeTagFromMedia('${mediaId}','${t.id}')">&#10005;</span></span>`).join('');
        } else {
            html += '<span style="color:#5a6a7f;">No tags assigned</span>';
        }
        html += '</div>';
        const isAdmin = currentUser && currentUser.role === 'admin';
        if (isAdmin && allTags.length > 0) {
            const available = allTags.filter(t => !assignedIds.has(t.id));
            html += '<div style="margin-top:12px;display:flex;gap:8px;align-items:center;"><select id="tagAssignSelect"><option value="">Add tag...</option>';
            available.forEach(t => { html += `<option value="${t.id}">${t.name} (${t.category})</option>`; });
            html += '</select><button class="btn-primary btn-small" onclick="addTagToMedia(\''+mediaId+'\')">+ Add</button></div>';
        }
        tc.innerHTML = html;
    } else if (tab === 'info') {
        loadMediaDetail(mediaId);
    } else if (tab === 'cast') {
        tc.innerHTML = '<div class="spinner"></div>';
        const castRes = await api('GET', '/media/' + mediaId + '/cast');
        if (castRes.success && castRes.data && castRes.data.length > 0) {
            const all = castRes.data.map(c => {
                const subtitle = c.role === 'actor' ? (c.character_name || '') : c.role;
                return `<div class="cast-card" onclick="loadPerformerDetail('${c.performer_id}')"><div class="person-avatar">${c.photo_path ? '<img src="'+c.photo_path+'">' : '&#128100;'}</div><div class="person-name">${c.name}</div><div class="person-role">${subtitle}</div></div>`;
            }).join('');
            tc.innerHTML = `<div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:14px;"><h4 style="color:#e5e5e5;margin:0;">Cast &amp; Crew</h4></div><div class="cast-row"><button class="cast-row-arrow left" onclick="document.getElementById('castScroll').scrollLeft-=400">&#8249;</button><div class="cast-row-scroll" id="castScroll">${all}</div><button class="cast-row-arrow right" onclick="document.getElementById('castScroll').scrollLeft+=400">&#8250;</button></div>`;
        } else {
            tc.innerHTML = '<p style="color:#5a6a7f;">No cast information available</p>';
        }
    } else if (tab === 'editions') {
        tc.innerHTML = '<div class="spinner"></div>';
        const edRes = await api('GET', '/media/' + mediaId + '/editions');
        let html = '';
        if (edRes.success && edRes.data.has_editions && edRes.data.editions && edRes.data.editions.length > 0) {
            const eds = edRes.data.editions;
            const rows = eds.map(e => {
                const dur = e.duration_seconds ? formatDuration(e.duration_seconds) : '-';
                const res = e.resolution || '-';
                const codec = e.codec || '-';
                const size = e.file_size ? (e.file_size/1024/1024).toFixed(0)+' MB' : '-';
                const audio = e.audio_codec || '-';
                const src = e.source_type || '-';
                const dr = e.dynamic_range === 'HDR' ? (e.hdr_format || 'HDR') : (e.dynamic_range || 'SDR');
                const defBadge = e.is_default ? ' <span class="edition-tab-default">Default</span>' : '';
                return `<tr>
                    <td>${e.edition_type}${defBadge}</td>
                    <td>${dur}</td><td>${res}</td><td>${codec}</td><td>${audio}</td><td>${src}</td><td>${dr}</td><td>${size}</td>
                    <td class="edition-tab-actions">
                        <button class="edition-tab-view" onclick="loadMediaDetail('${e.media_item_id}')" title="View this edition">&#128065; View</button>
                        <button class="edition-tab-play" onclick="playMedia('${e.media_item_id}','${e.title.replace(/'/g,"\\'")}')">&#9654; Play</button>
                    </td>
                </tr>`;
            }).join('');
            html += `<h4 class="edition-tab-heading">Your Editions (${eds.length})</h4>
                <table class="edition-tab-table">
                    <thead><tr><th>Edition</th><th>Runtime</th><th>Resolution</th><th>Codec</th><th>Audio</th><th>Source</th><th>DR</th><th>Size</th><th></th></tr></thead>
                    <tbody>${rows}</tbody>
                </table>`;
        } else {
            html += '<p style="color:#5a6a7f;">No local editions found for this item.</p>';
        }
        // Cache server AI-discovered editions
        if (edRes.success && edRes.data.cache_editions && edRes.data.cache_editions.length > 0) {
            const ce = edRes.data.cache_editions;
            html += `<div class="cache-editions-section">
                <h4 class="edition-tab-heading">Known Editions <span class="cache-editions-badge">AI</span></h4>
                <p class="cache-editions-note">These editions are known to exist for this title, discovered via AI metadata enrichment.</p>
                <ul class="known-editions-list">`;
            for (let i = 0; i < ce.length; i++) {
                const ed = ce[i];
                const runtime = ed.runtime ? ed.runtime + ' min' : '';
                const extra = ed.additional_runtime ? ' (+' + ed.additional_runtime + ' min)' : '';
                const year = ed.edition_release_year ? '(' + ed.edition_release_year + ')' : '';
                const overview = ed.overview || '';
                const contentSummary = ed.new_content_summary || '';
                // Build detail content for expand
                let detailParts = [];
                if (runtime) detailParts.push('<strong>Runtime:</strong> ' + runtime + extra);
                if (year) detailParts.push('<strong>Year:</strong> ' + ed.edition_release_year);
                if (ed.content_rating) detailParts.push('<strong>Rating:</strong> ' + escapeHtml(ed.content_rating));
                if (ed.known_resolutions) {
                    let resolutions = ed.known_resolutions;
                    if (typeof resolutions === 'string') { try { resolutions = JSON.parse(resolutions); } catch(e) { resolutions = []; } }
                    if (Array.isArray(resolutions) && resolutions.length > 0) detailParts.push('<strong>Resolutions:</strong> ' + resolutions.map(r => escapeHtml(r)).join(', '));
                }
                if (overview) detailParts.push('<strong>Overview:</strong> ' + escapeHtml(overview));
                if (contentSummary) detailParts.push('<strong>New content:</strong> ' + escapeHtml(contentSummary));
                if (ed.verified) detailParts.push('<span class="edition-verified">' + (ed.verification_source ? escapeHtml(ed.verification_source) : 'Verified') + '</span>');
                const hasDetails = detailParts.length > 0;
                const label = escapeHtml(ed.edition_type) + (runtime ? ' &mdash; ' + runtime + extra : '') + (year ? ' ' + year : '');
                html += `<li class="known-edition-item">
                    <div class="known-edition-row">
                        <span class="known-edition-name">${label}</span>
                        ${hasDetails ? `<a class="known-edition-toggle" href="javascript:void(0)" onclick="toggleKnownEdition(this)" data-idx="${i}">Show details</a>` : ''}
                    </div>
                    ${hasDetails ? `<div class="known-edition-details" id="knownEdDetail_${i}" style="display:none;">${detailParts.join('<br>')}</div>` : ''}
                </li>`;
            }
            html += '</ul></div>';
        }
        tc.innerHTML = html;
    } else if (tab === 'parts') {
        tc.innerHTML = '<div class="spinner"></div>';
        const mediaRes = await api('GET', '/media/' + mediaId);
        if (mediaRes.success && mediaRes.data.sister_group_id) {
            const sgRes = await api('GET', '/sisters/' + mediaRes.data.sister_group_id);
            if (sgRes.success && sgRes.data.members && sgRes.data.members.length > 0) {
                const parts = sgRes.data.members;
                const rows = parts.map((p, idx) => {
                    const dur = p.duration_seconds ? formatDuration(p.duration_seconds) : '-';
                    const res = p.resolution || '-';
                    const codec = p.codec || '-';
                    const size = p.file_size ? (p.file_size/1024/1024).toFixed(0)+' MB' : '-';
                    return `<tr>
                        <td>Part ${idx + 1}</td>
                        <td>${p.title}</td>
                        <td>${dur}</td><td>${res}</td><td>${codec}</td><td>${size}</td>
                        <td><button class="edition-tab-play" onclick="playMedia('${p.id}','${(p.title||'').replace(/'/g,"\\'")}')">&#9654; Play</button></td>
                    </tr>`;
                }).join('');
                const totalDur = parts.reduce((s, p) => s + (p.duration_seconds || 0), 0);
                tc.innerHTML = `<h4 class="edition-tab-heading">Parts (${parts.length}) &mdash; Total: ${formatDuration(totalDur)}</h4>
                    <table class="edition-tab-table">
                        <thead><tr><th>#</th><th>Title</th><th>Runtime</th><th>Resolution</th><th>Codec</th><th>Size</th><th></th></tr></thead>
                        <tbody>${rows}</tbody>
                    </table>`;
            } else {
                tc.innerHTML = '<p style="color:#5a6a7f;">No parts found.</p>';
            }
        } else {
            tc.innerHTML = '<p style="color:#5a6a7f;">No parts found.</p>';
        }
    } else if (tab === 'metadata') {
        tc.innerHTML = '<div class="spinner"></div>';
        const data = await api('GET', '/media/' + mediaId);
        if (data.success) {
            const m = data.data;
            let ids = null;
            try { if (m.external_ids) ids = JSON.parse(m.external_ids); } catch(e) {}

            let html = '<div class="metadata-tab-badges">';
            // Cache server badge
            if (ids && ids.cache_server) {
                html += '<span class="tag tag-green">Cache Server</span>';
            } else {
                html += '<span class="tag" style="background:rgba(100,116,139,0.15);color:#94a3b8;border:1px solid rgba(100,116,139,0.2);">Direct API</span>';
            }
            // Source badge
            if (ids && ids.source) {
                const srcLabel = {tmdb:'TMDB',porndb:'ThePornDB',musicbrainz:'MusicBrainz',openlibrary:'OpenLibrary'}[ids.source] || ids.source;
                html += '<span class="tag tag-cyan">Source: ' + srcLabel + '</span>';
            }
            html += '</div>';

            // External IDs section
            const idRows = [];
            if (ids) {
                if (ids.tmdb_id) idRows.push({name:'TMDB', id:ids.tmdb_id, url:'https://www.themoviedb.org/movie/'+ids.tmdb_id});
                if (ids.imdb_id) idRows.push({name:'IMDB', id:ids.imdb_id, url:'https://www.imdb.com/title/'+ids.imdb_id+'/'});
                if (ids.tpdb_id) idRows.push({name:'ThePornDB', id:ids.tpdb_id, url:'https://theporndb.net/movies/'+ids.tpdb_id});
                if (ids.musicbrainz_id) idRows.push({name:'MusicBrainz', id:ids.musicbrainz_id, url:'https://musicbrainz.org/release/'+ids.musicbrainz_id});
                if (ids.openlibrary_id) idRows.push({name:'OpenLibrary', id:ids.openlibrary_id, url:'https://openlibrary.org/works/'+ids.openlibrary_id});
            }

            html += '<div class="metadata-tab-section">';
            html += '<h4 class="metadata-tab-heading">External IDs</h4>';
            if (idRows.length > 0) {
                html += '<table class="metadata-ids-table">';
                html += '<thead><tr><th>Service</th><th>External ID</th></tr></thead><tbody>';
                idRows.forEach(r => {
                    html += '<tr><td class="metadata-ids-service">' + r.name + '</td>';
                    html += '<td><a href="' + r.url + '" target="_blank" rel="noopener" class="metadata-ids-link">' + r.id + '</a></td></tr>';
                });
                html += '</tbody></table>';
            } else {
                html += '<p style="color:var(--text-muted);">No external IDs stored. Run a metadata refresh or identify this item to populate.</p>';
            }
            html += '</div>';

            // Technical metadata section
            // ── Unified Cache Data Section ──
            let cacheSection = '';

            // Metacritic score
            if (m.metacritic_score) {
                const mcColor = m.metacritic_score >= 75 ? '#4ade80' : m.metacritic_score >= 50 ? '#fbbf24' : '#ef4444';
                cacheSection += `<tr><td class="metadata-tech-label">Metacritic</td><td class="metadata-tech-value"><span style="color:${mcColor};font-weight:600;">${m.metacritic_score}/100</span></td></tr>`;
            }

            // Multi-country content ratings
            if (m.content_ratings_json) {
                try {
                    const cr = JSON.parse(m.content_ratings_json);
                    let badges = '<div class="multi-rating-row">';
                    Object.entries(cr).forEach(([country, rating]) => {
                        badges += `<span class="rating-country-badge"><span class="country-code">${country}</span> ${rating}</span>`;
                    });
                    badges += '</div>';
                    cacheSection += `<tr><td class="metadata-tech-label">Content Ratings</td><td class="metadata-tech-value">${badges}</td></tr>`;
                } catch(e) {}
            }

            // Trailers
            if (m.trailers_json) {
                try {
                    const trailers = JSON.parse(m.trailers_json);
                    if (Array.isArray(trailers) && trailers.length > 0) {
                        let tLinks = trailers.slice(0,5).map(t =>
                            `<a href="${t.url}" target="_blank" style="color:#00D9FF;text-decoration:none;margin-right:8px;">${escapeHtml(t.name || 'Trailer')} <span style="color:#5a6a7f;">(${t.source || ''})</span></a>`
                        ).join('<br>');
                        cacheSection += `<tr><td class="metadata-tech-label">Trailers</td><td class="metadata-tech-value">${tLinks}</td></tr>`;
                    }
                } catch(e) {}
            }

            if (cacheSection) {
                html += '<div class="metadata-tab-section">';
                html += '<h4 class="metadata-tab-heading">Extended Metadata</h4>';
                html += '<table class="metadata-tech-table">' + cacheSection + '</table>';
                html += '</div>';
            }

            const techRows = [];
            if (m.source_type) techRows.push({k:'Source', v:m.source_type});
            if (m.dynamic_range && m.dynamic_range !== 'SDR') techRows.push({k:'Dynamic Range', v:m.dynamic_range + (m.hdr_format ? ' ('+m.hdr_format+')' : '')});
            if (m.custom_notes) techRows.push({k:'Custom Notes', v:m.custom_notes});
            const ctRaw = m.custom_tags ? (typeof m.custom_tags === 'string' ? JSON.parse(m.custom_tags || '{}') : m.custom_tags) : {};
            const ctArr = ctRaw.tags || [];
            if (ctArr.length > 0) techRows.push({k:'Custom Tags', v:ctArr.map(t=>'<span class="tag tag-blue">'+t+'</span>').join(' ')});
            if (techRows.length > 0) {
                html += '<div class="metadata-tab-section">';
                html += '<h4 class="metadata-tab-heading">Technical &amp; Custom Metadata</h4>';
                html += '<table class="metadata-tech-table">';
                techRows.forEach(r => {
                    html += '<tr><td class="metadata-tech-label">'+r.k+'</td><td class="metadata-tech-value">'+r.v+'</td></tr>';
                });
                html += '</table>';
                html += '</div>';
            }

            tc.innerHTML = html;
        }
    } else if (tab === 'chapters') {
        tc.innerHTML = '<div class="spinner"></div>';
        const chapRes = await api('GET', '/media/' + mediaId + '/chapters');
        const chapters = (chapRes.success && chapRes.data) ? chapRes.data : [];
        if (chapters.length > 0) {
            let html = '<table style="width:100%;font-size:0.85rem;">';
            html += '<thead><tr><th style="text-align:left;padding:6px 12px 6px 0;color:#8a9bae;">Title</th><th style="text-align:left;padding:6px 12px;color:#8a9bae;">Start</th><th style="text-align:left;padding:6px 12px;color:#8a9bae;">End</th><th></th></tr></thead><tbody>';
            chapters.forEach(ch => {
                html += `<tr>
                    <td style="padding:6px 12px 6px 0;color:#e5e5e5;">${ch.title || 'Chapter'}</td>
                    <td style="padding:6px 12px;">${formatTime(ch.start_seconds)}</td>
                    <td style="padding:6px 12px;">${ch.end_seconds ? formatTime(ch.end_seconds) : '-'}</td>
                    <td style="padding:6px 0;"><button class="btn-secondary btn-small" onclick="playMedia('${mediaId}','');seekToTime(${ch.start_seconds})">&#9654; Play</button></td>
                </tr>`;
            });
            html += '</tbody></table>';
            tc.innerHTML = html;
        } else {
            tc.innerHTML = '<p style="color:#5a6a7f;">No chapters found for this item.</p>';
        }
    } else if (tab === 'extras') {
        tc.innerHTML = '<div class="spinner"></div>';
        const extrasRes = await api('GET', '/media/' + mediaId + '/extras');
        if (extrasRes.success && extrasRes.data && extrasRes.data.length > 0) {
            const extras = extrasRes.data;
            const grouped = {};
            extras.forEach(e => {
                const t = e.extra_type || 'other';
                if (!grouped[t]) grouped[t] = [];
                grouped[t].push(e);
            });
            let html = '';
            const typeLabels = {trailer:'Trailers',featurette:'Featurettes','behind-the-scenes':'Behind the Scenes','deleted-scene':'Deleted Scenes',interview:'Interviews',short:'Shorts',other:'Other'};
            for (const [type, items] of Object.entries(grouped)) {
                html += '<h4 style="color:#e5e5e5;margin:12px 0 8px;">' + (typeLabels[type] || type) + '</h4>';
                html += '<div class="extras-list">';
                items.forEach(e => {
                    const dur = e.duration_seconds ? formatDuration(e.duration_seconds) : '';
                    html += `<div class="extras-item" onclick="playMedia('${e.id}','${(e.title||'').replace(/'/g,"\\'")}')">
                        <span class="extras-icon">&#9654;</span>
                        <span class="extras-title">${e.title || e.file_name}</span>
                        ${dur ? '<span class="extras-duration">'+dur+'</span>' : ''}
                    </div>`;
                });
                html += '</div>';
            }
            tc.innerHTML = html;
        } else {
            tc.innerHTML = '<p style="color:#5a6a7f;">No extras found</p>';
        }
    } else if (tab === 'segments') {
        tc.innerHTML = '<div class="spinner"></div>';
        const segRes = await api('GET', '/media/' + mediaId + '/segments');
        const isAdmin = currentUser && currentUser.role === 'admin';
        const segs = (segRes.success && segRes.data) ? segRes.data : [];

        if (segs.length > 0) {
            const typeLabel = { intro: 'Intro', credits: 'Credits', recap: 'Recap', preview: 'Preview' };
            const sourceLabel = { auto: 'Auto-detected', manual: 'Manual', community: 'Community' };
            let html = '<table style="width:100%;font-size:0.85rem;">';
            html += '<thead><tr><th style="text-align:left;padding:6px 12px 6px 0;color:#8a9bae;">Type</th><th style="text-align:left;padding:6px 12px;color:#8a9bae;">Start</th><th style="text-align:left;padding:6px 12px;color:#8a9bae;">End</th><th style="text-align:left;padding:6px 12px;color:#8a9bae;">Confidence</th><th style="text-align:left;padding:6px 12px;color:#8a9bae;">Source</th>';
            if (isAdmin) html += '<th style="padding:6px 0;"></th>';
            html += '</tr></thead><tbody>';
            segs.forEach(seg => {
                html += `<tr>
                    <td style="padding:6px 12px 6px 0;"><span class="tag tag-cyan">${typeLabel[seg.segment_type] || seg.segment_type}</span></td>
                    <td style="padding:6px 12px;">${formatTime(seg.start_seconds)}</td>
                    <td style="padding:6px 12px;">${formatTime(seg.end_seconds)}</td>
                    <td style="padding:6px 12px;">${Math.round(seg.confidence * 100)}%</td>
                    <td style="padding:6px 12px;">${sourceLabel[seg.source] || seg.source}${seg.verified ? ' &#9989;' : ''}</td>
                    ${isAdmin ? '<td style="padding:6px 0;"><button class="btn-danger btn-small" onclick="deleteSegment(\''+mediaId+'\',\''+seg.segment_type+'\')">Delete</button></td>' : ''}
                </tr>`;
            });
            html += '</tbody></table>';
            if (isAdmin) html += `<div style="margin-top:16px;"><button class="btn-secondary btn-small" onclick="showAddSegmentForm('${mediaId}')">+ Add Segment</button></div>`;
            tc.innerHTML = html;
        } else {
            let html = '<p style="color:#5a6a7f;margin-bottom:16px;">No skip segments detected for this item.</p>';
            if (isAdmin) html += `<button class="btn-secondary btn-small" onclick="showAddSegmentForm('${mediaId}')">+ Add Segment Manually</button>`;
            tc.innerHTML = html;
        }
    } else if (tab === 'file') {
        tc.innerHTML = '<p style="color:#5a6a7f;">Loading file info...</p>';
        const data = await api('GET', '/media/' + mediaId);
        if (data.success) {
            const m = data.data;
            tc.innerHTML = `<table style="width:100%;font-size:0.85rem;">
                <tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">File</td><td>${m.file_name}</td></tr>
                <tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Path</td><td style="word-break:break-all;">${m.file_path}</td></tr>
                ${m.width ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Resolution</td><td>'+m.width+'x'+m.height+'</td></tr>' : ''}
                ${m.codec ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Video Codec</td><td>'+m.codec+'</td></tr>' : ''}
                ${m.audio_codec ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Audio Codec</td><td>'+m.audio_codec+'</td></tr>' : ''}
                ${m.source_type ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Source</td><td>'+m.source_type+'</td></tr>' : ''}
                ${m.dynamic_range && m.dynamic_range !== 'SDR' ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Dynamic Range</td><td>'+m.dynamic_range+(m.hdr_format ? ' ('+m.hdr_format+')' : '')+'</td></tr>' : ''}
                ${m.bitrate ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Bitrate</td><td>'+(m.bitrate/1000).toFixed(0)+' kbps</td></tr>' : ''}
                ${m.file_hash ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">MD5 Hash</td><td style="font-family:monospace;font-size:0.78rem;">'+m.file_hash+'</td></tr>' : ''}
                ${m.phash ? '<tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">pHash</td><td style="font-family:monospace;font-size:0.78rem;">'+m.phash+'</td></tr>' : ''}
                <tr><td style="color:#5a6a7f;padding:4px 16px 4px 0;">Added</td><td>${new Date(m.added_at).toLocaleString()}</td></tr>
            </table>`;
        }
    }
}

// ──── Manual Segment Editor ────
function showAddSegmentForm(mediaId) {
    const tc = document.getElementById('detailTabContent');
    tc.innerHTML += `
        <div class="segment-editor" style="margin-top:20px;padding:16px;background:var(--surface-card);border-radius:var(--radius-lg);border:1px solid var(--border-subtle);">
            <h4 style="color:#e5e5e5;margin:0 0 12px;">Add Skip Segment</h4>
            <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;">
                <div>
                    <label style="color:#8a9bae;font-size:0.8rem;">Type</label>
                    <select id="segType" style="width:100%;padding:8px;background:var(--bg-secondary);border:1px solid var(--border-subtle-input);border-radius:var(--radius-md);color:#e5e5e5;">
                        <option value="intro">Intro</option>
                        <option value="credits">Credits</option>
                        <option value="recap">Recap</option>
                        <option value="preview">Preview</option>
                    </select>
                </div>
                <div>
                    <label style="color:#8a9bae;font-size:0.8rem;">Start (seconds)</label>
                    <input type="number" id="segStart" step="0.1" min="0" placeholder="0.0" style="width:100%;padding:8px;background:var(--bg-secondary);border:1px solid var(--border-subtle-input);border-radius:var(--radius-md);color:#e5e5e5;">
                </div>
                <div>
                    <label style="color:#8a9bae;font-size:0.8rem;">End (seconds)</label>
                    <input type="number" id="segEnd" step="0.1" min="0" placeholder="90.0" style="width:100%;padding:8px;background:var(--bg-secondary);border:1px solid var(--border-subtle-input);border-radius:var(--radius-md);color:#e5e5e5;">
                </div>
            </div>
            <div style="margin-top:12px;display:flex;gap:8px;justify-content:flex-end;">
                <button class="btn-secondary btn-small" onclick="showDetailTab(document.querySelector('.detail-tab[onclick*=segments]'),'segments','${mediaId}')">Cancel</button>
                <button class="btn-primary btn-small" onclick="saveSegment('${mediaId}')">Save</button>
            </div>
        </div>`;
}

async function saveSegment(mediaId) {
    const segType = document.getElementById('segType').value;
    const start = parseFloat(document.getElementById('segStart').value);
    const end = parseFloat(document.getElementById('segEnd').value);
    if (isNaN(start) || isNaN(end) || end <= start) {
        toast('Invalid time range', 'error');
        return;
    }
    const res = await api('POST', '/media/' + mediaId + '/segments', {
        segment_type: segType,
        start_seconds: start,
        end_seconds: end,
        verified: true
    });
    if (res.success) {
        toast('Segment saved!');
        const btn = document.querySelector('.detail-tab[onclick*="segments"]');
        if (btn) showDetailTab(btn, 'segments', mediaId);
    } else {
        toast(res.error || 'Failed to save segment', 'error');
    }
}

async function deleteSegment(mediaId, segType) {
    if (!confirm('Delete this ' + segType + ' segment?')) return;
    const res = await api('DELETE', '/media/' + mediaId + '/segments/' + segType);
    if (res.success) {
        toast('Segment deleted');
        const btn = document.querySelector('.detail-tab[onclick*="segments"]');
        if (btn) showDetailTab(btn, 'segments', mediaId);
    } else {
        toast(res.error || 'Failed to delete', 'error');
    }
}

// ──── Paginated Media Grid with Alpha Jump ────
let _gridState = { libraryId: null, offset: 0, total: 0, loading: false, done: false, observer: null, letterIndex: [], filters: {}, mediaType: '' };
let _filterOpts = {};
let _pickerVals = [];

const ALL_FILTER_DEFS = [
    { key: 'genre', label: 'Genre', apiKey: 'genres' },
    { key: 'country', label: 'Country', apiKey: 'countries' },
    { key: 'content_rating', label: 'Content Rating', apiKey: 'content_ratings' },
    { key: 'folder', label: 'Folder', apiKey: 'folders', displayFn: v => v.split('/').filter(Boolean).pop() || v },
    { key: 'edition', label: 'Edition', apiKey: 'editions' },
    { key: 'source', label: 'Source', apiKey: 'sources', displayFn: v => v.charAt(0).toUpperCase() + v.slice(1) },
    { key: 'dynamic_range', label: 'Dynamic Range', apiKey: 'dynamic_ranges' },
    { key: 'resolution', label: 'Resolution', apiKey: 'resolutions' },
    { key: 'codec', label: 'Codec', apiKey: 'codecs', displayMap: {'hevc':'HEVC (H.265)','h264':'H.264','av1':'AV1','mpeg4':'MPEG-4','mpeg2video':'MPEG-2','vp9':'VP9','vc1':'VC-1'} },
    { key: 'hdr_format', label: 'HDR Format', apiKey: 'hdr_formats' },
    { key: 'audio_codec', label: 'Audio Codec', apiKey: 'audio_codecs', displayMap: {'truehd':'TrueHD','eac3':'EAC3 (DD+)','ac3':'AC3 (DD)','aac':'AAC','dts':'DTS','flac':'FLAC','opus':'Opus','vorbis':'Vorbis','pcm_s16le':'PCM','pcm_s24le':'PCM 24-bit','mp3':'MP3'} },
    { key: 'bitrate_range', label: 'Bitrate', options: [{v:'low',l:'< 5 Mbps'},{v:'medium',l:'5-15 Mbps'},{v:'high',l:'15-30 Mbps'},{v:'ultra',l:'30+ Mbps'}] },
    { key: 'duration_range', label: 'Duration', options: [{v:'short',l:'< 30 min'},{v:'medium',l:'30-90 min'},{v:'long',l:'90-180 min'},{v:'vlong',l:'180+ min'}] },
    { key: 'watch_status', label: 'Watched', options: [{v:'watched',l:'Watched'},{v:'unwatched',l:'Unwatched'}] },
    { key: 'added_days', label: 'Recently Added', options: [{v:'1',l:'Today'},{v:'7',l:'Last 7 Days'},{v:'30',l:'Last 30 Days'},{v:'90',l:'Last 90 Days'}] }
];

const MUSIC_FILTER_KEYS = new Set(['genre', 'folder', 'audio_codec', 'duration_range', 'added_days']);

function getFilterDefs() {
    if (_gridState.mediaType === 'music') {
        return ALL_FILTER_DEFS.filter(d => MUSIC_FILTER_KEYS.has(d.key));
    }
    return ALL_FILTER_DEFS;
}

// Keep FILTER_DEFS as a getter for backward compatibility
const FILTER_DEFS = ALL_FILTER_DEFS;

function teardownGrid() {
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
    const grid = document.getElementById('libGrid');
    if (!grid) return;

    _appendingNewItems = true;
    try {
        // Fetch the most recently added items, sorted by added_at desc
        const m = await api('GET', '/libraries/' + libraryId + '/media?sort=added_at&order=desc&limit=' + Math.min(newCount, 50));
        const items = (m.success && m.data && m.data.items) ? m.data.items : [];
        if (items.length === 0) return;

        // Filter out items already in the grid
        const newItems = items.filter(item => !grid.querySelector('[data-media-id="' + item.id + '"]'));
        if (newItems.length === 0) return;

        // Insert before the sentinel (or append at end)
        const sentinel = document.getElementById('gridSentinel');
        const html = newItems.map(renderMediaCard).join('');
        if (sentinel) {
            sentinel.insertAdjacentHTML('beforebegin', html);
        } else {
            grid.insertAdjacentHTML('beforeend', html);
        }

        // Update the item count display
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
    const letter = el.dataset.letter;
    const grid = document.getElementById('libGrid');
    if (!grid || !_gridState.libraryId) return;

    // Disconnect both observers
    if (_gridState.observer) _gridState.observer.disconnect();
    if (_gridState.topObserver) { _gridState.topObserver.disconnect(); _gridState.topObserver = null; }

    // Clear the grid, reset state to the target offset
    grid.innerHTML = '';
    _gridState.offset = targetOffset;
    _gridState.startOffset = targetOffset;
    _gridState.done = false;
    _gridState.loading = false;
    await loadMoreMedia();

    // Scroll grid to top so the first loaded card is visible
    const container = document.getElementById('mainContent');
    if (container) container.scrollTop = 0;

    // Re-attach infinite scroll for continued browsing forward
    setupScrollObserver();

    // Add top sentinel for backward scrolling if not at the beginning
    if (_gridState.startOffset > 0) {
        grid.insertAdjacentHTML('afterbegin', '<div id="gridTopSentinel" class="load-more-sentinel"></div>');
        setupTopScrollObserver();
    }

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
           <div id="albumsArea" style="display:none;"></div>`
        : `<div id="collectionsArea" style="display:none;"></div>
           <div id="seriesArea" style="display:none;"></div>`;

    mc.innerHTML = `<div class="section-header"><h2 class="section-title">${label}</h2><span class="tag tag-cyan" style="margin-left:12px;">${MEDIA_LABELS[type]||type}</span><span class="tag" id="libItemCount" style="margin-left:8px;">${totalCount.toLocaleString()} items</span></div>
        ${buildFilterToolbar(filterOpts)}
        <div class="media-grid-wrapper" id="mediaGridWrapper">
            <div class="media-grid" id="libGrid"><div id="gridSentinel" class="load-more-sentinel"><div class="spinner"></div></div></div>
            ${buildAlphaJump(letterIndex)}
        </div>
        ${extraAreas}`;

    renderFilterChips();
    loadFilterPresetsIntoDropdown();
    setupScrollObserver();
    loadMoreMedia();

    // Update active letter on scroll
    const container = document.getElementById('mainContent');
    container.addEventListener('scroll', () => { requestAnimationFrame(updateAlphaCount); });
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
        ? `<button class="ft-btn active" id="ftGridBtn" onclick="showLibraryGrid()" title="All Tracks">&#9638; Grid</button>
           <button class="ft-btn" id="ftArtistBtn" onclick="showMusicArtists()" title="View by Artist">&#127908; Artists</button>
           <button class="ft-btn" id="ftAlbumBtn" onclick="showMusicAlbums()" title="View by Album">&#128191; Albums</button>`
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

    // Reset grid state but keep libraryId and filters
    const filters = _gridState.filters;
    if (_gridState.observer) { _gridState.observer.disconnect(); _gridState.observer = null; }
    _gridState.offset = 0;
    _gridState.total = 0;
    _gridState.loading = false;
    _gridState.done = false;
    _gridState.filters = filters;

    // Refresh letter index with filters
    const qs = buildFilterQS();
    const idxData = await api('GET', '/libraries/' + libId + '/media/index' + (qs ? '?' + qs.substring(1) : ''));
    const letterIndex = (idxData.success && idxData.data) ? idxData.data : [];
    _gridState.letterIndex = letterIndex;

    const totalCount = letterIndex.reduce((s, e) => s + e.count, 0);
    const countEl = document.getElementById('libItemCount');
    if (countEl) countEl.textContent = totalCount.toLocaleString() + ' items';

    // Show grid, hide all secondary areas
    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'flex';
    ['collectionsArea','seriesArea','artistsArea','albumsArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    });

    // Rebuild grid
    const grid = document.getElementById('libGrid');
    if (grid) {
        grid.innerHTML = '<div id="gridSentinel" class="load-more-sentinel"><div class="spinner"></div></div>';
    }

    // Rebuild alpha jump
    const existingJump = document.querySelector('.alpha-jump');
    if (existingJump) existingJump.outerHTML = buildAlphaJump(letterIndex);

    // Update toggle buttons
    ['ftGridBtn','ftCollBtn','ftSeriesBtn','ftArtistBtn','ftAlbumBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftGridBtn');
    });

    setupScrollObserver();
    loadMoreMedia();
}

function showLibraryGrid() {
    const wrapper = document.getElementById('mediaGridWrapper');
    if (wrapper) wrapper.style.display = 'flex';
    // Hide all secondary areas
    ['collectionsArea','seriesArea','artistsArea','albumsArea'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.style.display = 'none';
    });
    // Update toggle buttons
    ['ftGridBtn','ftCollBtn','ftSeriesBtn','ftArtistBtn','ftAlbumBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftGridBtn');
    });
}

// ──── Music: View by Artist / Album ────
function renderArtistCard(artist) {
    const meta = [];
    if (artist.album_count) meta.push(artist.album_count + ' album' + (artist.album_count !== 1 ? 's' : ''));
    if (artist.track_count) meta.push(artist.track_count + ' track' + (artist.track_count !== 1 ? 's' : ''));
    return `<div class="media-card" tabindex="0" data-artist-id="${artist.id}" onclick="showArtistDetail('${artist.id}')">
        <div class="media-poster music-poster-round">
            ${artist.poster_path ? '<img src="'+posterSrc(artist.poster_path, artist.updated_at)+'" alt="" loading="lazy">' : '<div class="music-icon-placeholder">&#127908;</div>'}
        </div>
        <div class="media-info"><div class="media-title">${escapeHtml(artist.name)}</div><div class="media-meta">${meta.join(' \u00b7 ')}</div></div>
    </div>`;
}

function renderAlbumCard(album) {
    const meta = [];
    if (album.artist_name) meta.push(album.artist_name);
    if (album.year) meta.push(album.year);
    if (album.track_count) meta.push(album.track_count + ' track' + (album.track_count !== 1 ? 's' : ''));
    return `<div class="media-card" tabindex="0" data-album-id="${album.id}" onclick="showAlbumDetail('${album.id}')">
        <div class="media-poster">
            ${album.poster_path ? '<img src="'+posterSrc(album.poster_path, album.updated_at)+'" alt="" loading="lazy">' : '<div class="music-icon-placeholder">&#128191;</div>'}
        </div>
        <div class="media-info"><div class="media-title">${escapeHtml(album.title)}</div><div class="media-meta">${meta.join(' \u00b7 ')}</div></div>
    </div>`;
}

async function showMusicArtists() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    const artistsArea = document.getElementById('artistsArea');
    const albumsArea = document.getElementById('albumsArea');
    if (wrapper) wrapper.style.display = 'none';
    if (artistsArea) artistsArea.style.display = 'block';
    if (albumsArea) albumsArea.style.display = 'none';

    ['ftGridBtn','ftArtistBtn','ftAlbumBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftArtistBtn');
    });

    artistsArea.innerHTML = '<div class="spinner"></div> Loading artists...';
    const data = await api('GET', '/libraries/' + libId + '/artists');
    const artists = (data.success && data.data) ? data.data : [];

    if (artists.length > 0) {
        artistsArea.innerHTML = `<div class="media-grid-wrapper"><div class="media-grid music-artist-grid">${artists.map(renderArtistCard).join('')}</div></div>`;
    } else {
        artistsArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#127908;</div><div class="empty-state-title">No artists found</div><p>Scan the library to detect artists</p></div>';
    }
}

async function showMusicAlbums() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    const artistsArea = document.getElementById('artistsArea');
    const albumsArea = document.getElementById('albumsArea');
    if (wrapper) wrapper.style.display = 'none';
    if (artistsArea) artistsArea.style.display = 'none';
    if (albumsArea) albumsArea.style.display = 'block';

    ['ftGridBtn','ftArtistBtn','ftAlbumBtn'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.classList.toggle('active', id === 'ftAlbumBtn');
    });

    albumsArea.innerHTML = '<div class="spinner"></div> Loading albums...';
    const data = await api('GET', '/libraries/' + libId + '/albums');
    const albums = (data.success && data.data) ? data.data : [];

    if (albums.length > 0) {
        albumsArea.innerHTML = `<div class="media-grid-wrapper"><div class="media-grid">${albums.map(renderAlbumCard).join('')}</div></div>`;
    } else {
        albumsArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#128191;</div><div class="empty-state-title">No albums found</div><p>Scan the library to detect albums</p></div>';
    }
}

async function showArtistDetail(artistId) {
    const libId = _gridState.libraryId;
    if (!libId) return;
    _detailReturnNav = { view: 'library', extra: libId };

    const data = await api('GET', '/libraries/' + libId + '/albums');
    const allAlbums = (data.success && data.data) ? data.data : [];
    const albums = allAlbums.filter(a => a.artist_id === artistId);

    const artistsData = await api('GET', '/libraries/' + libId + '/artists');
    const allArtists = (artistsData.success && artistsData.data) ? artistsData.data : [];
    const artist = allArtists.find(a => a.id === artistId);
    const artistName = artist ? artist.name : 'Unknown Artist';

    const mc = document.getElementById('mainContent');
    let html = `<div class="section-header">
        <button class="btn-secondary" onclick="navigate('library','${libId}')" style="margin-right:12px;">&#8592; Back</button>
        <h2 class="section-title">${escapeHtml(artistName)}</h2>
        <span class="tag" style="margin-left:8px;">${albums.length} album${albums.length !== 1 ? 's' : ''}</span>
    </div>`;

    if (albums.length > 0) {
        html += `<div class="media-grid">${albums.map(renderAlbumCard).join('')}</div>`;
    } else {
        html += '<div class="empty-state"><div class="empty-state-icon">&#128191;</div><div class="empty-state-title">No albums</div></div>';
    }

    mc.innerHTML = html;
}

async function showAlbumDetail(albumId) {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const albumData = await api('GET', '/libraries/' + libId + '/albums');
    const allAlbums = (albumData.success && albumData.data) ? albumData.data : [];
    const album = allAlbums.find(a => a.id === albumId);
    const albumTitle = album ? album.title : 'Unknown Album';
    const artistName = album ? album.artist_name : '';

    const mediaData = await api('GET', '/libraries/' + libId + '/media?limit=500&sort=track_number&order=asc');
    const allItems = (mediaData.success && mediaData.data && mediaData.data.items) ? mediaData.data.items : [];
    const tracks = allItems.filter(m => m.album_id === albumId);

    const mc = document.getElementById('mainContent');
    let html = `<div class="section-header">
        <button class="btn-secondary" onclick="navigate('library','${libId}')" style="margin-right:12px;">&#8592; Back</button>
        <h2 class="section-title">${escapeHtml(albumTitle)}</h2>
        ${artistName ? '<span class="tag tag-cyan" style="margin-left:8px;">'+escapeHtml(artistName)+'</span>' : ''}
        <span class="tag" style="margin-left:8px;">${tracks.length} track${tracks.length !== 1 ? 's' : ''}</span>
    </div>`;

    if (tracks.length > 0) {
        html += `<div class="media-grid">${tracks.map(renderMediaCard).join('')}</div>`;
    } else {
        html += '<div class="empty-state"><div class="empty-state-icon">&#127925;</div><div class="empty-state-title">No tracks</div></div>';
    }

    mc.innerHTML = html;
}

async function showLibraryCollections() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    const collArea = document.getElementById('collectionsArea');
    const serArea = document.getElementById('seriesArea');
    if (wrapper) wrapper.style.display = 'none';
    if (collArea) collArea.style.display = 'block';
    if (serArea) serArea.style.display = 'none';

    const gridBtn = document.getElementById('ftGridBtn');
    const collBtn = document.getElementById('ftCollBtn');
    const serBtn = document.getElementById('ftSeriesBtn');
    if (gridBtn) gridBtn.classList.remove('active');
    if (collBtn) collBtn.classList.add('active');
    if (serBtn) serBtn.classList.remove('active');

    collArea.innerHTML = '<div class="spinner"></div> Loading collections...';
    const data = await api('GET', '/collections?library_id=' + libId);
    const collections = (data.success && data.data) ? data.data : [];

    // Header with action buttons
    let html = `<div class="section-header" style="margin-bottom:16px;">
        <h2 class="section-title" style="font-size:var(--text-lg);margin:0;">Collections</h2>
        <div style="display:flex;gap:8px;">
            <button class="btn-secondary" onclick="createCollectionTemplates('${libId}')" title="Create smart collection presets">+ Templates</button>
            <button class="btn-primary" onclick="showCreateCollection(null, '${libId}')">+ New</button>
        </div>
    </div>`;

    if (collections.length === 0) {
        collArea.innerHTML = html + '<div class="empty-state"><div class="empty-state-icon">&#128218;</div><div class="empty-state-title">No collections</div><p>Create a collection or use templates to get started</p></div>';
        return;
    }

    // Show only top-level collections (no parent)
    const topLevel = collections.filter(c => !c.parent_collection_id);
    const toShow = topLevel.length > 0 ? topLevel : collections;
    collArea.innerHTML = html + '<div class="collection-grid">' + toShow.map(c => {
        const poster = c.poster_path
            ? `<img src="${posterSrc(c.poster_path, c.updated_at)}" alt="">`
            : '&#128218;';
        const childInfo = c.child_count > 0 ? `<div class="cc-children">${c.child_count} sub-collection${c.child_count !== 1 ? 's' : ''}</div>` : '';
        return `<div class="collection-card" onclick="navigate('collection','${c.id}')">
            <div class="cc-poster">${poster}</div>
            <div class="cc-info">
                <div class="cc-name">${c.name}</div>
                <div class="cc-meta">${c.item_count || 0} item${(c.item_count||0) !== 1 ? 's' : ''}</div>
                ${childInfo}
            </div>
        </div>`;
    }).join('') + '</div>';
}

function renderShowCard(show) {
    const year = show.year || '';
    const desc = show.description ? show.description.substring(0, 100) + '...' : '';
    return `<div class="media-card" onclick="loadShowView('${show.id}')">
        <div class="media-poster" style="position:relative;">
            ${show.poster_path ? '<img src="'+posterSrc(show.poster_path, show.updated_at)+'" alt="">' : '&#128250;'}
            ${renderOverlayBadges(show)}
        </div>
        <div class="media-info"><div class="media-title">${show.title}</div><div class="media-meta">${year}</div></div>
    </div>`;
}

async function loadShowView(showId) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div> Loading...';

    // Fetch show data and missing episodes in parallel (show-specific endpoint)
    const [data, missingData] = await Promise.all([
        api('GET', '/tv/shows/' + showId),
        api('GET', '/tv/shows/' + showId + '/missing-episodes')
    ]);
    if (!data.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Show not found</div></div>'; return; }
    const show = data.data.show;
    const seasons = data.data.seasons || [];
    const totalEps = seasons.reduce((sum, s) => sum + (s.episode_count || 0), 0);

    // Parse missing episode data from show-specific endpoint
    let missingMap = {}; // seasonId -> {missing_numbers, have_count, expected_count}
    let totalMissing = 0;
    if (missingData.success && missingData.data) {
        totalMissing = missingData.data.total_missing || 0;
        (missingData.data.seasons || []).forEach(sm => { missingMap[sm.season_id] = sm; });
    }

    const missingBadge = totalMissing > 0 ? ` <span class="tag tag-warning" style="margin-left:8px;">${totalMissing} missing episode${totalMissing !== 1 ? 's' : ''}</span>` : '';

    const heroHTML = `
        <div class="detail-hero">
            <div class="detail-poster">${show.poster_path ? '<img src="'+posterSrc(show.poster_path, show.updated_at)+'">' : '&#128250;'}</div>
            <div class="detail-info">
                <h1>${show.title}</h1>
                <div class="meta-row">${show.year ? show.year + ' \u00b7 ' : ''}${seasons.length} Season${seasons.length !== 1 ? 's' : ''} \u00b7 ${totalEps} Episode${totalEps !== 1 ? 's' : ''}${missingBadge}</div>
                ${show.description ? '<p class="description">'+show.description+'</p>' : ''}
            </div>
        </div>`;

    if (seasons.length === 1) {
        const sm = missingMap[seasons[0].id];
        const singleMissing = sm ? `<div class="missing-episodes-banner">${sm.have_count}/${sm.expected_count} episodes \u00b7 Missing: ${sm.missing_numbers.join(', ')}</div>` : '';
        mc.innerHTML = heroHTML + `
            <h3 style="color:#e5e5e5;margin:24px 0 16px;">${totalEps} Episode${totalEps !== 1 ? 's' : ''}</h3>
            ${singleMissing}
            <div class="media-grid" id="season-${seasons[0].id}"><div class="spinner"></div></div>
            <button class="btn-secondary" onclick="history.back(); return false;" style="margin-top:20px;">&#8592; Back</button>`;
        loadSeasonEpisodes(seasons[0].id);
    } else {
        mc.innerHTML = heroHTML + `
            <h3 style="color:#e5e5e5;margin:24px 0 16px;">Seasons</h3>
            <div class="media-grid" id="seasonsGrid">
                ${seasons.map(s => {
                    const sm = missingMap[s.id];
                    const missingLabel = sm ? `<div class="media-meta missing-meta">${sm.have_count}/${sm.expected_count} eps \u00b7 ${sm.missing_numbers.length} missing</div>` : '';
                    return `<div class="media-card" onclick="loadSeasonView('${showId}','${s.id}', ${s.season_number})">
                    <div class="media-poster">
                        ${s.poster_path ? '<img src="'+posterSrc(s.poster_path, s.updated_at)+'" alt="">' : (show.poster_path ? '<img src="'+posterSrc(show.poster_path, show.updated_at)+'" alt="">' : '&#128250;')}
                    </div>
                    <div class="media-info">
                        <div class="media-title">${s.title || 'Season ' + s.season_number}</div>
                        <div class="media-meta">${s.episode_count} episode${s.episode_count !== 1 ? 's' : ''}</div>
                        ${missingLabel}
                    </div>
                </div>`;
                }).join('')}
            </div>
            <button class="btn-secondary" onclick="history.back(); return false;" style="margin-top:20px;">&#8592; Back</button>`;
    }
}

async function loadSeasonView(showId, seasonId, seasonNum) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div> Loading...';
    // Get show info and missing episode data in parallel
    const [showData, missingData] = await Promise.all([
        api('GET', '/tv/shows/' + showId),
        api('GET', '/tv/seasons/' + seasonId + '/missing')
    ]);
    if (!showData.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Show not found</div></div>'; return; }
    const show = showData.data.show;
    const seasons = showData.data.seasons || [];
    const season = seasons.find(s => s.id === seasonId);
    const epCount = season ? season.episode_count : 0;

    let missingBanner = '';
    if (missingData.success && missingData.data && missingData.data.missing_numbers && missingData.data.missing_numbers.length > 0) {
        const md = missingData.data;
        missingBanner = `<div class="missing-episodes-banner">${md.have_count}/${md.expected_count} episodes \u00b7 Missing: ${md.missing_numbers.join(', ')}</div>`;
    }

    mc.innerHTML = `
        <div class="detail-hero">
            <div class="detail-poster">${season && season.poster_path ? '<img src="'+posterSrc(season.poster_path, season.updated_at)+'">' : (show.poster_path ? '<img src="'+posterSrc(show.poster_path, show.updated_at)+'">' : '&#128250;')}</div>
            <div class="detail-info">
                <h1>${show.title}</h1>
                <div class="meta-row">Season ${seasonNum} \u00b7 ${epCount} Episode${epCount !== 1 ? 's' : ''}</div>
                ${season && season.description ? '<p class="description">'+season.description+'</p>' : (show.description ? '<p class="description">'+show.description+'</p>' : '')}
            </div>
        </div>
        <h3 style="color:#e5e5e5;margin:24px 0 16px;">${epCount} Episode${epCount !== 1 ? 's' : ''}</h3>
        ${missingBanner}
        <div class="media-grid" id="season-${seasonId}"><div class="spinner"></div></div>
        <button class="btn-secondary" onclick="loadShowView('${showId}')" style="margin-top:20px;">&#8592; Back to ${show.title}</button>`;
    loadSeasonEpisodes(seasonId);
}

async function loadSeasonEpisodes(seasonId) {
    const container = document.getElementById('season-' + seasonId);
    if (!container) return;
    const data = await api('GET', '/tv/seasons/' + seasonId + '/episodes');
    const episodes = (data.success && data.data) ? data.data : [];
    container.innerHTML = episodes.length > 0
        ? episodes.map(ep => {
            const epNum = ep.episode_number ? 'Episode ' + ep.episode_number : '';
            const dur = ep.duration_seconds ? Math.floor(ep.duration_seconds/60)+'min' : '';
            const res = ep.resolution || '';
            const meta = [epNum, dur, res].filter(Boolean).join(' \u00b7 ');
            return `<div class="media-card" onclick="loadMediaDetail('${ep.id}')">
                <div class="media-poster" style="position:relative;">
                    ${ep.poster_path ? '<img src="'+posterSrc(ep.poster_path, ep.updated_at)+'" alt="">' : '&#128250;'}
                    ${renderOverlayBadges(ep)}
                    <div class="play-overlay"><div class="play-button">&#9654;</div></div>
                </div>
                <div class="media-info"><div class="media-title">${ep.title}</div><div class="media-meta">${meta}</div></div>
            </div>`;
        }).join('')
        : '<p style="color:#5a6a7f;">No episodes found</p>';
}

async function loadSearchView(query) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Search: "${query}"</h2></div><div class="media-grid" id="searchGrid"><div class="spinner"></div> Searching...</div>`;
    const data = await api('GET', '/media/search?q=' + encodeURIComponent(query));
    const grid = document.getElementById('searchGrid');
    grid.innerHTML = (data.success && data.data && data.data.length > 0) ? data.data.map(renderMediaCard).join('') : '<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-title">No results</div></div>';
    enableGridKeyNav(grid);
}

// ──── Libraries ────
async function loadLibrariesView() {
    const mc = document.getElementById('mainContent');
    const isAdmin = currentUser && currentUser.role === 'admin';
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Libraries</h2>${isAdmin?'<button class="btn-primary" onclick="showCreateLibrary()">+ Add Library</button>':''}</div><div id="libList"><div class="spinner"></div> Loading...</div>`;
    const data = await api('GET', '/libraries');
    const div = document.getElementById('libList');
    if (data.success && data.data && data.data.length > 0) {
        div.innerHTML = data.data.map(lib => {
            const accessLabel = {everyone:'Everyone',select_users:'Select People',admin_only:'Admin Only'}[lib.access_level]||'Everyone';
            const accessColor = {everyone:'tag-green',select_users:'tag-purple',admin_only:'tag-red'}[lib.access_level]||'tag-green';
            const folderPaths = (lib.folders && lib.folders.length > 0) ? lib.folders.map(f => f.folder_path).join(', ') : lib.path;
            const folderCount = (lib.folders && lib.folders.length > 1) ? `<span class="tag tag-orange" style="margin-left:6px;">${lib.folders.length} folders</span>` : '';
            let settingsTags = '';
            if (!lib.include_in_homepage) settingsTags += '<span class="tag tag-red" style="margin-left:4px;">Hidden from Home</span>';
            if (!lib.include_in_search) settingsTags += '<span class="tag tag-red" style="margin-left:4px;">Hidden from Search</span>';
            if (!lib.retrieve_metadata) settingsTags += '<span class="tag tag-orange" style="margin-left:4px;">No Metadata</span>';
            if (lib.create_previews === false) settingsTags += '<span class="tag tag-orange" style="margin-left:4px;">No Previews</span>';
            if (lib.create_thumbnails === false) settingsTags += '<span class="tag tag-orange" style="margin-left:4px;">No Thumbnails</span>';
            if (lib.audio_normalization) settingsTags += '<span class="tag tag-cyan" style="margin-left:4px;">Audio Normalization</span>';
            if (lib.media_type === 'adult_movies' && lib.adult_content_type) settingsTags += `<span class="tag tag-purple" style="margin-left:4px;">${lib.adult_content_type === 'clips' ? 'Clips' : 'Movies'}</span>`;
            return `<div class="library-card" id="lib-card-${lib.id}"><div style="flex:1;"><h3>${lib.name}</h3><p style="color:#8a9bae;font-size:0.85rem;"><span class="tag tag-cyan">${MEDIA_LABELS[lib.media_type]||lib.media_type}</span>${lib.season_grouping?'<span class="tag tag-purple" style="margin-left:6px;">Season Grouping</span>':''}<span class="tag ${accessColor}" style="margin-left:6px;">${accessLabel}</span>${folderCount}<span style="margin-left:8px;">${folderPaths}</span></p><div class="lib-settings-tags">${settingsTags}</div><p style="color:#5a6a7f;font-size:0.78rem;margin-top:6px;">${lib.last_scan_at?'Last scan: '+new Date(lib.last_scan_at).toLocaleString():'Never scanned'}</p><div class="scan-progress" id="scan-progress-${lib.id}"><div class="scan-progress-bar"><div class="scan-progress-fill" id="scan-fill-${lib.id}"></div></div><div class="scan-progress-text"><span class="filename" id="scan-file-${lib.id}"></span><span id="scan-count-${lib.id}"></span></div></div></div><div class="library-actions">${isAdmin?`<button class="btn-secondary" id="scan-btn-${lib.id}" onclick="scanLibrary('${lib.id}',this)">&#128269; Scan</button><button class="btn-danger btn-small" onclick="deleteLibrary('${lib.id}')">Delete</button>`:''}</div></div>`;
        }).join('');
    } else div.innerHTML = `<div class="empty-state"><div class="empty-state-icon">&#128218;</div><div class="empty-state-title">No libraries configured</div><p>Create your first library to start organizing media</p></div>`;
}

async function scanLibrary(id, btn) {
    btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>Scanning...';
    const prog = document.getElementById('scan-progress-' + id);
    if (prog) prog.classList.add('active');
    const countEl = document.getElementById('scan-count-' + id);
    if (countEl) countEl.textContent = 'Counting files...';
    try {
        const data = await api('POST', '/libraries/'+id+'/scan');
        if (data.success) {
            if (data.data.job_id) { /* progress handled by WebSocket events */ }
            else {
                // Synchronous scan fallback (no job queue)
                const r = data.data;
                toast(`Scan: ${r.files_added} added, ${r.files_found} total`);
                loadLibrariesView(); loadSidebarCounts();
                btn.disabled = false; btn.innerHTML = '&#128269; Scan';
                if (prog) prog.classList.remove('active');
            }
        } else {
            toast('Scan failed: '+(data.error||'Unknown'), 'error');
            btn.disabled = false; btn.innerHTML = '&#128269; Scan';
            if (prog) prog.classList.remove('active');
        }
    } catch(e) {
        toast('Scan error: '+e.message, 'error');
        btn.disabled = false; btn.innerHTML = '&#128269; Scan';
        if (prog) prog.classList.remove('active');
    }
}

async function deleteLibrary(id) { if (!confirm('Delete this library and all its media?')) return; const d=await api('DELETE','/libraries/'+id); if(d.success){toast('Library deleted');loadLibrariesView();}else toast('Failed: '+d.error,'error'); }

function showCreateLibrary() {
    const mc = document.getElementById('mainContent');
    const types = Object.entries(MEDIA_LABELS).map(([k,v])=>`<option value="${k}">${v}</option>`).join('');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Create Library</h2></div>
    <div style="max-width:560px;">
        <div class="form-group"><label>Name</label><input type="text" id="libName" placeholder="My Movies"></div>
        <div class="form-group"><label>Media Type</label><select id="libType" onchange="onLibTypeChange()">${types}</select></div>

        <div class="form-group">
            <label>Folders</label>
            <p style="color:#8a9bae;font-size:0.78rem;margin-bottom:8px;">At least one folder is required. Add more to scan multiple locations.</p>
            <div class="folder-list" id="folderList">
                <div class="folder-row" data-idx="0">
                    <input type="text" class="lib-folder-input" placeholder="/media/movies" style="flex:1;">
                    <button class="btn-secondary" onclick="openFolderBrowser(0)" style="white-space:nowrap;padding:8px 12px;font-size:0.8rem;">&#128193; Browse</button>
                </div>
            </div>
            <button class="folder-add-btn" onclick="addFolderRow()">+ Add Folder</button>
        </div>
        <div id="pathBrowser" style="display:none;margin-bottom:18px;background:rgba(0,0,0,0.3);border:1px solid rgba(0,217,255,0.15);border-radius:12px;padding:14px;max-height:350px;overflow-y:auto;"></div>

        <div id="seasonGroupingOpt" style="display:none;">
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Group by Season</div>
                    <div class="option-row-desc">Parse SxxExx from filenames to group episodes by season</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="seasonGrouping" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="seasonGrouping" value="no"><span>No</span></label>
                </div>
            </div>
        </div>

        <div id="adultContentOpt" style="display:none;">
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Content Type</div>
                    <div class="option-row-desc">Clips &amp; Scenes will not retrieve metadata. Movies will scrape from TMDB.</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="adultContentType" value="movies" checked><span>Movies</span></label>
                    <label><input type="radio" name="adultContentType" value="clips"><span>Clips</span></label>
                </div>
            </div>
        </div>

        <div class="form-group">
            <label>Library Permissions</label>
            <select id="libAccess" onchange="onAccessChange()">
                <option value="everyone">Everyone</option>
                <option value="select_users">Select People</option>
                <option value="admin_only">Admin Only</option>
            </select>
        </div>
        <div id="userSelectPanel" style="display:none;margin-bottom:18px;background:rgba(0,0,0,0.3);border:1px solid rgba(0,217,255,0.15);border-radius:12px;padding:14px;">
            <p style="color:#8a9bae;font-size:0.8rem;margin-bottom:10px;">Select users who can access this library:</p>
            <div id="userCheckboxList"><div class="spinner"></div></div>
        </div>

        <div style="margin-bottom:18px;">
            <label style="display:block;margin-bottom:10px;font-weight:600;color:#e5e5e5;">Library Options</label>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Include in Homepage</div>
                    <div class="option-row-desc">Show this library's media on the home screen</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="includeHomepage" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="includeHomepage" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Include in Search</div>
                    <div class="option-row-desc">Allow this library's items to appear in search results</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="includeSearch" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="includeSearch" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row" id="metadataOpt">
                <div class="option-row-info">
                    <div class="option-row-label">Retrieve Metadata</div>
                    <div class="option-row-desc">Auto-populate from TMDB/MusicBrainz/OpenLibrary on scan</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="retrieveMetadata" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="retrieveMetadata" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">NFO Import</div>
                    <div class="option-row-desc">Read Kodi/Jellyfin .nfo sidecar files for metadata &amp; provider IDs</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="nfoImport" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="nfoImport" value="no" checked><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">NFO Export</div>
                    <div class="option-row-desc">Write .nfo sidecar files after metadata is populated</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="nfoExport" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="nfoExport" value="no" checked><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Prefer Local Artwork</div>
                    <div class="option-row-desc">Use poster/backdrop/logo files found next to media before fetching remote</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="preferLocalArtwork" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="preferLocalArtwork" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Generate Preview Clips</div>
                    <div class="option-row-desc">Create short animated preview clips shown on hover</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="createPreviews" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="createPreviews" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Generate Timeline Thumbnails</div>
                    <div class="option-row-desc">Create sprite sheets used for the player timeline scrubber</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="createThumbnails" value="yes" checked><span>Yes</span></label>
                    <label><input type="radio" name="createThumbnails" value="no"><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Audio Normalization</div>
                    <div class="option-row-desc">Analyze loudness (EBU R128) and normalize volume during playback</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="audioNormalization" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="audioNormalization" value="no" checked><span>No</span></label>
                </div>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Scheduled Scan</div>
                    <div class="option-row-desc">Automatically scan this library on a schedule</div>
                </div>
                <select id="editScanInterval" class="form-select" style="width:auto;">
                    <option value="disabled" selected>Disabled</option>
                    <option value="1h">Every Hour</option>
                    <option value="6h">Every 6 Hours</option>
                    <option value="12h">Every 12 Hours</option>
                    <option value="24h">Daily</option>
                    <option value="weekly">Weekly</option>
                </select>
            </div>
            <div class="option-row">
                <div class="option-row-info">
                    <div class="option-row-label">Filesystem Watcher</div>
                    <div class="option-row-desc">Watch folders for new/deleted files in real-time</div>
                </div>
                <div class="toggle-btns">
                    <label><input type="radio" name="watchEnabled" value="yes"><span>Yes</span></label>
                    <label><input type="radio" name="watchEnabled" value="no" checked><span>No</span></label>
                </div>
            </div>
        </div>

        <button class="btn-primary" onclick="createLibrary()">Create Library</button>
        <button class="btn-secondary" style="margin-left:12px;" onclick="loadLibrariesView()">Cancel</button>
    </div>`;
    onLibTypeChange();
}

let activeFolderIdx = null;

function addFolderRow() {
    const list = document.getElementById('folderList');
    const idx = list.children.length;
    const row = document.createElement('div');
    row.className = 'folder-row';
    row.dataset.idx = idx;
    row.innerHTML = `<input type="text" class="lib-folder-input" placeholder="/media/folder" style="flex:1;">
        <button class="btn-secondary" onclick="openFolderBrowser(${idx})" style="white-space:nowrap;padding:8px 12px;font-size:0.8rem;">&#128193; Browse</button>
        <button class="folder-remove" onclick="removeFolderRow(this)" title="Remove folder">&#10005;</button>`;
    list.appendChild(row);
}

function removeFolderRow(btn) {
    const row = btn.closest('.folder-row');
    const list = document.getElementById('folderList');
    if (list.children.length <= 1) { toast('At least one folder is required', 'error'); return; }
    row.remove();
}

function openFolderBrowser(idx) {
    activeFolderIdx = idx;
    const inputs = document.querySelectorAll('.lib-folder-input');
    const currentPath = inputs[idx]?.value || '/media';
    openPathBrowser(currentPath);
}

function onLibTypeChange() {
    const type = document.getElementById('libType').value;
    const seasonOpt = document.getElementById('seasonGroupingOpt');
    const adultOpt = document.getElementById('adultContentOpt');
    if (seasonOpt) seasonOpt.style.display = (type === 'tv_shows') ? '' : 'none';
    if (adultOpt) adultOpt.style.display = (type === 'adult_movies') ? '' : 'none';
}

async function onAccessChange() {
    const val = document.getElementById('libAccess').value;
    const panel = document.getElementById('userSelectPanel');
    if (val === 'select_users') {
        panel.style.display = '';
        const data = await api('GET', '/users');
        const list = document.getElementById('userCheckboxList');
        if (data.success && data.data && data.data.length > 0) {
            list.innerHTML = data.data
                .filter(u => u.role !== 'admin')
                .map(u => `<label style="display:flex;align-items:center;gap:8px;padding:6px 0;cursor:pointer;color:#e5e5e5;">
                    <input type="checkbox" class="user-perm-cb" value="${u.id}">
                    <span>${u.username}</span>
                    <span style="color:#5a6a7f;font-size:0.75rem;margin-left:auto;">${u.role}</span>
                </label>`).join('');
            if (list.innerHTML === '') list.innerHTML = '<p style="color:#5a6a7f;font-size:0.85rem;">No non-admin users found</p>';
        } else {
            list.innerHTML = '<p style="color:#5a6a7f;font-size:0.85rem;">No users found</p>';
        }
    } else {
        panel.style.display = 'none';
    }
}

async function openPathBrowser(path) {
    const browser = document.getElementById('pathBrowser');
    browser.style.display = 'block';
    await loadPathEntries(path || '/media');
}

async function loadPathEntries(path) {
    const browser = document.getElementById('pathBrowser');
    browser.innerHTML = '<div class="spinner" style="margin:10px auto;"></div>';
    const data = await api('GET', '/browse?path=' + encodeURIComponent(path));
    if (!data.success) { browser.innerHTML = '<p style="color:#ff5555;">Failed to browse</p>'; return; }
    const d = data.data;
    let html = `<div style="display:flex;align-items:center;gap:8px;margin-bottom:12px;padding-bottom:10px;border-bottom:1px solid rgba(0,217,255,0.1);">
        <span style="color:#00D9FF;font-size:0.8rem;font-weight:600;">&#128194; ${d.path}</span>
        <button class="btn-secondary" style="margin-left:auto;padding:4px 12px;font-size:0.75rem;" onclick="selectBrowsePath('${d.path}')">&#10003; Select This</button>
    </div>`;
    if (d.parent) {
        html += `<div onclick="loadPathEntries('${d.parent}')" style="display:flex;align-items:center;gap:8px;padding:8px 10px;cursor:pointer;border-radius:8px;color:#8a9bae;font-size:0.85rem;transition:background 0.2s;" onmouseover="this.style.background='rgba(0,217,255,0.08)'" onmouseout="this.style.background='transparent'">&#11168; ..</div>`;
    }
    if (d.entries && d.entries.length > 0) {
        d.entries.forEach(e => {
            html += `<div onclick="loadPathEntries('${e.path}')" style="display:flex;align-items:center;gap:8px;padding:8px 10px;cursor:pointer;border-radius:8px;color:#e5e5e5;font-size:0.85rem;transition:background 0.2s;" onmouseover="this.style.background='rgba(0,217,255,0.08)'" onmouseout="this.style.background='transparent'">&#128193; ${e.name}</div>`;
        });
    } else if (!d.parent) {
        html += '<p style="color:#5a6a7f;font-size:0.8rem;text-align:center;margin:12px 0;">No subdirectories</p>';
    } else {
        html += '<p style="color:#5a6a7f;font-size:0.8rem;text-align:center;margin:12px 0;">Empty folder</p>';
    }
    browser.innerHTML = html;
}

function selectBrowsePath(path) {
    // Multi-folder mode: set the active folder input
    if (activeFolderIdx !== null) {
        const inputs = document.querySelectorAll('.lib-folder-input');
        if (inputs[activeFolderIdx]) {
            inputs[activeFolderIdx].value = path;
        }
        activeFolderIdx = null;
    } else {
        // Fallback for single-path mode (legacy)
        const libPath = document.getElementById('libPath');
        if (libPath) libPath.value = path;
    }
    document.getElementById('pathBrowser').style.display = 'none';
}

async function createLibrary() {
    const name = document.getElementById('libName').value;
    const media_type = document.getElementById('libType').value;
    // Collect folders
    const folderInputs = document.querySelectorAll('.lib-folder-input');
    const folders = [...folderInputs].map(i => i.value.trim()).filter(Boolean);
    if (!name || folders.length === 0) { toast('Name and at least one folder required', 'error'); return; }

    const season_grouping = media_type === 'tv_shows' && document.querySelector('input[name="seasonGrouping"]:checked')?.value === 'yes';
    const access_level = document.getElementById('libAccess').value;
    const allowed_users = [...document.querySelectorAll('.user-perm-cb:checked')].map(cb => cb.value);
    const include_in_homepage = document.querySelector('input[name="includeHomepage"]:checked')?.value === 'yes';
    const include_in_search = document.querySelector('input[name="includeSearch"]:checked')?.value === 'yes';
    const retrieve_metadata = document.querySelector('input[name="retrieveMetadata"]:checked')?.value === 'yes';
    const nfo_import = document.querySelector('input[name="nfoImport"]:checked')?.value === 'yes';
    const nfo_export = document.querySelector('input[name="nfoExport"]:checked')?.value === 'yes';
    const prefer_local_artwork = document.querySelector('input[name="preferLocalArtwork"]:checked')?.value === 'yes';
    const create_previews = document.querySelector('input[name="createPreviews"]:checked')?.value === 'yes';
    const create_thumbnails = document.querySelector('input[name="createThumbnails"]:checked')?.value === 'yes';
    const audio_normalization = document.querySelector('input[name="audioNormalization"]:checked')?.value === 'yes';
    const scan_interval = document.getElementById('editScanInterval')?.value || 'disabled';
    const watch_enabled = document.querySelector('input[name="watchEnabled"]:checked')?.value === 'yes';

    let adult_content_type = null;
    if (media_type === 'adult_movies') {
        adult_content_type = document.querySelector('input[name="adultContentType"]:checked')?.value || 'movies';
    }

    const d = await api('POST', '/libraries', {
        name, media_type, path: folders[0], folders, is_enabled: true,
        season_grouping, access_level, allowed_users,
        include_in_homepage, include_in_search, retrieve_metadata,
        nfo_import, nfo_export, prefer_local_artwork,
        create_previews, create_thumbnails, audio_normalization, adult_content_type,
        scan_interval, watch_enabled
    });
    if (d.success) { toast('Library created!'); loadLibrariesView(); loadSidebarCounts(); }
    else toast('Failed: ' + d.error, 'error');
}

// ──── Collections ────
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
                            <div class="hover-meta">${ci.rating ? '<span class="hover-rating-badge">&#11088; ' + ci.rating.toFixed(1) + '</span>' : ''}<span>${meta}</span></div>
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
async function loadPerformersView() {
    const mc=document.getElementById('mainContent'); const isAdmin=currentUser&&currentUser.role==='admin';
    mc.innerHTML=`<div class="section-header"><h2 class="section-title">Performers</h2>${isAdmin?'<button class="btn-primary" onclick="showCreatePerformer()">+ Add Performer</button>':''}</div><div class="person-grid" id="performerGrid"><div class="spinner"></div></div>`;
    const data=await api('GET','/performers');
    const grid=document.getElementById('performerGrid');
    if(data.success&&data.data&&data.data.length>0){
        grid.innerHTML=data.data.map(p=>`<div class="person-card" onclick="loadPerformerDetail('${p.id}')"><div class="person-avatar">${p.photo_path?'<img src="'+p.photo_path+'">':'&#128100;'}</div><div class="person-name">${p.name}</div><div class="person-role">${p.performer_type} \u00b7 ${p.media_count||0} media</div></div>`).join('');
    } else grid.innerHTML='<div class="empty-state" style="grid-column:1/-1;"><div class="empty-state-icon">&#128100;</div><div class="empty-state-title">No performers</div><p>Add performers to link them to your media</p></div>';
}

async function loadPerformerDetail(id) {
    const mc=document.getElementById('mainContent');
    mc.innerHTML='<div class="spinner"></div> Loading...';
    const data=await api('GET','/performers/'+id);
    if(!data.success){mc.innerHTML='<div class="empty-state"><div class="empty-state-title">Performer not found</div></div>';return;}
    const p=data.data.performer; const media=data.data.media||[];
    const isAdmin = currentUser && currentUser.role === 'admin';
    let metaRows = '';
    if (p.birth_date) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Born</td><td>${new Date(p.birth_date).toLocaleDateString()}</td></tr>`;
    if (p.birth_place) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Birthplace</td><td>${p.birth_place}</td></tr>`;
    if (p.death_date) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Died</td><td>${new Date(p.death_date).toLocaleDateString()}</td></tr>`;
    if (p.aliases) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Aliases</td><td>${p.aliases}</td></tr>`;
    if (p.nationality) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Nationality</td><td>${p.nationality}</td></tr>`;
    if (p.height) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">Height</td><td>${p.height}</td></tr>`;
    if (p.tmdb_person_id) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">TMDB</td><td><a href="https://www.themoviedb.org/person/${p.tmdb_person_id}" target="_blank" style="color:#00D9FF;">#${p.tmdb_person_id}</a></td></tr>`;
    if (p.imdb_person_id) metaRows += `<tr><td style="color:#5a6a7f;padding:3px 16px 3px 0;">IMDB</td><td><a href="https://www.imdb.com/name/${p.imdb_person_id}/" target="_blank" style="color:#00D9FF;">${p.imdb_person_id}</a></td></tr>`;
    const extMeta = metaRows ? `<table style="width:100%;font-size:0.85rem;margin-top:12px;">${metaRows}</table>` : '';
    mc.innerHTML=`<div class="detail-hero"><div class="detail-poster">${p.photo_path?'<img src="'+p.photo_path+'">':'&#128100;'}</div><div class="detail-info"><h1>${p.name}</h1><div class="meta-row">${p.performer_type}${p.birth_date?' \u00b7 Born: '+new Date(p.birth_date).toLocaleDateString():''}</div>${p.bio?'<p class="description">'+p.bio+'</p>':''}<span class="tag tag-cyan">${p.media_count||0} media items</span>${extMeta}${isAdmin ? '<div style="margin-top:12px;display:flex;gap:8px;"><button class="btn-secondary btn-small" onclick="showEditPerformer(\''+id+'\')">&#9998; Edit</button><button class="btn-danger btn-small" onclick="deletePerformer(\''+id+'\')">Delete</button></div>' : ''}</div></div>
    ${media.length>0?'<h3 style="color:#00D9FF;margin-bottom:16px;">Linked Media</h3><div class="media-grid">'+media.map(renderMediaCard).join('')+'</div>':''}
    <button class="btn-secondary" onclick="loadPerformersView()">&#8592; Back</button>`;
}

function showCreatePerformer(){const mc=document.getElementById('mainContent');mc.innerHTML=`<div class="section-header"><h2 class="section-title">Add Performer</h2></div><div style="max-width:500px;"><div class="form-group"><label>Name</label><input type="text" id="perfName"></div><div class="form-group"><label>Type</label><select id="perfType"><option value="actor">Actor</option><option value="director">Director</option><option value="producer">Producer</option><option value="musician">Musician</option><option value="narrator">Narrator</option><option value="adult_performer">Adult Performer</option><option value="other">Other</option></select></div><div class="form-group"><label>Bio</label><textarea id="perfBio" rows="3"></textarea></div><button class="btn-primary" onclick="createPerformer()">Create</button><button class="btn-secondary" style="margin-left:12px;" onclick="loadPerformersView()">Cancel</button></div>`;}
async function createPerformer(){const n=document.getElementById('perfName').value,t=document.getElementById('perfType').value,b=document.getElementById('perfBio').value||null;if(!n){toast('Name required','error');return;}const d=await api('POST','/performers',{name:n,performer_type:t,bio:b});if(d.success){toast('Created!');loadPerformersView();}else toast(d.error,'error');}

async function deletePerformer(id) {
    if (!confirm('Delete this performer?')) return;
    const d = await api('DELETE', '/performers/' + id);
    if (d.success) { toast('Performer deleted'); loadPerformersView(); }
    else toast(d.error, 'error');
}

async function showEditPerformer(id) {
    const res = await api('GET', '/performers/' + id);
    if (!res.success) { toast('Failed to load performer', 'error'); return; }
    const p = res.data.performer;
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Performer</h2></div>
        <div style="max-width:560px;">
            <div class="form-group"><label>Name</label><input type="text" id="epName" value="${p.name || ''}"></div>
            <div class="form-group"><label>Type</label><select id="epType"><option value="actor" ${p.performer_type==='actor'?'selected':''}>Actor</option><option value="director" ${p.performer_type==='director'?'selected':''}>Director</option><option value="producer" ${p.performer_type==='producer'?'selected':''}>Producer</option><option value="musician" ${p.performer_type==='musician'?'selected':''}>Musician</option><option value="narrator" ${p.performer_type==='narrator'?'selected':''}>Narrator</option><option value="adult_performer" ${p.performer_type==='adult_performer'?'selected':''}>Adult Performer</option><option value="other" ${p.performer_type==='other'?'selected':''}>Other</option></select></div>
            <div class="form-group"><label>Bio</label><textarea id="epBio" rows="3">${p.bio || ''}</textarea></div>
            <div class="form-group"><label>Photo URL</label><input type="text" id="epPhoto" value="${p.photo_path || ''}" placeholder="URL to photo"></div>
            <div class="edit-field-row">
                <div class="form-group"><label>Birth Date</label><input type="date" id="epBirthDate" value="${p.birth_date ? p.birth_date.substring(0,10) : ''}"></div>
                <div class="form-group"><label>Birth Place</label><input type="text" id="epBirthPlace" value="${p.birth_place || ''}"></div>
            </div>
            <div class="edit-field-row">
                <div class="form-group"><label>Nationality</label><input type="text" id="epNationality" value="${p.nationality || ''}"></div>
                <div class="form-group"><label>Aliases</label><input type="text" id="epAliases" value="${p.aliases || ''}" placeholder="Comma-separated"></div>
            </div>
            <button class="btn-primary" onclick="savePerformerEdit('${id}')">Save Changes</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadPerformerDetail('${id}')">Cancel</button>
        </div>`;
}

async function savePerformerEdit(id) {
    const name = document.getElementById('epName').value.trim();
    if (!name) { toast('Name required', 'error'); return; }
    const d = await api('PUT', '/performers/' + id, {
        name,
        performer_type: document.getElementById('epType').value,
        bio: document.getElementById('epBio').value.trim() || null,
        photo_path: document.getElementById('epPhoto').value.trim() || null,
        birth_date: document.getElementById('epBirthDate').value || null,
        birth_place: document.getElementById('epBirthPlace').value.trim() || null,
        nationality: document.getElementById('epNationality').value.trim() || null,
        aliases: document.getElementById('epAliases').value.trim() || null
    });
    if (d.success) { toast('Performer updated'); loadPerformerDetail(id); }
    else toast(d.error, 'error');
}

// ──── Tags ────
async function loadTagsView() {
    const mc=document.getElementById('mainContent'); const isAdmin=currentUser&&currentUser.role==='admin';
    mc.innerHTML=`<div class="section-header"><h2 class="section-title">Tags & Genres</h2>${isAdmin?'<button class="btn-primary" onclick="showCreateTag()">+ Add Tag</button>':''}</div><div id="tagsList"><div class="spinner"></div></div>`;
    const data=await api('GET','/tags?tree=true');
    const div=document.getElementById('tagsList');
    if(data.success&&data.data&&data.data.length>0){
        div.innerHTML=data.data.map(t=>renderTag(t,0)).join('');
    } else div.innerHTML='<div class="empty-state"><div class="empty-state-icon">&#127991;</div><div class="empty-state-title">No tags</div></div>';
}

function renderTag(tag, depth) {
    const indent = depth * 24;
    const isAdmin = currentUser && currentUser.role === 'admin';
    let html = `<div class="group-card" style="margin-left:${indent}px;display:flex;justify-content:space-between;align-items:center;"><div><h4>${tag.name}</h4>${tag.description ? '<p style="color:#5a6a7f;font-size:0.78rem;margin:2px 0;">'+tag.description+'</p>' : ''}<span class="tag tag-${tag.category==='genre'?'purple':tag.category==='custom'?'orange':'cyan'}">${tag.category}</span><span class="tag tag-green">${tag.media_count||0} media</span></div><div style="display:flex;gap:6px;">${isAdmin ? '<button class="btn-secondary btn-small" onclick="showEditTag(\''+tag.id+'\',\''+tag.name.replace(/'/g,"\\'")+'\',\''+tag.category+'\',\''+(tag.description||'').replace(/'/g,"\\'")+'\')">&#9998;</button><button class="btn-danger btn-small" onclick="deleteTag(\''+tag.id+'\')">Delete</button>' : ''}</div></div>`;
    if (tag.children) tag.children.forEach(c => html += renderTag(c, depth+1));
    return html;
}

function showCreateTag(){const mc=document.getElementById('mainContent');mc.innerHTML=`<div class="section-header"><h2 class="section-title">Add Tag</h2></div><div style="max-width:500px;"><div class="form-group"><label>Name</label><input type="text" id="tagName"></div><div class="form-group"><label>Category</label><select id="tagCat"><option value="genre">Genre</option><option value="tag">Tag</option><option value="custom">Custom</option></select></div><div class="form-group"><label>Description</label><input type="text" id="tagDesc"></div><button class="btn-primary" onclick="createTag()">Create</button><button class="btn-secondary" style="margin-left:12px;" onclick="loadTagsView()">Cancel</button></div>`;}
async function createTag(){const n=document.getElementById('tagName').value,c=document.getElementById('tagCat').value,d=document.getElementById('tagDesc').value||null;if(!n){toast('Name required','error');return;}const r=await api('POST','/tags',{name:n,category:c,description:d});if(r.success){toast('Created!');loadTagsView();}else toast(r.error,'error');}
async function deleteTag(id){if(!confirm('Delete?'))return;const d=await api('DELETE','/tags/'+id);if(d.success){toast('Deleted');loadTagsView();}else toast(d.error,'error');}

function showEditTag(id, name, category, description) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Tag</h2></div>
        <div style="max-width:500px;">
            <div class="form-group"><label>Name</label><input type="text" id="editTagName" value="${name}"></div>
            <div class="form-group"><label>Category</label><select id="editTagCat"><option value="genre" ${category==='genre'?'selected':''}>Genre</option><option value="tag" ${category==='tag'?'selected':''}>Tag</option><option value="custom" ${category==='custom'?'selected':''}>Custom</option></select></div>
            <div class="form-group"><label>Description</label><input type="text" id="editTagDesc" value="${description}"></div>
            <button class="btn-primary" onclick="saveTagEdit('${id}')">Save</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadTagsView()">Cancel</button>
        </div>`;
}

async function saveTagEdit(id) {
    const name = document.getElementById('editTagName').value.trim();
    if (!name) { toast('Name required', 'error'); return; }
    const d = await api('PUT', '/tags/' + id, {
        name,
        category: document.getElementById('editTagCat').value,
        description: document.getElementById('editTagDesc').value.trim() || null
    });
    if (d.success) { toast('Tag updated'); loadTagsView(); }
    else toast(d.error, 'error');
}

async function addTagToMedia(mediaId) {
    const sel = document.getElementById('tagAssignSelect');
    if (!sel || !sel.value) { toast('Select a tag first', 'error'); return; }
    const d = await api('POST', '/media/' + mediaId + '/tags/' + sel.value);
    if (d.success) { toast('Tag added'); showDetailTab(document.querySelector('.detail-tab.active') || document.querySelector('.detail-tab'), 'tags-tab', mediaId); }
    else toast(d.error || 'Failed to add tag', 'error');
}

async function removeTagFromMedia(mediaId, tagId) {
    const d = await api('DELETE', '/media/' + mediaId + '/tags/' + tagId);
    if (d.success) { toast('Tag removed'); showDetailTab(document.querySelector('.detail-tab.active') || document.querySelector('.detail-tab'), 'tags-tab', mediaId); }
    else toast(d.error || 'Failed to remove tag', 'error');
}

// ──── Studios ────
async function loadStudiosView() {
    const mc=document.getElementById('mainContent'); const isAdmin=currentUser&&currentUser.role==='admin';
    mc.innerHTML=`<div class="section-header"><h2 class="section-title">Studios / Labels</h2>${isAdmin?'<button class="btn-primary" onclick="showCreateStudio()">+ Add Studio</button>':''}</div><div id="studiosList"><div class="spinner"></div></div>`;
    const data=await api('GET','/studios');
    const div=document.getElementById('studiosList');
    if(data.success&&data.data&&data.data.length>0){
        div.innerHTML=data.data.map(s=>`<div class="group-card" style="cursor:pointer;" onclick="loadStudioDetail('${s.id}')"><div style="display:flex;justify-content:space-between;align-items:center;"><div><h4>${s.name}</h4>${s.website ? '<a href="'+s.website+'" target="_blank" style="color:#00D9FF;font-size:0.78rem;" onclick="event.stopPropagation();">'+s.website+'</a>' : ''}<div style="margin-top:4px;"><span class="tag tag-cyan">${s.studio_type}</span><span class="tag tag-green">${s.media_count||0} media</span></div></div><div style="display:flex;gap:6px;">${isAdmin?'<button class="btn-secondary btn-small" onclick="event.stopPropagation();showEditStudio(\''+s.id+'\',\''+s.name.replace(/'/g,"\\'")+'\',\''+s.studio_type+'\',\''+(s.website||'').replace(/'/g,"\\'")+'\')">&#9998;</button><button class="btn-danger btn-small" onclick="event.stopPropagation();deleteStudio(\''+s.id+'\')">Delete</button>':''}</div></div></div>`).join('');
    } else div.innerHTML='<div class="empty-state"><div class="empty-state-icon">&#127980;</div><div class="empty-state-title">No studios</div></div>';
}
function showCreateStudio(){const mc=document.getElementById('mainContent');mc.innerHTML=`<div class="section-header"><h2 class="section-title">Add Studio</h2></div><div style="max-width:500px;"><div class="form-group"><label>Name</label><input type="text" id="studioName"></div><div class="form-group"><label>Type</label><select id="studioType"><option value="studio">Studio</option><option value="label">Label</option><option value="publisher">Publisher</option><option value="network">Network</option><option value="distributor">Distributor</option></select></div><div class="form-group"><label>Website</label><input type="text" id="studioWeb"></div><button class="btn-primary" onclick="createStudio()">Create</button><button class="btn-secondary" style="margin-left:12px;" onclick="loadStudiosView()">Cancel</button></div>`;}
async function createStudio(){const n=document.getElementById('studioName').value,t=document.getElementById('studioType').value,w=document.getElementById('studioWeb').value||null;if(!n){toast('Name required','error');return;}const d=await api('POST','/studios',{name:n,studio_type:t,website:w});if(d.success){toast('Created!');loadStudiosView();}else toast(d.error,'error');}
async function deleteStudio(id){if(!confirm('Delete?'))return;const d=await api('DELETE','/studios/'+id);if(d.success){toast('Deleted');loadStudiosView();}else toast(d.error,'error');}

async function loadStudioDetail(id) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div>';
    const data = await api('GET', '/studios/' + id);
    if (!data.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Studio not found</div></div>'; return; }
    const s = data.data.studio || data.data;
    const media = data.data.media || [];
    const isAdmin = currentUser && currentUser.role === 'admin';
    mc.innerHTML = `<div class="detail-hero"><div class="detail-poster" style="font-size:3rem;">&#127980;</div>
        <div class="detail-info"><h1>${s.name}</h1>
            <div class="meta-row">${s.studio_type}${s.website ? ' &middot; <a href="'+s.website+'" target="_blank" style="color:#00D9FF;">'+s.website+'</a>' : ''}</div>
            <span class="tag tag-cyan">${s.media_count || media.length} media items</span>
            ${isAdmin ? '<div style="margin-top:12px;display:flex;gap:8px;"><button class="btn-secondary btn-small" onclick="showEditStudio(\''+id+'\',\''+s.name.replace(/'/g,"\\'")+'\',\''+s.studio_type+'\',\''+(s.website||'').replace(/'/g,"\\'")+'\')">&#9998; Edit</button><button class="btn-danger btn-small" onclick="deleteStudio(\''+id+'\')">Delete</button></div>' : ''}
        </div></div>
    ${media.length > 0 ? '<h3 style="color:#00D9FF;margin-bottom:16px;">Media</h3><div class="media-grid">' + media.map(renderMediaCard).join('') + '</div>' : ''}
    <button class="btn-secondary" onclick="loadStudiosView()">&#8592; Back</button>`;
}

function showEditStudio(id, name, studioType, website) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Studio</h2></div>
        <div style="max-width:500px;">
            <div class="form-group"><label>Name</label><input type="text" id="editStudioName" value="${name}"></div>
            <div class="form-group"><label>Type</label><select id="editStudioType"><option value="studio" ${studioType==='studio'?'selected':''}>Studio</option><option value="label" ${studioType==='label'?'selected':''}>Label</option><option value="publisher" ${studioType==='publisher'?'selected':''}>Publisher</option><option value="network" ${studioType==='network'?'selected':''}>Network</option><option value="distributor" ${studioType==='distributor'?'selected':''}>Distributor</option></select></div>
            <div class="form-group"><label>Website</label><input type="text" id="editStudioWeb" value="${website}" placeholder="https://..."></div>
            <button class="btn-primary" onclick="saveStudioEdit('${id}')">Save</button>
            <button class="btn-secondary" style="margin-left:12px;" onclick="loadStudiosView()">Cancel</button>
        </div>`;
}

async function saveStudioEdit(id) {
    const name = document.getElementById('editStudioName').value.trim();
    if (!name) { toast('Name required', 'error'); return; }
    const d = await api('PUT', '/studios/' + id, {
        name,
        studio_type: document.getElementById('editStudioType').value,
        website: document.getElementById('editStudioWeb').value.trim() || null
    });
    if (d.success) { toast('Studio updated'); loadStudiosView(); }
    else toast(d.error, 'error');
}

// ──── Duplicates ────
async function loadDuplicatesView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Duplicate Review</h2></div>
        <p style="color:#8a9bae;margin-bottom:20px;">Review media items flagged as exact (MD5) or potential (phash) duplicates</p>
        <div id="dupList"><div class="spinner"></div></div>`;
    const data = await api('GET', '/duplicates');
    const div = document.getElementById('dupList');
    if (!data.success) { div.innerHTML = '<p style="color:#ff6b6b;">Failed to load duplicates</p>'; return; }
    const groups = data.data && data.data.groups ? data.data.groups : [];
    if (groups.length === 0) {
        div.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#128257;</div><div class="empty-state-title">No duplicates found</div><p>Scan libraries to detect duplicates via MD5 and perceptual hashing</p></div>';
        return;
    }
    div.innerHTML = groups.map(g => renderDuplicateGroup(g)).join('');
}

function renderDuplicateGroup(g) {
    const item = g.item;
    const typeClass = g.dup_type === 'exact' ? 'dup-type-exact' : 'dup-type-potential';
    const typeLabel = g.dup_type === 'exact' ? 'Exact Duplicate' : 'Potential Duplicate';
    const matches = g.matches || [];
    const decisions = g.decisions || [];

    let decisionsHtml = '';
    if (decisions.length > 0) {
        decisionsHtml = `<div class="dup-decisions"><strong>Prior decisions:</strong> ` +
            decisions.map(d => `${d.action} by ${d.decided_by} on ${new Date(d.decided_at).toLocaleDateString()}`
                + (d.notes ? ' - ' + d.notes : '')).join('; ') + '</div>';
    }

    let matchesHtml = '';
    if (matches.length > 0) {
        matchesHtml = `<div class="dup-compare">` + renderDupCard(item, 'Source') +
            matches.map(m => renderDupCard(m.item, m.match_type === 'md5' ?
                `<span class="dup-match-badge dup-match-md5">MD5 Match</span>` :
                `<span class="dup-match-badge dup-match-phash">${Math.round(m.similarity * 100)}% Similar</span>`
            )).join('') + `</div>`;
    }

    const bestMatch = matches.length > 0 ? matches[0] : null;
    const partnerId = bestMatch ? bestMatch.item.id : '';
    const partnerTitle = bestMatch ? bestMatch.item.title : '';

    return `<div class="dup-group">
        <div class="dup-group-header">
            <h3 style="color:#e5e5e5;">${item.title}${item.year ? ' (' + item.year + ')' : ''}</h3>
            <span class="dup-type-badge ${typeClass}">${typeLabel}</span>
        </div>
        ${matchesHtml}
        ${decisionsHtml}
        <div class="dup-actions">
            <button class="btn-edit btn-small" onclick="dupEdit('${item.id}','${partnerId}')">&#9998; Edit</button>
            <button class="btn-edition btn-small" onclick="dupMergeEdition('${item.id}','${partnerId}','${item.title.replace(/'/g,"\\'")}','${partnerTitle.replace(/'/g,"\\'")}')">&#128191; Merge as Edition</button>
            <button class="btn-danger btn-small" onclick="dupDelete('${item.id}','${partnerId}')">&#128465; Delete</button>
            <button class="btn-secondary btn-small" onclick="dupIgnore('${item.id}','${partnerId}')">&#128683; Ignore</button>
        </div>
    </div>`;
}

function renderDupCard(item, label) {
    const fileSize = item.file_size ? (item.file_size / (1024*1024*1024)).toFixed(2) + ' GB' : '';
    const res = item.resolution || '';
    const filePath = item.file_path || '';
    const shortPath = filePath.length > 60 ? '...' + filePath.slice(-57) : filePath;
    return `<div class="dup-card">
        <div class="dup-card-header">
            <div class="dup-card-poster">${item.poster_path ? '<img src="'+posterSrc(item.poster_path, item.updated_at)+'">' : '<div style="display:flex;align-items:center;justify-content:center;height:100%;font-size:2rem;color:#4a5568;">&#127910;</div>'}</div>
            <div class="dup-card-meta">
                <h4>${item.title}</h4>
                <p>${item.year || 'N/A'}${res ? ' &middot; ' + res : ''}${fileSize ? ' &middot; ' + fileSize : ''}</p>
                <p title="${filePath}" style="word-break:break-all;">${shortPath}</p>
                <div style="margin-top:6px;">${label}</div>
            </div>
        </div>
    </div>`;
}

async function dupEdit(itemId, partnerId) {
    // Open the edit modal, then mark as addressed on save
    openEditModal(itemId);
    // After save, mark both as addressed
    window._dupResolveAfterEdit = { itemId, partnerId };
}

async function dupIgnore(itemId, partnerId) {
    const d = await api('POST', '/duplicates/resolve', { media_id: itemId, partner_id: partnerId, action: 'ignored' });
    if (d.success) { toast('Marked as ignored'); loadDuplicatesView(); loadSidebarCounts(); } else toast(d.error, 'error');
}

async function dupDelete(itemId, partnerId) {
    if (!confirm('Delete this item from the database?')) return;
    const deleteFile = confirm('Also delete the file from disk? (Cannot be undone)');
    const d = await api('POST', '/duplicates/resolve', { media_id: itemId, partner_id: partnerId, action: 'deleted', delete_file: deleteFile });
    if (d.success) { toast('Item deleted'); loadDuplicatesView(); loadSidebarCounts(); } else toast(d.error, 'error');
}

function dupMergeEdition(itemA, itemB, titleA, titleB) {
    document.getElementById('mergeItemA').value = itemA;
    document.getElementById('mergeItemB').value = itemB;
    const opts = document.getElementById('mergePrimaryOptions');
    opts.innerHTML = `
        <div class="merge-option selected" onclick="selectMergePrimary(this,'${itemA}')">
            <label><input type="radio" name="mergePrimary" value="${itemA}" checked> ${titleA} (Source)</label>
        </div>
        <div class="merge-option" onclick="selectMergePrimary(this,'${itemB}')">
            <label><input type="radio" name="mergePrimary" value="${itemB}"> ${titleB} (Match)</label>
        </div>`;
    document.getElementById('mergeEditionOverlay').classList.add('active');
}

function selectMergePrimary(el, id) {
    document.querySelectorAll('.merge-option').forEach(o => o.classList.remove('selected'));
    el.classList.add('selected');
    el.querySelector('input[type="radio"]').checked = true;
}

function closeMergeModal() {
    document.getElementById('mergeEditionOverlay').classList.remove('active');
}

async function submitMergeEdition() {
    const itemA = document.getElementById('mergeItemA').value;
    const itemB = document.getElementById('mergeItemB').value;
    const label = document.getElementById('mergeEditionLabel').value;
    const primaryId = document.querySelector('input[name="mergePrimary"]:checked').value;
    const d = await api('POST', '/duplicates/resolve', {
        media_id: itemA, partner_id: itemB, action: 'edition',
        edition_label: label, primary_id: primaryId
    });
    if (d.success) { toast('Merged as edition!'); closeMergeModal(); loadDuplicatesView(); loadSidebarCounts(); }
    else toast(d.error, 'error');
}

// Close merge modal on overlay click
document.getElementById('mergeEditionOverlay').addEventListener('click', function(e) {
    if (e.target === this) closeMergeModal();
});

// ──── Metadata Identify ────
let _identifyMatches = [];
let _identifyMediaId = '';

async function identifyMedia(id) {
    const mc=document.getElementById('mainContent');
    mc.innerHTML='<div class="section-header"><h2 class="section-title">Identify Media</h2></div><div id="matchList"><div class="spinner"></div> Searching external sources...</div>';
    const data=await api('POST','/media/'+id+'/identify');
    const div=document.getElementById('matchList');
    _identifyMediaId = id;
    _identifyMatches = (data.success && data.data) ? data.data : [];
    if(_identifyMatches.length>0){
        const esc = (s) => s ? s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;') : '';
        div.innerHTML=_identifyMatches.map((m,i)=>`<div class="group-card"><div style="display:flex;gap:16px;align-items:start;"><div style="width:80px;min-width:80px;aspect-ratio:2/3;border-radius:10px;overflow:hidden;background:rgba(0,0,0,0.3);">${m.poster_url?'<img src="'+m.poster_url+'" style="width:100%;height:100%;object-fit:cover;">':'&#128247;'}</div><div><h4>${esc(m.title)}${m.year?' ('+m.year+')':''}</h4><p>${m.description?esc(m.description.substring(0,200))+'...':''}</p><span class="tag tag-cyan">${m.source}</span><span class="tag tag-green">${Math.round(m.confidence*100)}% match</span><button class="btn-primary btn-small" style="margin-top:8px;" onclick="applyMatchByIndex(${i})">Apply</button></div></div></div>`).join('');
    } else div.innerHTML='<div class="empty-state"><div class="empty-state-title">No matches found</div></div>';
    div.innerHTML+='<button class="btn-secondary" style="margin-top:16px;" onclick="loadMediaDetail(\''+id+'\')">&#8592; Back</button>';
}

async function applyMatchByIndex(idx) {
    const match = _identifyMatches[idx];
    if (!match) { toast('Match not found','error'); return; }
    const d=await api('POST','/media/'+_identifyMediaId+'/apply-meta',match);
    if(d.success){toast('Metadata applied!');loadMediaDetail(_identifyMediaId);}else toast(d.error,'error');
}

// ──── Toggle Known Edition Details ────
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
async function openEditModal(id) {
    const data = await api('GET', '/media/' + id);
    if (!data.success) { toast('Failed to load media', 'error'); return; }
    const m = data.data;
    document.getElementById('editMediaId').value = m.id;
    document.getElementById('editTitle').value = m.title || '';
    document.getElementById('editSortTitle').value = m.sort_title || '';
    document.getElementById('editOriginalTitle').value = m.original_title || '';
    document.getElementById('editDescription').value = m.description || '';
    document.getElementById('editYear').value = m.year || '';
    document.getElementById('editReleaseDate').value = m.release_date ? m.release_date.substring(0,10) : '';
    document.getElementById('editRating').value = m.rating != null ? m.rating : '';
    // Edition type (always shown)
    document.getElementById('editEditionType').value = m.edition_type || 'Theatrical';
    // New fields: content rating, language, tagline, country, trailer, genres, studio
    document.getElementById('editContentRating').value = m.content_rating || '';
    document.getElementById('editOriginalLanguage').value = m.original_language || '';
    document.getElementById('editTagline').value = m.tagline || '';
    document.getElementById('editCountry').value = m.country || '';
    document.getElementById('editTrailerUrl').value = m.trailer_url || '';
    document.getElementById('editGenres').value = (m.genres || []).join(', ');
    document.getElementById('editStudio').value = m.studio || '';
    // Poster and backdrop
    document.getElementById('editPosterPath').value = m.poster_path || '';
    document.getElementById('editBackdropPath').value = m.backdrop_path || '';
    // Custom notes and tags
    document.getElementById('editCustomNotes').value = m.custom_notes || '';
    const ctRaw = m.custom_tags ? (typeof m.custom_tags === 'string' ? JSON.parse(m.custom_tags || '{}') : m.custom_tags) : {};
    const ctArr = ctRaw.tags || [];
    document.getElementById('editCustomTags').value = ctArr.join(', ');
    // Cast display
    document.getElementById('editCastPanel').style.display = 'none';
    document.getElementById('editCastToggle').classList.remove('open');
    loadEditCast(m.id);
    // Parent Movie field (movies and adult_movies only)
    const parentGroup = document.getElementById('parentMovieGroup');
    const parentSearch = document.getElementById('parentMovieSearch');
    const parentIdField = document.getElementById('parentMovieId');
    const parentInfo = document.getElementById('parentCurrentInfo');
    parentSearch.value = '';
    parentIdField.value = '';
    parentInfo.style.display = 'none';
    parentInfo.innerHTML = '';
    document.getElementById('parentSearchResults').classList.remove('active');
    if (m.media_type === 'movies' || m.media_type === 'adult_movies') {
        parentGroup.style.display = '';
        // Check if already in an edition group
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
    } else {
        parentGroup.style.display = 'none';
    }
    // Series info (movies and adult_movies only)
    const seriesGroup = document.getElementById('editSeriesGroup');
    const seriesInfo = document.getElementById('editSeriesInfo');
    seriesInfo.innerHTML = '';
    if (m.media_type === 'movies' || m.media_type === 'adult_movies') {
        seriesGroup.style.display = '';
        loadEditSeriesInfo(m.id);
    } else {
        seriesGroup.style.display = 'none';
    }
    // Lock status & per-field locks
    const lockedFields = m.locked_fields || [];
    renderFieldLocks(lockedFields);
    // Collapse the field locks panel by default
    document.getElementById('fieldLocksPanel').style.display = 'none';
    document.getElementById('fieldLocksToggle').classList.remove('open');

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
    document.getElementById('editMediaOverlay').classList.add('active');
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

    const d = await api('PUT', '/media/' + id, {
        title, sort_title: sortTitle, original_title: originalTitle,
        description, year, release_date: releaseDate, rating,
        edition_type: editionType, poster_path: posterPath, backdrop_path: backdropPath,
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

function toggleFieldLocks() {
    const panel = document.getElementById('fieldLocksPanel');
    const toggle = document.getElementById('fieldLocksToggle');
    const isOpen = panel.style.display !== 'none';
    panel.style.display = isOpen ? 'none' : '';
    toggle.classList.toggle('open', !isOpen);
}

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

async function showLibrarySeries() {
    const libId = _gridState.libraryId;
    if (!libId) return;

    const wrapper = document.getElementById('mediaGridWrapper');
    const collArea = document.getElementById('collectionsArea');
    const serArea = document.getElementById('seriesArea');
    if (wrapper) wrapper.style.display = 'none';
    if (collArea) collArea.style.display = 'none';
    if (serArea) serArea.style.display = 'block';

    const gridBtn = document.getElementById('ftGridBtn');
    const collBtn = document.getElementById('ftCollBtn');
    const serBtn = document.getElementById('ftSeriesBtn');
    if (gridBtn) gridBtn.classList.remove('active');
    if (collBtn) collBtn.classList.remove('active');
    if (serBtn) serBtn.classList.add('active');

    serArea.innerHTML = '<div class="spinner"></div> Loading series...';
    const data = await api('GET', '/series?library_id=' + libId);
    const seriesList = (data.success && data.data) ? data.data : [];

    if (seriesList.length === 0) {
        serArea.innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#127910;</div><div class="empty-state-title">No series</div><p>Add movies to a series from the edit modal</p></div>';
        return;
    }

    serArea.innerHTML = '<div class="series-grid">' + seriesList.map(s => {
        const poster = s.poster_path
            ? `<img src="${posterSrc(s.poster_path, s.updated_at)}" alt="">`
            : '&#127910;';
        return `<div class="series-card" onclick="loadSeriesDetail('${s.id}')">
            <div class="sc-poster">${poster}</div>
            <div class="sc-info">
                <div class="sc-name">${s.name}</div>
                <div class="sc-meta">${s.item_count || 0} movie${(s.item_count||0) !== 1 ? 's' : ''}</div>
            </div>
        </div>`;
    }).join('') + '</div>';
}

async function loadSeriesDetail(seriesId) {
    _currentNav = { view: '__series', extra: seriesId };
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner"></div> Loading...';
    const data = await api('GET', '/series/' + seriesId);
    if (!data.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Series not found</div></div>'; return; }
    const s = data.data;
    const richItems = s.rich_items || [];
    const count = richItems.length || (s.items || []).length;

    // Build media card grid from rich items (edition-grouped, full badges)
    let gridHTML = '';
    if (richItems.length > 0) {
        gridHTML = '<div class="media-grid">' + richItems.map(item => {
            const card = renderMediaCard(item);
            // Inject remove-from-series overlay button into the poster area
            const siId = item.series_item_id || '';
            const removeBtn = `<button class="smi-remove-overlay" onclick="event.stopPropagation();removeFromSeries('${s.id}','${siId}')" title="Remove from series">&#10005;</button>`;
            return card.replace('<div class="play-overlay">', removeBtn + '<div class="play-overlay">');
        }).join('') + '</div>';
    } else {
        gridHTML = '<div class="empty-state"><p>No movies in this series yet</p></div>';
    }

    mc.innerHTML = `
        <div class="series-detail-header">
            <h2>${s.name}</h2>
            <span class="tag tag-cyan">${count} movie${count !== 1 ? 's' : ''}</span>
            <div class="series-detail-actions">
                <button class="btn-danger" onclick="deleteSeries('${s.id}')">&#128465; Delete Series</button>
            </div>
        </div>
        ${gridHTML}
        <button class="btn-secondary" style="margin-top:20px;" onclick="navigate('library','${s.library_id}')">&#8592; Back to Library</button>`;
}

async function deleteSeries(seriesId) {
    if (!confirm('Delete this series? Movies will not be deleted.')) return;
    const d = await api('DELETE', '/series/' + seriesId);
    if (d.success) {
        toast('Series deleted');
        history.back();
    } else {
        toast(d.error || 'Failed to delete', 'error');
    }
}

async function removeFromSeries(seriesId, itemId) {
    if (!confirm('Remove this movie from the series?')) return;
    const d = await api('DELETE', '/series/' + seriesId + '/items/' + itemId);
    if (d.success) {
        toast('Removed from series');
        loadSeriesDetail(seriesId);
    } else {
        toast(d.error || 'Failed to remove', 'error');
    }
}

// ── Add to Series Modal ──
function openSeriesModal() {
    const mediaId = document.getElementById('editMediaId').value;
    document.getElementById('seriesMediaId').value = mediaId;
    document.getElementById('seriesNameInput').value = '';
    document.getElementById('seriesSelectedId').value = '';
    document.getElementById('seriesOrderInput').value = '1';

    // Load series list for this library
    const libId = _gridState.libraryId;
    if (libId) {
        api('GET', '/series?library_id=' + libId).then(data => {
            _seriesLibraryList = (data.success && data.data) ? data.data : [];
            renderSeriesDropdown(_seriesLibraryList);
        });
    }

    document.getElementById('seriesModalOverlay').classList.add('active');
}

function closeSeriesModal() {
    document.getElementById('seriesModalOverlay').classList.remove('active');
    document.getElementById('seriesDropdown').style.display = 'none';
}

function renderSeriesDropdown(list) {
    const dd = document.getElementById('seriesDropdown');
    if (list.length === 0) {
        dd.innerHTML = '<div style="padding:10px 14px;color:#5a6a7f;font-size:0.82rem;">No existing series — type a name to create one</div>';
    } else {
        dd.innerHTML = list.map(s =>
            `<div style="padding:10px 14px;color:#b8c5d6;font-size:0.85rem;cursor:pointer;transition:background 0.15s;" onmouseenter="this.style.background='rgba(0,217,255,0.1)'" onmouseleave="this.style.background='none'" onclick="selectSeriesOption('${s.id}','${s.name.replace(/'/g,"\\'")}')">${s.name} <span style="color:#5a6a7f;font-size:0.75rem;">(${s.item_count||0} movies)</span></div>`
        ).join('');
    }
}

function showSeriesDropdown() {
    const dd = document.getElementById('seriesDropdown');
    renderSeriesDropdown(_seriesLibraryList);
    dd.style.display = 'block';
}

function filterSeriesDropdown(query) {
    const dd = document.getElementById('seriesDropdown');
    document.getElementById('seriesSelectedId').value = '';
    if (!query.trim()) {
        renderSeriesDropdown(_seriesLibraryList);
        dd.style.display = 'block';
        return;
    }
    const filtered = _seriesLibraryList.filter(s => s.name.toLowerCase().includes(query.toLowerCase()));
    if (filtered.length === 0) {
        dd.innerHTML = '<div style="padding:10px 14px;color:#00D9FF;font-size:0.82rem;">&#10010; Create "' + query + '"</div>';
    } else {
        renderSeriesDropdown(filtered);
    }
    dd.style.display = 'block';
}

function selectSeriesOption(id, name) {
    document.getElementById('seriesNameInput').value = name;
    document.getElementById('seriesSelectedId').value = id;
    document.getElementById('seriesDropdown').style.display = 'none';
}

async function saveToSeries() {
    const mediaId = document.getElementById('seriesMediaId').value;
    const name = document.getElementById('seriesNameInput').value.trim();
    let seriesId = document.getElementById('seriesSelectedId').value;
    const sortOrder = parseInt(document.getElementById('seriesOrderInput').value) || 1;

    if (!name) { toast('Series name is required', 'error'); return; }

    // If no existing series selected, create a new one
    if (!seriesId) {
        const libId = _gridState.libraryId;
        if (!libId) { toast('Library not found', 'error'); return; }
        const createRes = await api('POST', '/series', { library_id: libId, name: name });
        if (!createRes.success) { toast(createRes.error || 'Failed to create series', 'error'); return; }
        seriesId = createRes.data.id;
    }

    // Add the movie to the series
    const addRes = await api('POST', '/series/' + seriesId + '/items', {
        media_item_id: mediaId,
        sort_order: sortOrder
    });
    if (addRes.success) {
        toast('Added to series "' + name + '"');
        closeSeriesModal();
        // Refresh edit modal series info
        loadEditSeriesInfo(mediaId);
    } else {
        toast(addRes.error || 'Failed to add to series', 'error');
    }
}

async function loadEditSeriesInfo(mediaId) {
    const seriesGroup = document.getElementById('editSeriesGroup');
    const seriesInfo = document.getElementById('editSeriesInfo');
    const data = await api('GET', '/media/' + mediaId + '/series');
    if (data.success && data.data && data.data.in_series) {
        const s = data.data;
        seriesInfo.innerHTML = `<div class="series-current-info"><span class="sci-name">&#127910; ${s.series.name} — #${s.sort_order}</span><button class="sci-remove" onclick="removeMediaFromSeries('${s.series.id}','${s.item_id}','${mediaId}')">Remove</button></div>`;
    } else {
        seriesInfo.innerHTML = '';
    }
}

async function removeMediaFromSeries(seriesId, itemId, mediaId) {
    if (!confirm('Remove this movie from the series?')) return;
    const d = await api('DELETE', '/series/' + seriesId + '/items/' + itemId);
    if (d.success) {
        toast('Removed from series');
        loadEditSeriesInfo(mediaId);
    } else {
        toast(d.error || 'Failed to remove', 'error');
    }
}

// Close series dropdown on click outside
document.addEventListener('click', function(e) {
    const dd = document.getElementById('seriesDropdown');
    const inp = document.getElementById('seriesNameInput');
    if (dd && inp && !dd.contains(e.target) && e.target !== inp) {
        dd.style.display = 'none';
    }
});

// Close series modal on overlay click
document.getElementById('seriesModalOverlay').addEventListener('click', function(e) {
    if (e.target === this) closeSeriesModal();
});

// ──── Edit Profile ────
async function loadProfileView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = `<div class="section-header"><h2 class="section-title">Edit Profile</h2></div><div class="settings-grid" id="profileGrid"><div class="spinner"></div></div>`;

    const profileData = await api('GET', '/profile');
    if (!profileData.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load profile</div></div>'; return; }
    const u = profileData.data;

    // Get PIN length setting for validation
    let pinLength = 4;
    try {
        const flSettings = await api('GET', '/auth/fast-login/settings');
        if (flSettings.success && flSettings.data) pinLength = parseInt(flSettings.data.fast_login_pin_length) || 4;
    } catch(e) {}

    document.getElementById('profileGrid').innerHTML = `
        <div class="settings-card">
            <h3>Personal Information</h3>
            <div class="form-group"><label>Username</label><input type="text" value="${u.username || ''}" disabled style="opacity:0.6;cursor:not-allowed;"></div>
            <div class="edit-field-row">
                <div class="form-group"><label>First Name</label><input type="text" id="profFirstName" value="${u.first_name || ''}" placeholder="Enter first name"></div>
                <div class="form-group"><label>Last Name</label><input type="text" id="profLastName" value="${u.last_name || ''}" placeholder="Enter last name"></div>
            </div>
            <div class="form-group"><label>Email</label><input type="email" id="profEmail" value="${u.email || ''}" placeholder="Enter email"></div>
            <button class="btn-primary" onclick="saveProfile()">Save Changes</button>
        </div>
        <div class="settings-card">
            <h3>Security</h3>
            <div class="form-group">
                <label>New Password</label>
                <input type="password" id="profPassword" placeholder="Leave blank to keep current">
            </div>
            <div class="form-group">
                <label>Confirm Password</label>
                <input type="password" id="profPasswordConfirm" placeholder="Confirm new password">
            </div>
            <button class="btn-primary" onclick="saveProfilePassword()" style="margin-bottom:20px;">Change Password</button>
            <div style="border-top:1px solid rgba(0,217,255,0.1);padding-top:16px;margin-top:4px;">
                <h3 style="margin-bottom:12px;">Login PIN</h3>
                <p style="color:#8a9bae;font-size:0.82rem;margin-bottom:12px;">Set a ${pinLength}-digit PIN for quick login. ${u.has_pin ? '<span style="color:#51cf66;">PIN is set.</span>' : '<span style="color:#5a6a7f;">No PIN set.</span>'}</p>
                <div class="form-group">
                    <label>New PIN (${pinLength} digits)</label>
                    <input type="password" id="profPin" placeholder="Enter ${pinLength}-digit PIN" maxlength="${pinLength}" pattern="[0-9]*" inputmode="numeric">
                </div>
                <button class="btn-primary" onclick="saveProfilePin(${pinLength})">Set PIN</button>
            </div>
        </div>`;
}

async function saveProfile() {
    const body = {
        first_name: document.getElementById('profFirstName').value.trim(),
        last_name: document.getElementById('profLastName').value.trim(),
        email: document.getElementById('profEmail').value.trim()
    };
    if (!body.email) { toast('Email is required', 'error'); return; }
    const d = await api('PUT', '/profile', body);
    if (d.success) {
        toast('Profile updated!');
        // Update local user data
        currentUser = { ...currentUser, ...d.data };
        localStorage.setItem('user', JSON.stringify(currentUser));
    } else toast(d.error || 'Failed to update profile', 'error');
}

async function saveProfilePassword() {
    const pw = document.getElementById('profPassword').value;
    const pwc = document.getElementById('profPasswordConfirm').value;
    if (!pw) { toast('Please enter a new password', 'error'); return; }
    if (pw !== pwc) { toast('Passwords do not match', 'error'); return; }
    if (pw.length < 4) { toast('Password must be at least 4 characters', 'error'); return; }
    const d = await api('PUT', '/profile', { password: pw });
    if (d.success) {
        toast('Password updated!');
        document.getElementById('profPassword').value = '';
        document.getElementById('profPasswordConfirm').value = '';
    } else toast(d.error || 'Failed to update password', 'error');
}

async function saveProfilePin(pinLength) {
    const pin = document.getElementById('profPin').value;
    if (!pin || pin.length !== pinLength) { toast(`PIN must be exactly ${pinLength} digits`, 'error'); return; }
    if (!/^\d+$/.test(pin)) { toast('PIN must contain only digits', 'error'); return; }
    const d = await api('PUT', '/auth/pin', { pin });
    if (d.success) {
        toast('PIN updated!');
        document.getElementById('profPin').value = '';
        loadProfileView(); // Refresh to show updated PIN status
    } else toast(d.error || 'Failed to set PIN', 'error');
}

// ──── User Stats View (P9-01) ────
async function loadStatsView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="section-header"><h2 class="section-title">My Stats</h2></div><div id="statsContainer"><div class="spinner"></div></div>';
    const res = await api('GET', '/profile/stats');
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load stats</div></div>'; return; }
    const s = res.data;
    const genres = (s.top_genres || []).map(g => `<span class="genre-chip">${g.name} <small>(${g.count})</small></span>`).join('');
    const shows = (s.top_shows || []).map(sh => `<li>${sh.title} <small>(${sh.count} episodes)</small></li>`).join('');
    const perfs = (s.top_performers || []).map(p => `<li>${p.name} <small>(${p.count})</small></li>`).join('');
    // Build heatmap (simple text-based)
    const heatmap = s.heatmap || [];
    const dayNames = ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'];
    let heatHTML = '<div class="stats-heatmap">';
    for (let d = 0; d < 7; d++) {
        heatHTML += `<div class="heatmap-row"><span class="heatmap-day">${dayNames[d]}</span>`;
        for (let h = 0; h < 24; h++) {
            const cell = heatmap.find(c => c.day === d && c.hour === h);
            const count = cell ? cell.count : 0;
            const intensity = Math.min(count / 5, 1);
            heatHTML += `<div class="heatmap-cell" style="opacity:${0.15 + intensity * 0.85}" title="${dayNames[d]} ${h}:00 - ${count} plays"></div>`;
        }
        heatHTML += '</div>';
    }
    heatHTML += '</div>';
    document.getElementById('statsContainer').innerHTML = `
        <div class="stats-grid">
            <div class="stat-card"><div class="stat-value">${s.total_watch_hours || 0}</div><div class="stat-label">Hours Watched</div></div>
            <div class="stat-card"><div class="stat-value">${s.items_watched || 0}</div><div class="stat-label">Items Completed</div></div>
            <div class="stat-card"><div class="stat-value">${s.average_rating ? s.average_rating.toFixed(1) : '—'}</div><div class="stat-label">Avg Rating Given</div></div>
        </div>
        <div class="stats-section"><h3>Top Genres</h3><div class="genre-chips">${genres || '<span style="color:#5a6a7f">No data yet</span>'}</div></div>
        <div class="stats-section"><h3>Top Shows</h3><ol class="stats-list">${shows || '<li style="color:#5a6a7f">No data yet</li>'}</ol></div>
        <div class="stats-section"><h3>Top Performers</h3><ol class="stats-list">${perfs || '<li style="color:#5a6a7f">No data yet</li>'}</ol></div>
        <div class="stats-section"><h3>Watch Activity Heatmap</h3>${heatHTML}</div>
        <div style="text-align:center;margin-top:24px;"><button class="btn-secondary" onclick="navigate('wrapped','${new Date().getFullYear()}')">View ${new Date().getFullYear()} Wrapped</button></div>`;
}

// ──── Genre Hub View (P9-04) ────
async function loadGenreHubView(slug) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner" style="margin:60px auto;"></div>';
    const res = await api('GET', '/discover/genre/' + encodeURIComponent(slug));
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">Genre not found</div></div>'; return; }
    const g = res.data;
    const subGenres = (g.sub_genres || []).map(sg => `<span class="genre-chip" onclick="navigate('genre','${sg}')">${sg}</span>`).join('');
    mc.innerHTML = `
        <div class="genre-hub-header"><h1 class="genre-hub-title">${g.name}</h1><span class="genre-hub-count">${g.total} items</span></div>
        ${subGenres ? '<div class="genre-sub-tags">' + subGenres + '</div>' : ''}
        <div class="media-grid">${g.items.map(renderMediaCard).join('')}</div>`;
}

// ──── Decade Hub View (P9-04) ────
async function loadDecadeHubView(year) {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner" style="margin:60px auto;"></div>';
    const res = await api('GET', '/discover/decade/' + encodeURIComponent(year));
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">No items found</div></div>'; return; }
    const d = res.data;
    mc.innerHTML = `
        <div class="genre-hub-header"><h1 class="genre-hub-title">The ${d.decade}</h1><span class="genre-hub-count">${d.total} items</span></div>
        <div class="media-grid">${d.items.map(renderMediaCard).join('')}</div>`;
}

// ──── Year-in-Review / Wrapped (P9-06) ────
async function loadWrappedView(year) {
    if (!year) year = new Date().getFullYear();
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="spinner" style="margin:60px auto;"></div>';
    const res = await api('GET', '/profile/wrapped/' + year);
    if (!res.success) { mc.innerHTML = '<div class="empty-state"><div class="empty-state-title">No data for ' + year + '</div></div>'; return; }
    const w = res.data;
    const topMovies = (w.top_movies || []).map(m => `<div class="wrapped-item">${m.poster_path ? '<img src="'+posterSrc(m.poster_path,'')+'" class="wrapped-poster">' : ''}<span>${m.title}</span></div>`).join('');
    const topShows = (w.top_shows || []).map(s => `<div class="wrapped-item">${s.poster_path ? '<img src="'+posterSrc(s.poster_path,'')+'" class="wrapped-poster">' : ''}<span>${s.title} <small>(${s.episodes} eps)</small></span></div>`).join('');
    mc.innerHTML = `
        <div class="wrapped-container">
            <h1 class="wrapped-title">${year} Wrapped</h1>
            <div class="stats-grid">
                <div class="stat-card wrapped-card"><div class="stat-value">${w.total_hours || 0}</div><div class="stat-label">Hours Watched</div></div>
                <div class="stat-card wrapped-card"><div class="stat-value">${w.items_watched || 0}</div><div class="stat-label">Items Watched</div></div>
                <div class="stat-card wrapped-card"><div class="stat-value">${w.longest_binge_hours || 0}h</div><div class="stat-label">Longest Binge</div></div>
            </div>
            <div class="stats-section"><h3>Top Genres</h3><div class="genre-chips">${(w.top_genres||[]).map(g=>'<span class="genre-chip">'+g+'</span>').join('')}</div></div>
            <div class="stats-section"><h3>Top Movies</h3><div class="wrapped-grid">${topMovies||'<span style="color:#5a6a7f">No movies watched</span>'}</div></div>
            <div class="stats-section"><h3>Top Shows</h3><div class="wrapped-grid">${topShows||'<span style="color:#5a6a7f">No shows watched</span>'}</div></div>
        </div>`;
}

// ──── Content Requests (P9-07) ────
async function loadContentRequestsView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="section-header"><h2 class="section-title">Content Requests</h2><button class="btn-primary btn-sm" onclick="showNewRequestDialog()">+ New Request</button></div><div id="requestsList"><div class="spinner"></div></div>';
    const res = await api('GET', '/requests/mine');
    if (!res.success) { document.getElementById('requestsList').innerHTML = '<div class="empty-state"><div class="empty-state-title">Failed to load</div></div>'; return; }
    const items = res.data;
    if (items.length === 0) {
        document.getElementById('requestsList').innerHTML = '<div class="empty-state"><div class="empty-state-icon">&#128269;</div><div class="empty-state-title">No requests yet</div><p>Request movies or shows you would like added</p></div>';
        return;
    }
    document.getElementById('requestsList').innerHTML = `<div class="requests-grid">${items.map(r => `<div class="request-card">
        <div class="request-poster">${r.poster_url ? '<img src="'+r.poster_url+'">' : '<span style="font-size:2rem;">&#127916;</span>'}</div>
        <div class="request-info">
            <div class="request-title">${r.title} ${r.year ? '('+r.year+')' : ''}</div>
            <div class="request-type">${r.media_type}</div>
            <div class="request-status status-${r.status}">${r.status}</div>
            ${r.admin_note ? '<div class="request-note">'+r.admin_note+'</div>' : ''}
        </div>
    </div>`).join('')}</div>`;
}

async function showNewRequestDialog() {
    const title = prompt('Enter movie or show title:');
    if (!title) return;
    const mediaType = confirm('Is this a TV show? (OK = TV, Cancel = Movie)') ? 'tv' : 'movie';
    const res = await api('POST', '/requests', { title: title, media_type: mediaType });
    if (res.success) { toast('Request submitted!'); loadContentRequestsView(); }
    else toast(res.error || 'Failed to submit request', 'error');
}

// ──────────────────── Artwork Picker ────────────────────

function browseArtworkFromEdit(type) {
    const mediaId = document.getElementById('editMediaId').value;
    if (!mediaId) { toast('No media item loaded', 'error'); return; }
    // Close the edit modal first
    const modal = document.getElementById('editModal');
    if (modal) modal.style.display = 'none';
    openArtworkPicker(mediaId, type);
}

function identifyFromEdit() {
    const mediaId = document.getElementById('editMediaId').value;
    if (!mediaId) { toast('No media item loaded', 'error'); return; }
    closeEditModal();
    identifyMedia(mediaId);
}

async function openArtworkPicker(mediaId, type) {
    const res = await api('GET', `/media/${mediaId}/artwork`);
    if (!res.success || !res.data) {
        toast(res.error || 'No artwork available from cache server', 'error');
        return;
    }
    const urls = type === 'poster' ? (res.data.posters || []) :
                 type === 'backdrop' ? (res.data.backdrops || []) : (res.data.logos || []);
    if (urls.length === 0) {
        toast(`No ${type} images available`, 'error');
        return;
    }
    showArtworkPickerModal(mediaId, type, urls);
}

function showArtworkPickerModal(mediaId, type, urls) {
    let existing = document.getElementById('artworkPickerOverlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'artworkPickerOverlay';
    overlay.className = 'artwork-picker-overlay';

    const label = type.charAt(0).toUpperCase() + type.slice(1);
    let grid = '';
    urls.forEach((url, i) => {
        grid += `<div class="artwork-thumb" data-idx="${i}" onclick="selectArtwork(this,'${mediaId}','${type}','${url.replace(/'/g,"\\'")}')">
            <img src="${url}" loading="lazy" alt="${label} ${i+1}">
            <span class="artwork-source">${extractArtworkSource(url)}</span>
        </div>`;
    });

    overlay.innerHTML = `
        <div class="artwork-picker-modal">
            <div class="artwork-picker-header">
                <h2>Choose ${label} (${urls.length} available)</h2>
                <button class="artwork-picker-close" onclick="this.closest('.artwork-picker-overlay').remove()">&times;</button>
            </div>
            <div class="artwork-picker-grid">${grid}</div>
        </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) overlay.remove();
    });
}

function extractArtworkSource(url) {
    if (url.includes('tmdb.org') || url.includes('themoviedb.org')) return 'TMDB';
    if (url.includes('fanart.tv')) return 'Fanart.tv';
    if (url.includes('thetvdb.com')) return 'TVDB';
    if (url.includes('anilist')) return 'AniList';
    return 'Source';
}

async function selectArtwork(el, mediaId, type, url) {
    document.querySelectorAll('.artwork-thumb.selected').forEach(t => t.classList.remove('selected'));
    el.classList.add('selected');
    const res = await api('PUT', `/media/${mediaId}/artwork`, { type: type, url: url });
    if (res.success) {
        toast(`${type.charAt(0).toUpperCase()+type.slice(1)} updated!`);
        const overlay = document.getElementById('artworkPickerOverlay');
        if (overlay) overlay.remove();
        showMediaDetail(mediaId);
    } else {
        toast(res.error || 'Failed to update artwork', 'error');
    }
}

// ──────────────────── Collection Artwork Picker ────────────────────

async function openCollectionArtworkPicker(collId, type) {
    const res = await api('GET', `/collections/${collId}/artwork`);
    if (!res.success || !res.data) {
        toast(res.error || 'No artwork available from cache server', 'error');
        return;
    }
    const urls = type === 'poster' ? (res.data.posters || []) : (res.data.backdrops || []);
    if (urls.length === 0) {
        toast(`No ${type} images available`, 'error');
        return;
    }
    showCollectionArtworkPickerModal(collId, type, urls);
}

function showCollectionArtworkPickerModal(collId, type, urls) {
    let existing = document.getElementById('artworkPickerOverlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'artworkPickerOverlay';
    overlay.className = 'artwork-picker-overlay';

    const label = type.charAt(0).toUpperCase() + type.slice(1);
    let grid = '';
    urls.forEach((url, i) => {
        grid += `<div class="artwork-thumb" data-idx="${i}" onclick="selectCollectionArtwork(this,'${collId}','${type}','${url.replace(/'/g,"\\'")}')">
            <img src="${url}" loading="lazy" alt="${label} ${i+1}">
            <span class="artwork-source">${extractArtworkSource(url)}</span>
        </div>`;
    });

    overlay.innerHTML = `
        <div class="artwork-picker-modal">
            <div class="artwork-picker-header">
                <h2>Choose Collection ${label} (${urls.length} available)</h2>
                <button class="artwork-picker-close" onclick="this.closest('.artwork-picker-overlay').remove()">&times;</button>
            </div>
            <div class="artwork-picker-grid">${grid}</div>
        </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) overlay.remove();
    });
}

async function selectCollectionArtwork(el, collId, type, url) {
    document.querySelectorAll('.artwork-thumb.selected').forEach(t => t.classList.remove('selected'));
    el.classList.add('selected');
    const res = await api('PUT', `/collections/${collId}/artwork`, { type: type, url: url });
    if (res.success) {
        toast(`Collection ${type.charAt(0).toUpperCase()+type.slice(1)} updated!`);
        const overlay = document.getElementById('artworkPickerOverlay');
        if (overlay) overlay.remove();
        loadCollectionDetailView(collId);
    } else {
        toast(res.error || 'Failed to update collection artwork', 'error');
    }
}