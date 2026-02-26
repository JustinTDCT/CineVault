import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as fmt from '../utils/format.js';

function parseMeta(item) {
    if (!item.metadata) return {};
    return typeof item.metadata === 'string' ? JSON.parse(item.metadata) : item.metadata;
}

function ratingIconSvg(type, value) {
    if (type === 'tmdb') {
        const color = value >= 7 ? '#01d277' : value >= 5 ? '#d2d531' : '#db2360';
        return `<svg width="18" height="18" viewBox="0 0 18 18"><circle cx="9" cy="9" r="8" fill="none" stroke="${color}" stroke-width="2" opacity="0.3"/><circle cx="9" cy="9" r="8" fill="none" stroke="${color}" stroke-width="2" stroke-dasharray="${(value/10)*50.3} 50.3" transform="rotate(-90 9 9)"/><text x="9" y="12" text-anchor="middle" fill="${color}" font-size="7" font-weight="700">${value.toFixed(1)}</text></svg>`;
    }
    if (type === 'imdb') {
        return `<svg width="32" height="16" viewBox="0 0 32 16"><rect width="32" height="16" rx="3" fill="#f5c518"/><text x="16" y="12" text-anchor="middle" fill="#000" font-size="8" font-weight="800">IMDb</text></svg>`;
    }
    if (type === 'rt') {
        const fresh = value >= 60;
        return fresh ? '🍅' : '🥫';
    }
    if (type === 'metacritic') {
        const color = value >= 61 ? '#6c3' : value >= 40 ? '#fc3' : '#f00';
        return `<svg width="18" height="18" viewBox="0 0 18 18"><rect width="18" height="18" rx="3" fill="${color}"/><text x="9" y="13" text-anchor="middle" fill="#fff" font-size="9" font-weight="700">${value}</text></svg>`;
    }
    return '';
}

function buildRatingsRow(meta) {
    const parts = [];
    if (meta.tmdb_rating) {
        parts.push(`<div class="rating-badge rating-tmdb">
            <span class="rating-icon">${ratingIconSvg('tmdb', meta.tmdb_rating)}</span>
            <span class="rating-value">${meta.tmdb_rating.toFixed(1)}</span>
            <span class="rating-label">TMDB</span></div>`);
    }
    if (meta.imdb_rating) {
        parts.push(`<div class="rating-badge rating-imdb">
            <span class="rating-icon">${ratingIconSvg('imdb', meta.imdb_rating)}</span>
            <span class="rating-value">${meta.imdb_rating.toFixed(1)}</span>
            <span class="rating-label">IMDb</span></div>`);
    }
    if (meta.rotten_tomatoes_score != null) {
        parts.push(`<div class="rating-badge rating-rt">
            <span class="rating-icon">${ratingIconSvg('rt', meta.rotten_tomatoes_score)}</span>
            <span class="rating-value">${meta.rotten_tomatoes_score}%</span>
            <span class="rating-label">Rotten Tomatoes</span></div>`);
    }
    if (meta.metacritic_score) {
        parts.push(`<div class="rating-badge rating-metacritic">
            <span class="rating-icon">${ratingIconSvg('metacritic', meta.metacritic_score)}</span>
            <span class="rating-value">${meta.metacritic_score}</span>
            <span class="rating-label">Metacritic</span></div>`);
    }
    return parts.length ? `<div class="ratings-row">${parts.join('')}</div>` : '';
}

function buildContentRatings(meta) {
    const ratings = meta.content_ratings;
    if (!ratings || !Array.isArray(ratings) || ratings.length === 0) return '';
    let html = '<div class="multi-rating-row">';
    ratings.forEach(cr => {
        html += `<span class="rating-country-badge"><span class="country-code">${cr.region}</span> ${cr.rating}</span>`;
    });
    html += '</div>';
    return html;
}

function buildGenreTags(meta) {
    const genres = meta.genres;
    if (!genres || !Array.isArray(genres) || genres.length === 0) return '';
    return '<div class="genre-tags">' +
        genres.map(g => `<span class="genre-tag">${g}</span>`).join('') +
        '</div>';
}

