import { el } from '../utils/dom.js';

export function mediaCard(item, opts = {}) {
    const card = el('div', { class: 'media-card', 'data-id': item.id });

    const posterUrl = getPosterUrl(item);
    const img = el('img', {
        class: 'media-poster',
        src: posterUrl,
        alt: item.title || 'Untitled',
        loading: 'lazy',
    });
    img.onerror = () => { img.src = placeholderPoster(); };

    card.appendChild(img);

    if (opts.overlays) {
        for (const overlay of opts.overlays) {
            card.appendChild(el('span', {
                class: `overlay-badge ${overlay.position}`,
            }, overlay.text));
        }
    }

    const info = el('div', { class: 'media-card-info' },
        el('div', { class: 'media-card-title' }, item.title || 'Untitled'),
        el('div', { class: 'media-card-year' }, item.release_year ? String(item.release_year) : '')
    );
    card.appendChild(info);

    if (opts.onClick) {
        card.addEventListener('click', () => opts.onClick(item));
    }

    return card;
}

function getPosterUrl(item) {
    const meta = typeof item.metadata === 'string' ? JSON.parse(item.metadata) : item.metadata;
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
