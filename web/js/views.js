// ──── Utilities ────
function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

// ──── Navigation ────
let _currentNav = { view: 'home', extra: null };
let _detailReturnNav = null;
let _detailReturnScroll = null;
let _pendingScrollRestore = null;

function navigate(view, extra, scrollRestore) {
    _currentNav = { view, extra: extra || null };
    _detailMediaId = null;
    _pendingScrollRestore = scrollRestore || null;
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
    const scrollRestore = _detailReturnScroll;
    _detailReturnScroll = null;
    if (_detailReturnNav && _detailReturnNav.view) {
        const r = _detailReturnNav;
        if (r.view === '__series') {
            loadSeriesDetail(r.extra);
            return;
        }
        navigate(r.view, r.extra, scrollRestore);
    } else {
        navigate('library', libraryId, scrollRestore);
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
    if (overlayPrefs.hide_theatrical !== false && raw.includes('theatrical')) return '';
    if (EDITION_LABEL_MAP[raw]) return EDITION_LABEL_MAP[raw];
    for (const [key, label] of Object.entries(EDITION_LABEL_MAP)) {
        if (raw.includes(key)) return label;
    }
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

function _overlayGroup(prefs, key) {
    if (!prefs.groups) return null;
    const g = prefs.groups[key];
    return (g && g.enabled) ? g : null;
}

const ZONE_CSS_MAP = {
    'top-left': 'overlay-zone-tl', 'top': 'overlay-zone-t', 'top-right': 'overlay-zone-tr',
    'left': 'overlay-zone-l', 'right': 'overlay-zone-r',
    'bottom-left': 'overlay-zone-bl', 'bottom': 'overlay-zone-b', 'bottom-right': 'overlay-zone-br'
};

function _buildRatingBadges(item) {
    let badges = '';
    if (item.imdb_rating)
        badges += `<span class="overlay-badge overlay-badge-rating">${ratingIcon('imdb', item.imdb_rating)} ${item.imdb_rating.toFixed(1)}</span>`;
    if (item.rt_rating != null)
        badges += `<span class="overlay-badge overlay-badge-rating">${ratingIcon('rt_critic', item.rt_rating)} ${item.rt_rating}%</span>`;
    if (item.audience_score != null)
        badges += `<span class="overlay-badge overlay-badge-rating">${ratingIcon('rt_audience', item.audience_score)} ${item.audience_score}%</span>`;
    if (item.rating)
        badges += `<span class="overlay-badge overlay-badge-rating">${ratingIcon('tmdb', item.rating)} ${item.rating.toFixed(1)}</span>`;
    if (item.metacritic_score)
        badges += `<span class="overlay-badge overlay-badge-rating">${ratingIcon('metacritic', item.metacritic_score)}</span>`;
    return badges;
}

function _buildResAudioBadges(item, prefs) {
    let badges = '';
    const resEnabled = prefs.resolution_hdr !== undefined ? prefs.resolution_hdr : true;
    const audioEnabled = prefs.audio_codec !== undefined ? prefs.audio_codec : true;
    if (resEnabled) {
        const resLabel = buildResolutionLabel(item);
        if (resLabel) {
            const isHDR = resLabel.includes('HDR') || resLabel.includes('DV') || resLabel.includes('HLG');
            badges += `<span class="overlay-badge ${isHDR ? 'overlay-badge-res-hdr' : 'overlay-badge-res'}">${resLabel}</span>`;
        }
    }
    if (audioEnabled) {
        const audioLabel = mapAudioLabel(item);
        if (audioLabel) badges += `<span class="overlay-badge overlay-badge-audio">${audioLabel}</span>`;
    }
    return badges;
}

function _buildEditionBadges(item) {
    let badges = '';
    if (item.edition_count && item.edition_count > 1) {
        badges += `<span class="overlay-badge overlay-badge-edition">${item.edition_count} Editions</span>`;
    } else {
        const edLabel = mapEditionLabel(item);
        if (edLabel) badges += `<span class="overlay-badge overlay-badge-edition">${edLabel}</span>`;
    }
    return badges;
}

function _buildContentRatingBadge(item) {
    if (!item.content_rating) return '';
    return `<span class="overlay-badge overlay-badge-content-rating">${item.content_rating}</span>`;
}

function _buildSourceTypeBadge(item) {
    const srcLabel = mapSourceLabel(item);
    if (!srcLabel) return '';
    return `<span class="overlay-badge overlay-badge-source">${srcLabel}</span>`;
}

function renderOverlayBadges(item) {
    const p = overlayPrefs;
    const isMusic = item.media_type === 'music';

    // Music: only show audio codec in top-right
    if (isMusic) {
        const audioLabel = mapAudioLabel(item);
        if (!audioLabel) return '';
        const g = _overlayGroup(p, 'resolution_audio');
        const pos = g ? g.position : 'top-right';
        const cls = ZONE_CSS_MAP[pos] || 'overlay-zone-tr';
        return `<div class="${cls}"><span class="overlay-badge overlay-badge-audio">${audioLabel}</span></div>`;
    }

    // New group-based system
    if (p.groups) {
        const zones = {};
        const addToZone = (pos, html) => { if (html) { zones[pos] = (zones[pos] || '') + html; } };

        const raG = _overlayGroup(p, 'resolution_audio');
        if (raG) addToZone(raG.position, _buildResAudioBadges(item, p));

        const edG = _overlayGroup(p, 'edition');
        if (edG) addToZone(edG.position, _buildEditionBadges(item));

        const rtG = _overlayGroup(p, 'ratings');
        if (rtG) addToZone(rtG.position, _buildRatingBadges(item));

        const crG = _overlayGroup(p, 'content_rating');
        if (crG) addToZone(crG.position, _buildContentRatingBadge(item));

        const stG = _overlayGroup(p, 'source_type');
        if (stG) addToZone(stG.position, _buildSourceTypeBadge(item));

        // Multi-part always goes with edition group or bottom-right fallback
        if (item.sister_part_count > 1) {
            const mpPos = edG ? edG.position : 'bottom-right';
            addToZone(mpPos, `<span class="overlay-badge overlay-badge-multipart">${item.sister_part_count} Parts</span>`);
        }

        let html = '';
        for (const [pos, content] of Object.entries(zones)) {
            const cls = ZONE_CSS_MAP[pos] || 'overlay-zone-tr';
            html += `<div class="${cls}">${content}</div>`;
        }
        return html;
    }

    // Legacy boolean fallback
    let trBadges = '', tlBadges = '', blBadges = '', brBadges = '';
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
    if (p.ratings) {
        blBadges += _buildRatingBadges(item);
    }
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
        ratingBadge = item.rating ? `<span class="hover-rating-badge">${ratingIcon('tmdb', item.rating)} ${item.rating.toFixed(1)}</span>` : '';
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