function buildStarRating(currentRating) {
    let html = '<div class="user-star-rating">';
    for (let i = 1; i <= 10; i++) {
        html += `<span class="star${currentRating && i <= currentRating ? ' filled' : ''}" data-rating="${i}">★</span>`;
    }
    html += '</div>';
    return html;
}

function buildTagBadges(item, meta) {
    const tags = [];
    if (item.metadata_locked) {
        tags.push('<span class="lock-badge">🔒 Metadata Locked</span>');
    }
    tags.push('<span class="tag tag-cyan">Movies</span>');
    if (meta.edition_type && meta.edition_type !== 'Theatrical' && meta.edition_type !== 'Standard' && meta.edition_type !== 'unknown') {
        tags.push('<span class="tag tag-purple">Multiple Editions</span>');
    }
    if (item.file_size) {
        tags.push(`<span class="tag tag-purple">${Math.round(item.file_size / 1024 / 1024)} MB</span>`);
    }
    if (item.audio_codec) {
        tags.push(`<span class="tag tag-green">${item.audio_codec}</span>`);
    }
    if (item.bitrate) {
        tags.push(`<span class="tag tag-orange">${Math.round(item.bitrate / 1000)} kbps</span>`);
    }
    return tags.length ? `<div class="detail-tag-row">${tags.join('')}</div>` : '';
}

function getFileName(filePath) {
    if (!filePath) return '';
    return filePath.split('/').pop();
}

