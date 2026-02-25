import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';

export async function init(container, itemId) {
    clear(container);

    const item = await api.media.get(itemId);
    const meta = typeof item.metadata === 'string' ? JSON.parse(item.metadata || '{}') : (item.metadata || {});

    const wrapper = el('div', { style: { maxWidth: '1200px', margin: '0 auto' } });

    const backBtn = el('button', {
        class: 'btn btn-sm btn-secondary',
        style: { marginBottom: '16px' },
        onClick: () => { window.location.hash = `#/media-detail/${itemId}`; },
    }, '\u2190 Back');
    wrapper.appendChild(backBtn);

    wrapper.appendChild(el('h2', { style: { marginBottom: '12px' } }, item.title || 'Untitled'));

    const video = el('video', {
        id: 'main-player',
        style: { width: '100%', maxHeight: '70vh', background: '#000', borderRadius: 'var(--radius)' },
        controls: 'true',
        autoplay: 'true',
    });

    video.appendChild(el('source', {
        src: `/api/player/stream/${itemId}`,
        type: detectType(item.file_path),
    }));

    wrapper.appendChild(video);

    const controls = el('div', { style: { display: 'flex', gap: '8px', marginTop: '12px' } });
    controls.appendChild(el('button', {
        class: 'btn btn-sm btn-secondary',
        onClick: () => startTranscode(itemId, '720p'),
    }, 'Transcode 720p'));
    controls.appendChild(el('button', {
        class: 'btn btn-sm btn-secondary',
        onClick: () => startTranscode(itemId, '1080p'),
    }, 'Transcode 1080p'));
    wrapper.appendChild(controls);

    container.appendChild(wrapper);
}

function detectType(path) {
    if (!path) return 'video/mp4';
    const ext = path.split('.').pop().toLowerCase();
    const types = { mp4: 'video/mp4', webm: 'video/webm', mkv: 'video/x-matroska', m4v: 'video/mp4' };
    return types[ext] || 'video/mp4';
}

async function startTranscode(itemId, quality) {
    try {
        const data = await api.post(`/player/transcode/${itemId}`, { quality });
        const video = document.getElementById('main-player');
        if (video && data.playlist) {
            video.src = data.playlist;
            video.load();
            video.play();
        }
        import('../components/toast.js').then(t => t.info(`Transcoding to ${quality}...`));
    } catch (e) {
        import('../components/toast.js').then(t => t.error(e.message));
    }
}
