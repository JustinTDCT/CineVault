import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as fmt from '../utils/format.js';

export async function init(container, itemId) {
    clear(container);

    const item = await api.media.get(itemId);
    const meta = typeof item.metadata === 'string' ? JSON.parse(item.metadata || '{}') : (item.metadata || {});

    const page = el('div', { style: { display: 'flex', gap: '24px', flexWrap: 'wrap' } });

    const posterUrl = meta.poster_url || meta.poster_path
        ? `https://image.tmdb.org/t/p/w500${meta.poster_path}`
        : null;

    const sidebar = el('div', { style: { flexShrink: '0', width: '250px' } });
    if (posterUrl) {
        sidebar.appendChild(el('img', {
            src: posterUrl,
            style: { width: '100%', borderRadius: 'var(--radius)', marginBottom: '16px' },
        }));
    }
    sidebar.appendChild(el('button', {
        class: 'btn btn-primary',
        style: { width: '100%' },
        onClick: () => { window.location.hash = `#/player/${item.id}`; },
    }, 'Play'));
    page.appendChild(sidebar);

    const details = el('div', { style: { flex: '1', minWidth: '300px' } });
    details.appendChild(el('h1', { style: { marginBottom: '4px' } }, item.title || 'Untitled'));

    const tagLine = [];
    if (item.release_year) tagLine.push(String(item.release_year));
    if (item.runtime_minutes) tagLine.push(fmt.duration(item.runtime_minutes));
    if (item.resolution) tagLine.push(item.resolution);
    if (tagLine.length) {
        details.appendChild(el('div', { style: { color: 'var(--text-muted)', marginBottom: '16px', fontSize: '14px' } },
            tagLine.join(' \u2022 ')));
    }

    if (item.description || meta.overview) {
        details.appendChild(el('p', { style: { lineHeight: '1.6', marginBottom: '16px' } },
            item.description || meta.overview));
    }

    const infoGrid = el('div', { class: 'card', style: { marginBottom: '16px' } });
    const rows = [
        ['File', item.file_path],
        ['Size', fmt.fileSize(item.file_size)],
        ['Video', item.video_codec],
        ['Audio', item.audio_codec],
        ['Bitrate', item.bitrate ? `${Math.round(item.bitrate / 1000)} kbps` : ''],
        ['Match', item.match_confidence ? `${Math.round(item.match_confidence * 100)}%` : ''],
    ];
    rows.forEach(([label, value]) => {
        if (value) {
            infoGrid.appendChild(el('div', { style: { display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--border)' } },
                el('span', { style: { color: 'var(--text-muted)', fontSize: '13px' } }, label),
                el('span', { style: { fontSize: '13px' } }, String(value))
            ));
        }
    });
    details.appendChild(infoGrid);

    if (item.parent_id === null || item.parent_id === undefined) {
        try {
            const children = await api.media.children(item.id);
            if (children && children.length > 0) {
                details.appendChild(el('h3', { style: { marginBottom: '12px' } }, 'Episodes'));
                const list = el('div', {});
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
                details.appendChild(list);
            }
        } catch { /* no children */ }
    }

    page.appendChild(details);
    container.appendChild(page);
}