function showTab(tabName, item, meta, contentEl) {
    const tabs = contentEl.closest('.detail-info').querySelectorAll('.detail-tab');
    tabs.forEach(t => t.classList.toggle('active', t.dataset.tab === tabName));

    let html = '';
    switch (tabName) {
        case 'info':
            html = '<table>';
            html += `<tr><td>File</td><td>${getFileName(item.file_path)}</td></tr>`;
            html += `<tr><td>Path</td><td>${item.file_path || ''}</td></tr>`;
            if (item.resolution) html += `<tr><td>Resolution</td><td>${item.resolution}</td></tr>`;
            if (item.video_codec) html += `<tr><td>Video Codec</td><td>${item.video_codec}</td></tr>`;
            if (item.audio_codec) html += `<tr><td>Audio Codec</td><td>${item.audio_codec}</td></tr>`;
            if (item.bitrate) html += `<tr><td>Bitrate</td><td>${Math.round(item.bitrate / 1000)} kbps</td></tr>`;
            if (item.file_size) html += `<tr><td>File Size</td><td>${fmt.fileSize(item.file_size)}</td></tr>`;
            if (item.match_confidence) html += `<tr><td>Match</td><td>${Math.round(item.match_confidence * 100)}%</td></tr>`;
            html += `<tr><td>Added</td><td>${new Date(item.date_added).toLocaleString()}</td></tr>`;
            html += '</table>';
            break;
        case 'cast':
            if (meta.cast && meta.cast.length > 0) {
                html = '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(160px,1fr));gap:12px;">';
                meta.cast.forEach(p => {
                    html += `<div style="display:flex;align-items:center;gap:10px;padding:8px;border-radius:var(--radius-lg);background:rgba(255,255,255,0.03);">`;
                    if (p.photo_url) {
                        html += `<img src="${p.photo_url}" style="width:40px;height:40px;border-radius:var(--radius-full);object-fit:cover;">`;
                    } else {
                        html += `<div style="width:40px;height:40px;border-radius:var(--radius-full);background:rgba(255,255,255,0.06);display:flex;align-items:center;justify-content:center;font-size:1rem;">👤</div>`;
                    }
                    html += `<div><div style="font-size:0.82rem;font-weight:600;">${p.name}</div>`;
                    if (p.character) html += `<div style="font-size:0.72rem;color:var(--text-muted);">${p.character}</div>`;
                    html += '</div></div>';
                });
                html += '</div>';
            } else {
                html = '<div style="color:var(--text-muted);padding:20px;">No cast information available.</div>';
            }
            break;
        case 'metadata':
            html = '<table>';
            if (meta.tagline) html += `<tr><td>Tagline</td><td>${meta.tagline}</td></tr>`;
            if (meta.status) html += `<tr><td>Status</td><td>${meta.status}</td></tr>`;
            if (meta.original_language) html += `<tr><td>Language</td><td>${meta.original_language}</td></tr>`;
            if (meta.production_countries && meta.production_countries.length) html += `<tr><td>Countries</td><td>${meta.production_countries.join(', ')}</td></tr>`;
            if (meta.budget) html += `<tr><td>Budget</td><td>$${meta.budget.toLocaleString()}</td></tr>`;
            if (meta.revenue) html += `<tr><td>Revenue</td><td>$${meta.revenue.toLocaleString()}</td></tr>`;
            if (meta.box_office_us) html += `<tr><td>Box Office (US)</td><td>${meta.box_office_us}</td></tr>`;
            if (meta.awards_text) html += `<tr><td>Awards</td><td>${meta.awards_text}</td></tr>`;
            if (meta.collection_name) html += `<tr><td>Collection</td><td>${meta.collection_name}</td></tr>`;
            if (item.match_confidence) html += `<tr><td>Match Confidence</td><td>${Math.round(item.match_confidence * 100)}%</td></tr>`;
            html += `<tr><td>Date Modified</td><td>${new Date(item.date_modified).toLocaleString()}</td></tr>`;
            html += '</table>';
            break;
        case 'file':
            html = '<table>';
            html += `<tr><td>File Name</td><td>${getFileName(item.file_path)}</td></tr>`;
            html += `<tr><td>Full Path</td><td>${item.file_path || ''}</td></tr>`;
            if (item.file_size) html += `<tr><td>File Size</td><td>${fmt.fileSize(item.file_size)}</td></tr>`;
            if (item.video_codec) html += `<tr><td>Video Codec</td><td>${item.video_codec}</td></tr>`;
            if (item.audio_codec) html += `<tr><td>Audio Codec</td><td>${item.audio_codec}</td></tr>`;
            if (item.resolution) html += `<tr><td>Resolution</td><td>${item.resolution}</td></tr>`;
            if (item.bitrate) html += `<tr><td>Bitrate</td><td>${Math.round(item.bitrate / 1000)} kbps</td></tr>`;
            if (item.file_hash) html += `<tr><td>File Hash</td><td style="font-family:monospace;font-size:0.75rem;">${item.file_hash}</td></tr>`;
            html += `<tr><td>Date Added</td><td>${new Date(item.date_added).toLocaleString()}</td></tr>`;
            html += '</table>';
            break;
        default:
            html = `<div style="color:var(--text-muted);padding:20px;">No ${tabName} data available.</div>`;
    }
    contentEl.innerHTML = html;
}

