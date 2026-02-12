// ──── Chromecast (P14-02) — Bidirectional Control ────
let castSession = null;
let castMediaId = null;
let castProgressInterval = null;

function initCast() {
    if (!window.chrome || !window.chrome.cast) return;
    const sessionRequest = new chrome.cast.SessionRequest(chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID);
    const apiConfig = new chrome.cast.ApiConfig(sessionRequest,
        (session) => { castSession = session; toast('Connected to ' + session.receiver.friendlyName); },
        (availability) => {
            const btn = document.getElementById('castBtn');
            if (btn) btn.style.display = availability === chrome.cast.ReceiverAvailability.AVAILABLE ? 'inline-block' : 'none';
        }
    );
    chrome.cast.initialize(apiConfig, () => {}, () => {});
}

async function startCast(mediaId, title) {
    if (!window.chrome || !window.chrome.cast) { toast('Chromecast not available', 'error'); return; }
    chrome.cast.requestSession((session) => {
        castSession = session;
        castMediaId = mediaId;
        const token = localStorage.getItem('token');
        const url = location.origin + '/api/v1/stream/' + mediaId + '/direct?token=' + encodeURIComponent(token);
        const mediaInfo = new chrome.cast.media.MediaInfo(url, 'video/mp4');
        mediaInfo.metadata = new chrome.cast.media.GenericMediaMetadata();
        mediaInfo.metadata.title = title;

        // Forward subtitle tracks if available
        if (currentStreamInfo && currentStreamInfo.subtitles && currentStreamInfo.subtitles.length > 0) {
            const tracks = currentStreamInfo.subtitles.map((sub, i) => {
                const track = new chrome.cast.media.Track(i + 1, chrome.cast.media.TrackType.TEXT);
                track.trackContentId = location.origin + '/api/v1/stream/' + mediaId + '/subtitles/' + sub.id + '?token=' + encodeURIComponent(token);
                track.trackContentType = 'text/vtt';
                track.subtype = chrome.cast.media.TextTrackType.SUBTITLES;
                track.name = sub.title || sub.language || 'Subtitle ' + (i + 1);
                track.language = sub.language || 'und';
                return track;
            });
            mediaInfo.tracks = tracks;
        }

        const loadReq = new chrome.cast.media.LoadRequest(mediaInfo);
        session.loadMedia(loadReq,
            (m) => {
                toast('Casting: ' + title);
                api('POST', '/cast/session', { media_item_id: mediaId, device_name: session.receiver.friendlyName, device_type: 'chromecast', state: 'playing', current_time: 0, duration: 0 });
                // Start progress polling
                startCastProgressPolling(mediaId);
            },
            (err) => toast('Cast error', 'error')
        );
    }, () => toast('Cast cancelled'));
}

function startCastProgressPolling(mediaId) {
    if (castProgressInterval) clearInterval(castProgressInterval);
    castProgressInterval = setInterval(() => {
        if (!castSession || !castSession.media || castSession.media.length === 0) {
            clearInterval(castProgressInterval);
            return;
        }
        const media = castSession.media[0];
        const currentTime = media.getEstimatedTime ? media.getEstimatedTime() : (media.currentTime || 0);
        const duration = media.media ? (media.media.duration || 0) : 0;
        // Report progress
        api('POST', '/watch/' + mediaId + '/progress', {
            progress_seconds: Math.floor(currentTime),
            duration_seconds: Math.floor(duration)
        });
    }, 5000);
}

function castCommand(command, value) {
    if (!castSession || !castSession.media || castSession.media.length === 0) return;
    const media = castSession.media[0];
    switch (command) {
        case 'play': media.play(new chrome.cast.media.PlayRequest()); break;
        case 'pause': media.pause(new chrome.cast.media.PauseRequest()); break;
        case 'seek':
            const seekReq = new chrome.cast.media.SeekRequest();
            seekReq.currentTime = value;
            media.seek(seekReq);
            break;
        case 'volume':
            const vol = new chrome.cast.Volume(value, false);
            castSession.setReceiverVolumeLevel(value);
            break;
        case 'stop':
            media.stop(new chrome.cast.media.StopRequest());
            break;
    }
}

function stopCast() {
    if (castProgressInterval) clearInterval(castProgressInterval);
    if (castSession) { castSession.stop(() => {}, () => {}); castSession = null; castMediaId = null; toast('Cast stopped'); }
}

// ──── Scene Markers (P15-02) ────
async function loadMarkers(mediaId) {
    const res = await api('GET', '/media/' + mediaId + '/markers');
    if (!res.success || !res.data || res.data.length === 0) return;
    const chaptersEl = document.getElementById('playerChapters');
    if (!chaptersEl) return;
    chaptersEl.innerHTML = '';
    const video = document.getElementById('videoPlayer');
    const duration = video.duration || knownDuration || 1;
    res.data.forEach(m => {
        const marker = document.createElement('div');
        marker.className = 'scene-marker-pip';
        marker.style.left = (m.start_seconds / duration * 100) + '%';
        if (m.end_seconds) marker.style.width = ((m.end_seconds - m.start_seconds) / duration * 100) + '%';
        else marker.style.width = '2px';
        marker.title = m.title + (m.tag_name ? ' (' + m.tag_name + ')' : '');
        marker.onclick = () => { if (video) video.currentTime = m.start_seconds; };
        chaptersEl.appendChild(marker);
    });
}

