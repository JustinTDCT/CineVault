// TV show views
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

