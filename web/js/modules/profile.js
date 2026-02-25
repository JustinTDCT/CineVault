import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as toast from '../components/toast.js';
import { toggle } from '../components/slider-toggle.js';
import { select } from '../components/dropdown.js';

export async function init(container) {
    clear(container);
    container.appendChild(el('div', { class: 'page-title' }, 'My Profile'));

    try {
        const profile = await api.users.profile();
        const user = await api.users.me();

        const infoCard = el('div', { class: 'card', style: { marginBottom: '16px' } });
        infoCard.appendChild(el('h3', { style: { marginBottom: '12px' } }, 'Account'));
        infoCard.appendChild(infoRow('Name', user.full_name));
        infoCard.appendChild(infoRow('Email', user.email));
        infoCard.appendChild(infoRow('Account Type', user.account_type));
        container.appendChild(infoCard);

        const playCard = el('div', { class: 'card', style: { marginBottom: '16px' } });
        playCard.appendChild(el('h3', { style: { marginBottom: '12px' } }, 'Playback'));
        playCard.appendChild(el('div', { class: 'form-group' },
            el('label', { class: 'form-label' }, 'Default Video Quality'),
            select(
                [{ value: 'original', label: 'Direct Play' },
                 { value: '1080p', label: '1080p' }, { value: '720p', label: '720p' },
                 { value: '480p', label: '480p' }],
                profile.default_video_quality || 'original',
                (v) => updateProfile({ default_video_quality: v })
            )
        ));
        playCard.appendChild(toggle('Auto-play Music', profile.auto_play_music, (v) => updateProfile({ auto_play_music: v })));
        playCard.appendChild(toggle('Auto-play Videos', profile.auto_play_videos, (v) => updateProfile({ auto_play_videos: v })));
        playCard.appendChild(toggle('Auto-play Music Videos', profile.auto_play_music_videos, (v) => updateProfile({ auto_play_music_videos: v })));
        playCard.appendChild(toggle('Auto-play Audiobooks', profile.auto_play_audiobooks, (v) => updateProfile({ auto_play_audiobooks: v })));
        container.appendChild(playCard);

        const overlayCard = el('div', { class: 'card' });
        overlayCard.appendChild(el('h3', { style: { marginBottom: '12px' } }, 'Overlay Badges'));

        const overlays = typeof profile.overlay_settings === 'string'
            ? JSON.parse(profile.overlay_settings) : (profile.overlay_settings || {});

        const positions = ['top_left', 'top', 'top_right', 'left', 'right', 'bottom_left', 'bottom', 'bottom_right'];
        const posOptions = positions.map(p => ({ value: p, label: p.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase()) }));

        ['resolution_audio', 'edition', 'ratings', 'content_rating', 'source_type'].forEach(key => {
            const setting = overlays[key] || { enabled: false, position: 'top_left' };
            const label = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
            overlayCard.appendChild(el('div', { style: { display: 'flex', alignItems: 'center', gap: '12px', padding: '6px 0' } },
                toggle(label, setting.enabled, () => {}),
                select(posOptions, setting.position, () => {})
            ));
        });
        container.appendChild(overlayCard);

    } catch (e) {
        container.appendChild(el('div', { class: 'empty-state' },
            el('div', { class: 'empty-state-text' }, `Error: ${e.message}`)));
    }
}

function infoRow(label, value) {
    return el('div', { style: { display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--border)' } },
        el('span', { style: { color: 'var(--text-muted)' } }, label),
        el('span', {}, value || ''));
}

async function updateProfile(fields) {
    try {
        await api.users.updateProfile(fields);
    } catch (e) {
        toast.error(e.message);
    }
}
