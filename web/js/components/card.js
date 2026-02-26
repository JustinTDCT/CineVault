import { el } from '../utils/dom.js';
import * as fmt from '../utils/format.js';

export function mediaCard(item, opts = {}) {
    const card = el('div', { class: 'media-card', 'data-id': item.id, tabindex: '0' });

    const meta = typeof item.metadata === 'string' ? JSON.parse(item.metadata || '{}') : (item.metadata || {});
    const posterUrl = getPosterUrl(item, meta);

    const posterWrap = el('div', { class: 'media-poster-wrap' });

    const img = el('img', {
        src: posterUrl,
        alt: item.title || 'Untitled',
        loading: 'lazy',
    });
    img.onerror = () => { img.src = placeholderPoster(); };
    posterWrap.appendChild(img);

    if (opts.overlays) {
        for (const overlay of opts.overlays) {
            posterWrap.appendChild(el('span', {
                class: `overlay-badge ${overlay.position}`,
            }, overlay.text));
        }
    }

    const hoverInfo = el('div', { class: 'media-card-hover-info' });
    hoverInfo.appendChild(el('div', { class: 'hover-title' }, item.title || 'Untitled'));
    const hoverMeta = [
        item.release_year,
        item.resolution,
        item.video_codec,
    ].filter(Boolean).join(' · ');
    const hoverMetaEl = el('div', { class: 'hover-meta' });
    if (meta.tmdb_rating) {
        hoverMetaEl.appendChild(el('span', {
            class: 'hover-rating-badge',
            style: { color: '#01d277', fontWeight: '700', fontSize: '0.72rem' },
        }, `⭐ ${meta.tmdb_rating.toFixed(1)}`));
    }
    hoverMetaEl.appendChild(el('span', {}, hoverMeta));
    hoverInfo.appendChild(hoverMetaEl);
    posterWrap.appendChild(hoverInfo);

    const playOverlay = el('div', { class: 'play-overlay' });
    const playBtn = el('div', { class: 'play-button' });
    playBtn.innerHTML = '&#9654;';
    playBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        window.location.hash = `#/player/${item.id}`;
    });
    playOverlay.appendChild(playBtn);
    posterWrap.appendChild(playOverlay);

    card.appendChild(posterWrap);

    const dur = item.runtime_minutes ? fmt.duration(item.runtime_minutes) : '';
    const cardMeta = [item.release_year, dur, item.resolution].filter(Boolean).join(' · ');

    const info = el('div', { class: 'media-card-info' },
        el('div', { class: 'media-card-title' }, item.title || 'Untitled'),
        el('div', { class: 'media-card-meta' }, cardMeta)
    );
    card.appendChild(info);

    if (opts.onClick) {
        card.addEventListener('click', () => opts.onClick(item));
    }

    return card;
}

function getPosterUrl(item, meta) {
    if (meta?.poster_url) return meta.poster_url;
    if (meta?.poster_path) return `https://image.tmdb.org/t/p/w300${meta.poster_path}`;
    return placeholderPoster();
}

function placeholderPoster() {
    return 'data:image/svg+xml,' + encodeURIComponent(
        '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="300" viewBox="0 0 200 300">' +
        '<rect width="200" height="300" fill="#1a1a2e"/>' +
        '<text x="100" y="150" text-anchor="middle" font-family="sans-serif" font-size="14" fill="#666680">No Poster</text>' +
        '</svg>'
    );
}