async function addMarker(mediaId) {
    const title = prompt('Marker title:');
    if (!title) return;
    const video = document.getElementById('videoPlayer');
    const start = video ? video.currentTime : 0;
    const end = prompt('End seconds (optional):', '');
    await api('POST', '/media/' + mediaId + '/markers', {
        title, start_seconds: start, end_seconds: end ? parseFloat(end) : null
    });
    toast('Marker added');
    loadMarkers(mediaId);
}

// Wrapper for the player's Add Marker button
function addMarkerFromPlayer() {
    if (typeof currentMediaId !== 'undefined' && currentMediaId) {
        addMarker(currentMediaId);
    } else {
        toast('No media playing', 'error');
    }
}

// Jump to chapter from chapter selector dropdown
function jumpToChapter(seconds) {
    if (!seconds) return;
    const video = document.getElementById('videoPlayer');
    if (video) video.currentTime = parseFloat(seconds);
}

// ──── Live TV (P15-05) ────
async function loadLiveTVView() {
    const mc = document.getElementById('mainContent');
    mc.innerHTML = '<div class="section-header"><h2 class="section-title">Live TV & DVR</h2></div><div id="liveTVContainer"><div class="spinner"></div></div>';
    const epg = await api('GET', '/livetv/epg');
    const rec = await api('GET', '/livetv/recordings');
    const container = document.getElementById('liveTVContainer');
    if (!container) return;

    let html = '<div class="epg-grid">';
    if (epg.success && epg.data && epg.data.length > 0) {
        epg.data.forEach(ch => {
            html += `<div class="epg-channel"><div class="epg-channel-name">${ch.icon_url ? '<img src="'+ch.icon_url+'" class="epg-icon">' : ''}${ch.channel_number} ${ch.name}${ch.is_favorite ? ' ★' : ''}</div><div class="epg-programs">`;
            if (ch.programs) {
                ch.programs.forEach(p => {
                    const start = new Date(p.start_time).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
                    const end = new Date(p.end_time).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
                    html += `<div class="epg-program" title="${p.description||''}">${start}-${end} <strong>${p.title}</strong>${p.category ? ' <span class="tag tag-blue">'+p.category+'</span>' : ''}</div>`;
                });
            } else {
                html += '<div class="epg-program">No program data</div>';
            }
            html += '</div></div>';
        });
    } else {
        html += '<div class="empty-state"><div class="empty-state-title">No channels configured</div><div class="empty-state-subtitle">Add a tuner device in Settings to get started</div></div>';
    }
    html += '</div>';

    // DVR recordings
    if (rec.success && rec.data && rec.data.length > 0) {
        html += '<h3 style="margin-top:24px;">DVR Recordings</h3><div class="recordings-list">';
        rec.data.forEach(r => {
            const stateClass = r.state === 'recording' ? 'tag-red' : r.state === 'completed' ? 'tag-green' : 'tag-yellow';
            html += `<div class="recording-item"><strong>${r.title}</strong><span class="tag ${stateClass}">${r.state}</span><span class="text-muted">${new Date(r.start_time).toLocaleDateString()}</span></div>`;
        });
        html += '</div>';
    }

    container.innerHTML = html;
}

// ──── Comic / eBook Reader (P15-06) ────
async function openReader(mediaId, title) {
    const mc = document.getElementById('mainContent');
    const prog = await api('GET', '/media/' + mediaId + '/reading-progress');
    const p = (prog.success && prog.data) ? prog.data : { current_page: 0, total_pages: 0, font_size: 16 };

    mc.innerHTML = `<div class="reader-container">
        <div class="reader-header">
            <button class="btn-secondary" onclick="navigate('home')">&#8592; Back</button>
            <span class="reader-title">${title}</span>
            <span class="reader-page" id="readerPageNum">Page ${p.current_page + 1} / ${p.total_pages || '?'}</span>
            <div class="reader-controls">
                <button onclick="readerFontSize(-2)">A-</button>
                <button onclick="readerFontSize(2)">A+</button>
            </div>
        </div>
        <div class="reader-content" id="readerContent" style="font-size:${p.font_size}px;">
            <div class="spinner"></div>
        </div>
        <div class="reader-nav">
            <button class="btn-secondary" onclick="readerPage(-1, '${mediaId}')">&#8592; Prev</button>
            <button class="btn-primary" onclick="readerPage(1, '${mediaId}')">Next &#8594;</button>
        </div>
    </div>`;

    window._readerState = { mediaId, page: p.current_page, total: p.total_pages, fontSize: p.font_size };
}

function readerPage(delta, mediaId) {
    if (!window._readerState) return;
    window._readerState.page = Math.max(0, window._readerState.page + delta);
    const el = document.getElementById('readerPageNum');
    if (el) el.textContent = 'Page ' + (window._readerState.page + 1) + ' / ' + (window._readerState.total || '?');
    api('PUT', '/media/' + mediaId + '/reading-progress', {
        current_page: window._readerState.page, total_pages: window._readerState.total, font_size: window._readerState.fontSize
    });
}

function readerFontSize(delta) {
    if (!window._readerState) return;
    window._readerState.fontSize = Math.max(10, Math.min(32, window._readerState.fontSize + delta));
    const content = document.getElementById('readerContent');
    if (content) content.style.fontSize = window._readerState.fontSize + 'px';
}

// ──── Settings (moved to settings.html) ────

// Analytics + Admin moved to settings.html

// NOTE: Video player code lives in player.js — do not duplicate here
