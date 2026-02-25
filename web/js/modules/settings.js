import { el, clear } from '../utils/dom.js';
import * as api from '../services/api.js';
import * as toast from '../components/toast.js';
import { toggle } from '../components/slider-toggle.js';
import { select } from '../components/dropdown.js';
import { tabs } from '../components/tabs.js';

export async function init(container) {
    clear(container);
    container.appendChild(el('div', { class: 'page-title' }, 'Settings'));

    let current = {};
    try {
        current = await api.settings.list();
    } catch { /* defaults */ }

    tabs([
        { label: 'General', render: (body) => renderGeneral(body, current) },
        { label: 'Video', render: (body) => renderVideo(body, current) },
        { label: 'Metadata', render: (body) => renderMetadata(body, current) },
        { label: 'Security', render: (body) => renderSecurity(body, current) },
    ], container);
}

function renderGeneral(body, s) {
    const card = el('div', { class: 'card' });
    card.appendChild(el('h3', { style: { marginBottom: '16px' } }, 'General'));
    card.appendChild(formRow('Region', select(
        [{ value: '', label: 'Select...' }, { value: 'US', label: 'United States' },
         { value: 'UK', label: 'United Kingdom' }, { value: 'CA', label: 'Canada' },
         { value: 'AU', label: 'Australia' }, { value: 'DE', label: 'Germany' },
         { value: 'FR', label: 'France' }, { value: 'JP', label: 'Japan' }],
        s.region || '', (v) => save('region', v)
    )));
    card.appendChild(toggle('Enable Duplicates', s.duplicates_enabled === 'true', (v) => save('duplicates_enabled', String(v))));
    card.appendChild(toggle('Skip App Intro', s.skip_app_intro === 'true', (v) => save('skip_app_intro', String(v))));
    card.appendChild(toggle('Silent App Intro', s.silent_app_intro === 'true', (v) => save('silent_app_intro', String(v))));
    body.appendChild(card);
}

function renderVideo(body, s) {
    const card = el('div', { class: 'card' });
    card.appendChild(el('h3', { style: { marginBottom: '16px' } }, 'Video'));
    card.appendChild(formRow('Default Quality', select(
        [{ value: 'original', label: 'Original (Direct Play)' },
         { value: '1080p', label: '1080p' }, { value: '720p', label: '720p' },
         { value: '480p', label: '480p' }],
        s.default_video_quality || 'original', (v) => save('default_video_quality', v)
    )));
    card.appendChild(toggle('Auto-skip Intro', s.auto_skip_intro === 'true', (v) => save('auto_skip_intro', String(v))));
    card.appendChild(toggle('Auto-skip Credits', s.auto_skip_credits === 'true', (v) => save('auto_skip_credits', String(v))));
    card.appendChild(toggle('Auto-skip Recaps', s.auto_skip_recaps === 'true', (v) => save('auto_skip_recaps', String(v))));

    card.appendChild(el('h4', { style: { marginTop: '16px', marginBottom: '8px' } }, 'Transcoder'));
    card.appendChild(formRow('Type', select(
        [{ value: 'cpu', label: 'CPU / Software' },
         { value: 'qsv', label: 'Intel QSV' },
         { value: 'cuda', label: 'NVIDIA CUDA' }],
        s.transcoder_type || 'cpu', (v) => save('transcoder_type', v)
    )));
    card.appendChild(formRow('Max Simultaneous', el('input', {
        class: 'form-input', type: 'number', value: s.max_transcodes || '2',
        style: { width: '80px' },
        onChange: (e) => save('max_transcodes', e.target.value),
    })));
    body.appendChild(card);
}

function renderMetadata(body, s) {
    const card = el('div', { class: 'card' });
    card.appendChild(el('h3', { style: { marginBottom: '16px' } }, 'Metadata'));
    card.appendChild(toggle('Use Cache Server', s.cache_server_enabled === 'true', (v) => save('cache_server_enabled', String(v))));
    card.appendChild(formRow('Cache Server URL', el('input', {
        class: 'form-input', value: s.cache_server_url || '',
        onChange: (e) => save('cache_server_url', e.target.value),
    })));
    card.appendChild(formRow('API Key', el('input', {
        class: 'form-input', type: 'password', value: s.cache_server_api_key || '',
        onChange: (e) => save('cache_server_api_key', e.target.value),
    })));
    card.appendChild(formRow('Automatch Min %', el('input', {
        class: 'form-input', type: 'number', value: s.automatch_min_pct || '85',
        style: { width: '80px' },
        onChange: (e) => save('automatch_min_pct', e.target.value),
    })));
    card.appendChild(formRow('Manual Min %', el('input', {
        class: 'form-input', type: 'number', value: s.manual_min_pct || '50',
        style: { width: '80px' },
        onChange: (e) => save('manual_min_pct', e.target.value),
    })));
    card.appendChild(formRow('Manual Max Results', el('input', {
        class: 'form-input', type: 'number', value: s.manual_max_results || '5',
        style: { width: '80px' },
        onChange: (e) => save('manual_max_results', e.target.value),
    })));
    body.appendChild(card);
}

function renderSecurity(body, s) {
    const card = el('div', { class: 'card' });
    card.appendChild(el('h3', { style: { marginBottom: '16px' } }, 'Security'));
    card.appendChild(toggle('HTTPS', s.https_enabled === 'true', (v) => save('https_enabled', String(v))));
    card.appendChild(toggle('Fast User Switching', s.fast_user_switch === 'true', (v) => save('fast_user_switch', String(v))));
    card.appendChild(formRow('Min PIN Length', el('input', {
        class: 'form-input', type: 'number', value: s.min_pin_length || '4',
        style: { width: '80px' },
        onChange: (e) => save('min_pin_length', e.target.value),
    })));
    card.appendChild(formRow('Min Password Length', el('input', {
        class: 'form-input', type: 'number', value: s.password_min_length || '8',
        style: { width: '80px' },
        onChange: (e) => save('password_min_length', e.target.value),
    })));
    card.appendChild(toggle('Password Complexity', s.password_complexity === 'true', (v) => save('password_complexity', String(v))));
    body.appendChild(card);
}

function formRow(label, input) {
    return el('div', { class: 'form-group', style: { display: 'flex', alignItems: 'center', justifyContent: 'space-between' } },
        el('label', { class: 'form-label', style: { marginBottom: '0' } }, label),
        input
    );
}

async function save(key, value) {
    try {
        await api.settings.update({ [key]: value });
    } catch (e) {
        toast.error(`Failed to save: ${e.message}`);
    }
}
