// Media detail view
async function loadMediaDetail(id) {
    _detailReturnNav = { ..._currentNav };
    const mc = document.getElementById('mainContent');
    if (!_detailMediaId) {
        _detailReturnScroll = {
            scrollTop: mc ? mc.scrollTop : 0,
            startOffset: _gridState.startOffset || 0,
            offset: _gridState.offset || 0,
            libraryId: _currentNav.view === 'library' ? _currentNav.extra : null
        };
    }
    _detailMediaId = id;
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
        if (m.rating) ratingsHTML += `<div class="rating-badge rating-tmdb"><span class="rating-icon">${ratingIcon('tmdb', m.rating, 'large')}</span><span class="rating-value">${m.rating.toFixed(1)}</span><span class="rating-label">TMDB</span></div>`;
        if (m.imdb_rating) ratingsHTML += `<div class="rating-badge rating-imdb"><span class="rating-icon">${ratingIcon('imdb', m.imdb_rating, 'large')}</span><span class="rating-value">${m.imdb_rating.toFixed(1)}</span><span class="rating-label">IMDb</span></div>`;
        if (m.rt_rating != null) ratingsHTML += `<div class="rating-badge rating-rt"><span class="rating-icon">${ratingIcon('rt_critic', m.rt_rating, 'large')}</span><span class="rating-value">${m.rt_rating}%</span><span class="rating-label">Rotten Tomatoes</span></div>`;
        if (m.audience_score != null) ratingsHTML += `<div class="rating-badge rating-audience"><span class="rating-icon">${ratingIcon('rt_audience', m.audience_score, 'large')}</span><span class="rating-value">${m.audience_score}%</span><span class="rating-label">Audience</span></div>`;
        if (m.metacritic_score) ratingsHTML += `<div class="rating-badge rating-metacritic"><span class="rating-icon">${ratingIcon('metacritic', m.metacritic_score, 'large')}</span><span class="rating-value">${m.metacritic_score}</span><span class="rating-label">Metacritic</span></div>`;
        ratingsHTML += '</div>';
    }

    // Multi-country content ratings display (filtered by user region if set)
    let countryRatingsHTML = '';
    if (m.content_ratings_json) {
        try {
            const cr = JSON.parse(m.content_ratings_json);
            let pairs = [];
            if (Array.isArray(cr)) {
                const seen = {};
                cr.forEach(item => {
                    const region = item.region || item.country || '';
                    const rating = item.rating || '';
                    if (region && rating && !seen[region]) {
                        seen[region] = true;
                        pairs.push([region, rating]);
                    }
                });
            } else {
                pairs = Object.entries(cr);
            }
            if (_userRegion) {
                pairs = pairs.filter(([country]) => country === _userRegion);
            }
            if (pairs.length > 0) {
                countryRatingsHTML = '<div class="multi-rating-row">';
                pairs.forEach(([country, rating]) => {
                    countryRatingsHTML += `<span class="rating-country-badge"><span class="country-code">${country}</span> ${rating}</span>`;
                });
                countryRatingsHTML += '</div>';
            }
        } catch(e) {}
    }

    const detailTitle = m.sister_group_name || m.title;

    const backdropSrc = m.backdrop_path ? posterSrc(m.backdrop_path, m.updated_at) : '';

    mc.innerHTML = `
        <div class="detail-hero${backdropSrc ? ' has-backdrop' : ''}"${backdropSrc ? ` style="background-image:url('${backdropSrc.replace(/'/g, '%27')}');"` : ''}>
            ${backdropSrc ? '<div class="detail-hero-overlay"></div>' : ''}
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

