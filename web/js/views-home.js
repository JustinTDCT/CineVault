// Home view
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
                    ${heroItem.rating ? '<span class="hero-rating">'+ratingIcon('tmdb', heroItem.rating)+' '+heroItem.rating.toFixed(1)+'</span>' : ''}
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
                    <div class="media-info"><div class="media-title">${item.title}</div><div class="media-meta">${item.year||''} ${item.rating?ratingIcon('tmdb',item.rating)+' '+item.rating.toFixed(1):''}</div></div>
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