export async function init(container, itemId) {
    clear(container);
    container.appendChild(el('div', { class: 'loading-center' }, el('div', { class: 'spinner' })));

    const item = await api.media.get(itemId);
    const meta = parseMeta(item);

    clear(container);

    const posterUrl = meta.poster_url || (meta.poster_path ? `https://image.tmdb.org/t/p/w500${meta.poster_path}` : '');
    const backdropUrl = meta.backdrop_url || (meta.backdrop_path ? `https://image.tmdb.org/t/p/w1280${meta.backdrop_path}` : '');

    const title = item.title || 'Untitled';
    const dur = item.runtime_minutes ? fmt.duration(item.runtime_minutes) : '';
    const metaLine = [item.release_year, dur, item.resolution, item.video_codec].filter(Boolean).join(' · ');

    const editionBadge = (meta.edition_type && meta.edition_type !== 'Theatrical' && meta.edition_type !== 'Standard' && meta.edition_type !== '' && meta.edition_type !== 'unknown')
        ? `<span class="edition-appendix-badge">${meta.edition_type} Edition</span>` : '';

    const editionDesc = meta.edition_description
        ? `<div class="edition-description"><strong>New content:</strong> ${meta.edition_description}</div>` : '';

    const trailerBtn = (meta.trailers && meta.trailers.length > 0)
        ? `<button class="btn btn-secondary" onclick="window.open('https://www.youtube.com/watch?v=${meta.trailers[0].key}','_blank')">🎬 Trailer</button>` : '';

    const heroHtml = `
        <div class="detail-hero${backdropUrl ? ' has-backdrop' : ''}"${backdropUrl ? ` style="background-image:url('${backdropUrl.replace(/'/g, '%27')}')"` : ''}>
            ${backdropUrl ? '<div class="detail-hero-overlay"></div>' : ''}
            <div class="detail-poster">
                ${posterUrl ? `<img src="${posterUrl}" alt="${title}">` : '<div style="font-size:4rem;">🎬</div>'}
            </div>
            <div class="detail-info">
                <h1>${title}</h1>
                ${editionBadge}
                <div class="meta-row">${metaLine}</div>
                ${item.description || meta.overview ? `<p class="description">${item.description || meta.overview}</p>` : ''}
                ${editionDesc}
                ${buildGenreTags(meta)}
                ${buildRatingsRow(meta)}
                ${buildContentRatings(meta)}
                <div class="detail-actions">
                    <button class="btn btn-primary" onclick="window.location.hash='#/player/${item.id}'">▶ Play</button>
                    ${trailerBtn}
                    <button class="btn btn-secondary">👥 Watch Together</button>
                    <button class="btn btn-secondary">💎 Watchlist</button>
                    <button class="btn btn-secondary">❤ Favorite</button>
                    <button class="btn btn-secondary">▶▶ Queue</button>
                    <button class="btn btn-secondary">🎵 Music Play</button>
                    <button class="btn btn-secondary">✎ Edit</button>
                    <button class="btn btn-secondary">📚 + Collection</button>
                </div>
                ${buildStarRating(null)}
                ${buildTagBadges(item, meta)}
                <div class="detail-tabs">
                    <button class="detail-tab active" data-tab="info">Info</button>
                    <button class="detail-tab" data-tab="cast">Cast</button>
                    <button class="detail-tab" data-tab="tags">Tags</button>
                    <button class="detail-tab" data-tab="editions">Editions</button>
                    <button class="detail-tab" data-tab="metadata">Metadata</button>
                    <button class="detail-tab" data-tab="chapters">Chapters</button>
                    <button class="detail-tab" data-tab="extras">Extras</button>
                    <button class="detail-tab" data-tab="segments">Segments</button>
                    <button class="detail-tab" data-tab="file">File</button>
                </div>
                <div class="detail-tab-content" id="detailTabContent"></div>
            </div>
        </div>
        <button class="btn btn-secondary detail-back-btn" onclick="history.back()">← Back</button>`;

    container.innerHTML = heroHtml;

    const tabContent = container.querySelector('#detailTabContent');
    showTab('info', item, meta, tabContent);

    container.querySelectorAll('.detail-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab, item, meta, tabContent);
        });
    });

    if (item.parent_id === null || item.parent_id === undefined) {
        try {
            const children = await api.media.children(item.id);
            if (children && children.length > 0) {
                const episodeSection = el('div', { style: { marginTop: '20px' } });
                episodeSection.appendChild(el('h3', { style: { marginBottom: '12px' } }, 'Episodes'));
                const list = el('div');
                children.forEach(ep => {
                    const epEl = el('div', {
                        class: 'card',
                        style: { marginBottom: '8px', cursor: 'pointer', padding: '12px' },
                        onClick: () => { window.location.hash = `#/media-detail/${ep.id}`; },
                    },
                        el('span', { style: { fontWeight: '600' } },
                            `S${ep.season_number || 0}E${ep.episode_number || 0}`),
                        el('span', { style: { marginLeft: '12px' } }, ep.title || '')
                    );
                    list.appendChild(epEl);
                });
                episodeSection.appendChild(list);
                container.insertBefore(episodeSection, container.querySelector('.detail-back-btn'));
            }
        } catch { /* no children */ }
    }
}
